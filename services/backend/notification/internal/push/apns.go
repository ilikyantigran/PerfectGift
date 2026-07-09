// Package push holds the production Pusher implementations (APNs for iOS, FCM
// for Android). The dispatcher depends on the notify.Pusher interface and the
// unit tests use a fake, so this package is compiled but not exercised by
// `go test ./...`. Both clients map a provider's "dead token" response to
// notify.ErrInvalidToken so the dispatcher prunes the device instead of
// retrying it.
package push

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/notify"
)

// APNsConfig configures the iOS push client (token-based auth with a .p8 key).
type APNsConfig struct {
	KeyPath string // path to the AuthKey_XXXX.p8 file
	KeyID   string // the key id (kid)
	TeamID  string // Apple developer team id (iss)
	Topic   string // app bundle id (apns-topic)
	Sandbox bool   // use the sandbox host
}

// APNs is a notify.Pusher backed by the APNs HTTP/2 provider API.
type APNs struct {
	cfg    APNsConfig
	key    *ecdsa.PrivateKey
	client *http.Client
	host   string

	mu       sync.Mutex
	jwt      string
	jwtIssAt time.Time
}

var _ notify.Pusher = (*APNs)(nil)

// NewAPNs loads the signing key and prepares the client.
func NewAPNs(cfg APNsConfig) (*APNs, error) {
	raw, err := os.ReadFile(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("read apns key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("apns key: not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("apns key: not an ECDSA key")
	}
	host := "https://api.push.apple.com"
	if cfg.Sandbox {
		host = "https://api.sandbox.push.apple.com"
	}
	return &APNs{
		cfg:    cfg,
		key:    key,
		client: &http.Client{Timeout: 10 * time.Second},
		host:   host,
	}, nil
}

// Push sends one notification to a device via APNs.
func (a *APNs) Push(ctx context.Context, device notify.Device, msg notify.PushMessage) error {
	jwt, err := a.token()
	if err != nil {
		return err
	}
	body, err := buildAPNsPayload(msg)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/3/device/%s", a.host, device.PushToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("authorization", "bearer "+jwt)
	req.Header.Set("apns-topic", a.cfg.Topic)
	req.Header.Set("apns-push-type", "alert")

	resp, err := a.client.Do(req)
	if err != nil {
		return err // transient (network) → dispatcher retries
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusGone: // 410: token no longer valid
		return notify.ErrInvalidToken
	case resp.StatusCode == http.StatusBadRequest && apnsBadToken(resp):
		return notify.ErrInvalidToken
	default:
		return fmt.Errorf("apns status %d", resp.StatusCode) // transient
	}
}

func apnsBadToken(resp *http.Response) bool {
	var b struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&b)
	return b.Reason == "BadDeviceToken" || b.Reason == "Unregistered"
}

// token returns a cached provider JWT, refreshing it if older than ~50 minutes
// (APNs rejects tokens older than 60 minutes).
func (a *APNs) token() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.jwt != "" && time.Since(a.jwtIssAt) < 50*time.Minute {
		return a.jwt, nil
	}
	now := time.Now()
	header := b64json(map[string]string{"alg": "ES256", "kid": a.cfg.KeyID})
	claims := b64json(map[string]any{"iss": a.cfg.TeamID, "iat": now.Unix()})
	signingInput := header + "." + claims

	h := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, a.key, h[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	jwt := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	a.jwt = jwt
	a.jwtIssAt = now
	return jwt, nil
}

func buildAPNsPayload(msg notify.PushMessage) ([]byte, error) {
	title, body := titleBody(msg.Payload)
	return json.Marshal(map[string]any{
		"aps": map[string]any{
			"alert": map[string]string{"title": title, "body": body},
			"sound": "default",
		},
		"type": string(msg.Type),
	})
}

func b64json(v any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}

// titleBody extracts the display title/body from an outbox payload, tolerating
// a payload that lacks them.
func titleBody(payload []byte) (string, string) {
	var p struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	_ = json.Unmarshal(payload, &p)
	if p.Title == "" {
		p.Title = "PerfectGift"
	}
	return p.Title, p.Body
}
