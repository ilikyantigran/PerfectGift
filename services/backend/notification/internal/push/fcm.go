package push

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/notify"
)

// TokenSource yields a bearer token for the FCM v1 API. It is an interface so a
// test or an alternative auth mechanism can be substituted.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// FCMConfig configures the Android push client (FCM HTTP v1).
type FCMConfig struct {
	CredentialsPath string // service-account JSON key
	ProjectID       string
}

// FCM is a notify.Pusher backed by the FCM HTTP v1 API.
type FCM struct {
	projectID string
	tokens    TokenSource
	client    *http.Client
}

var _ notify.Pusher = (*FCM)(nil)

// NewFCM builds an FCM client from a service-account credentials file.
func NewFCM(cfg FCMConfig) (*FCM, error) {
	ts, projectID, err := newServiceAccountTokenSource(cfg.CredentialsPath)
	if err != nil {
		return nil, err
	}
	if cfg.ProjectID != "" {
		projectID = cfg.ProjectID
	}
	if projectID == "" {
		return nil, fmt.Errorf("fcm: project id is required")
	}
	return &FCM{
		projectID: projectID,
		tokens:    ts,
		client:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Push sends one notification to a device via FCM.
func (f *FCM) Push(ctx context.Context, device notify.Device, msg notify.PushMessage) error {
	tok, err := f.tokens.Token(ctx)
	if err != nil {
		return err
	}
	title, body := titleBody(msg.Payload)
	reqBody, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"token":        device.PushToken,
			"notification": map[string]string{"title": title, "body": body},
			"data":         map[string]string{"type": string(msg.Type)},
		},
	})
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", f.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("authorization", "Bearer "+tok)
	req.Header.Set("content-type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return err // transient
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusNotFound || fcmUnregistered(resp):
		return notify.ErrInvalidToken
	default:
		return fmt.Errorf("fcm status %d", resp.StatusCode) // transient
	}
}

func fcmUnregistered(resp *http.Response) bool {
	var b struct {
		Error struct {
			Status  string `json:"status"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&b)
	if b.Error.Status == "NOT_FOUND" {
		return true
	}
	for _, d := range b.Error.Details {
		if d.ErrorCode == "UNREGISTERED" {
			return true
		}
	}
	return false
}

// --- service-account OAuth token source (stdlib only) ----------------------

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
	ProjectID   string `json:"project_id"`
}

type saTokenSource struct {
	sa     serviceAccount
	key    *rsa.PrivateKey
	client *http.Client

	mu    sync.Mutex
	token string
	exp   time.Time
}

func newServiceAccountTokenSource(path string) (TokenSource, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read fcm credentials: %w", err)
	}
	var sa serviceAccount
	if err := json.Unmarshal(raw, &sa); err != nil {
		return nil, "", fmt.Errorf("parse fcm credentials: %w", err)
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, "", fmt.Errorf("fcm credentials: private_key not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("fcm private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, "", fmt.Errorf("fcm private key: not RSA")
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return &saTokenSource{sa: sa, key: key, client: &http.Client{Timeout: 10 * time.Second}}, sa.ProjectID, nil
}

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// Token returns a cached OAuth2 access token, minting a new one (via a signed
// JWT assertion exchanged at Google's token endpoint) when the cache is stale.
func (s *saTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Until(s.exp) > time.Minute {
		return s.token, nil
	}

	now := time.Now()
	header := b64json(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims := b64json(map[string]any{
		"iss":   s.sa.ClientEmail,
		"scope": fcmScope,
		"aud":   s.sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	signingInput := header + "." + claims
	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	assertion := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.sa.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint status %d", resp.StatusCode)
	}

	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	s.token = out.AccessToken
	s.exp = now.Add(time.Duration(out.ExpiresIn) * time.Second)
	return s.token, nil
}
