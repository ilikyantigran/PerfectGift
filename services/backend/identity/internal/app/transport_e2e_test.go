package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/identity/pkg/api/identity/v1"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/oauth"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/token"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestTransportEndToEnd boots the real gRPC server and the grpc-gateway REST
// edge (backed by in-memory fakes) and drives real HTTP requests through them —
// proving the wiring the App assembles, without needing Postgres/Valkey.
func TestTransportEndToEnd(t *testing.T) {
	tm, _ := token.NewManager("iss", "aud", 15*time.Minute)
	verifier := oauth.NewFakeVerifier()
	verifier.Register("g-token", oauth.Identity{Provider: "google", Subject: "sub-1", Email: "e2e@example.com"})
	srv := NewServer(Deps{
		Users:      newFakeUsers(),
		Sessions:   newFakeSessions(),
		Limiter:    newFakeLimiter(100),
		Verifier:   verifier,
		Tokens:     tm,
		RefreshTTL: time.Hour,
	})

	// Real gRPC server on a random local port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	defer gs.Stop()

	// grpc-gateway REST mux dialing the gRPC server, wrapped in an httptest server.
	ctx := context.Background()
	mux := runtime.NewServeMux()
	if err := identityv1.RegisterIdentityServiceHandlerFromEndpoint(
		ctx, mux, lis.Addr().String(),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	); err != nil {
		t.Fatalf("register gateway: %v", err)
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. POST /v1/auth/signin
	signInBody := `{"provider":"PROVIDER_GOOGLE","id_token":"g-token"}`
	resp, err := http.Post(ts.URL+"/v1/auth/signin", "application/json", strings.NewReader(signInBody))
	if err != nil {
		t.Fatalf("signin POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("signin status = %d", resp.StatusCode)
	}
	var signIn struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		User         struct {
			Id    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&signIn); err != nil {
		t.Fatalf("decode signin: %v", err)
	}
	if signIn.AccessToken == "" || signIn.User.Email != "e2e@example.com" {
		t.Fatalf("unexpected signin response: %+v", signIn)
	}

	// 2. GET /.well-known/jwks.json
	jresp, err := http.Get(ts.URL + "/.well-known/jwks.json")
	if err != nil {
		t.Fatalf("jwks GET: %v", err)
	}
	defer jresp.Body.Close()
	if jresp.StatusCode != http.StatusOK {
		t.Fatalf("jwks status = %d", jresp.StatusCode)
	}
	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(jresp.Body).Decode(&jwks); err != nil {
		t.Fatalf("decode jwks: %v", err)
	}
	if len(jwks.Keys) == 0 || jwks.Keys[0].Kty != "OKP" {
		t.Fatalf("unexpected jwks: %+v", jwks)
	}

	// 3. GET /v1/auth/me with the Bearer token from sign-in.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+signIn.AccessToken)
	meResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("me GET: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("me status = %d", meResp.StatusCode)
	}
	var me struct {
		User struct {
			Id string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(meResp.Body).Decode(&me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.User.Id != signIn.User.Id {
		t.Errorf("GetMe returned %q, want %q", me.User.Id, signIn.User.Id)
	}
}
