package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/catalog/v1"
	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/identity/v1"
	notificationv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/notification/v1"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/poll/v1"
	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/surprise/v1"
)

// testServer builds a Server with the given fakes and an always-valid verifier for
// subject "user-123" unless overridden.
type fakes struct {
	identity     *fakeIdentity
	poll         *fakePoll
	surprise     *fakeSurprise
	catalog      *fakeCatalog
	notification *fakeNotification
	verifier     TokenVerifier
}

func newFakes() *fakes {
	return &fakes{
		identity:     &fakeIdentity{},
		poll:         &fakePoll{},
		surprise:     &fakeSurprise{},
		catalog:      &fakeCatalog{},
		notification: &fakeNotification{},
		verifier:     fakeVerifier{subject: "user-123"},
	}
}

func (f *fakes) handler(t *testing.T) http.Handler {
	t.Helper()
	return New(Options{
		Identity:     f.identity,
		Poll:         f.poll,
		Surprise:     f.surprise,
		Catalog:      f.catalog,
		Notification: f.notification,
		Verifier:     f.verifier,
		CORSOrigins:  []string{"https://poll.perfectgift.app"},
	}).Handler()
}

func do(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func authHdr() map[string]string { return map[string]string{"Authorization": "Bearer good"} }

// ---------------------------------------------------------------------------
// Auth gating
// ---------------------------------------------------------------------------

func TestJWTRoute_RejectsMissingToken(t *testing.T) {
	f := newFakes()
	rec := do(t, f.handler(t), http.MethodGet, "/v1/me", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	decodeEnvelope(t, rec.Body.Bytes())
}

func TestJWTRoute_RejectsInvalidToken(t *testing.T) {
	f := newFakes()
	f.verifier = fakeVerifier{err: errors.New("bad")}
	rec := do(t, f.handler(t), http.MethodGet, "/v1/me", "", authHdr())
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (fail closed)", rec.Code)
	}
}

func TestGetMe_UsesSubjectFromJWT_NotBody(t *testing.T) {
	f := newFakes()
	var gotUserID string
	f.identity.getMe = func(in *identityv1.GetMeRequest) (*identityv1.GetMeResponse, error) {
		gotUserID = in.GetUserId()
		return &identityv1.GetMeResponse{User: &identityv1.User{Id: in.GetUserId(), Email: "a@b.c"}}, nil
	}
	rec := do(t, f.handler(t), http.MethodGet, "/v1/me", "", authHdr())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotUserID != "user-123" {
		t.Fatalf("downstream user_id = %q, want the JWT subject user-123", gotUserID)
	}
}

// ---------------------------------------------------------------------------
// Public routes
// ---------------------------------------------------------------------------

func TestSignIn_Public_RoutesToIdentity(t *testing.T) {
	f := newFakes()
	f.identity.signIn = func(in *identityv1.SignInRequest) (*identityv1.SignInResponse, error) {
		if in.GetProvider() != "google" {
			t.Errorf("provider = %q", in.GetProvider())
		}
		return &identityv1.SignInResponse{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 900}, nil
	}
	rec := do(t, f.handler(t), http.MethodPost, "/v1/auth/signin", `{"provider":"google","id_token":"x"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["access_token"] != "at" {
		t.Fatalf("access_token missing: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Anonymous poll (token) routes — no JWT
// ---------------------------------------------------------------------------

func TestGetPollByToken_Anonymous_NoJWT(t *testing.T) {
	f := newFakes()
	var gotToken string
	f.poll.getPollByToken = func(in *pollv1.GetPollByTokenRequest) (*pollv1.GetPollByTokenResponse, error) {
		gotToken = in.GetToken()
		return &pollv1.GetPollByTokenResponse{PollId: "p1", Title: "T"}, nil
	}
	// No Authorization header at all — must still succeed.
	rec := do(t, f.handler(t), http.MethodGet, "/v1/polls/token/abc123", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (anonymous)", rec.Code)
	}
	if gotToken != "abc123" {
		t.Fatalf("token = %q, want abc123", gotToken)
	}
}

func TestSubmitResponse_Anonymous_NoJWT(t *testing.T) {
	f := newFakes()
	f.poll.submitResponse = func(in *pollv1.SubmitResponseRequest) (*pollv1.SubmitResponseResponse, error) {
		if in.GetToken() != "tok" || len(in.GetAnswers()) != 1 {
			t.Errorf("unexpected submit: %+v", in)
		}
		return &pollv1.SubmitResponseResponse{Ok: true}, nil
	}
	body := `{"answers":[{"question_id":"q1","value":"blue"}]}`
	rec := do(t, f.handler(t), http.MethodPost, "/v1/polls/token/tok/responses", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// CORS — only on the two anonymous poll routes
// ---------------------------------------------------------------------------

func TestCORS_AllowedOnPollTokenRoute(t *testing.T) {
	f := newFakes()
	f.poll.getPollByToken = func(*pollv1.GetPollByTokenRequest) (*pollv1.GetPollByTokenResponse, error) {
		return &pollv1.GetPollByTokenResponse{PollId: "p1"}, nil
	}
	rec := do(t, f.handler(t), http.MethodGet, "/v1/polls/token/abc", "", map[string]string{
		"Origin": "https://poll.perfectgift.app",
	})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://poll.perfectgift.app" {
		t.Fatalf("CORS origin header = %q, want the poll web origin", got)
	}
}

func TestCORS_PreflightOnSubmitRoute(t *testing.T) {
	f := newFakes()
	rec := do(t, f.handler(t), http.MethodOptions, "/v1/polls/token/tok/responses", "", map[string]string{
		"Origin": "https://poll.perfectgift.app",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("preflight missing Access-Control-Allow-Methods")
	}
}

func TestCORS_NotSetOnJWTRoute(t *testing.T) {
	f := newFakes()
	f.catalog.getCategories = func(*catalogv1.GetCategoriesRequest) (*catalogv1.GetCategoriesResponse, error) {
		return &catalogv1.GetCategoriesResponse{}, nil
	}
	rec := do(t, f.handler(t), http.MethodGet, "/v1/categories", "", map[string]string{
		"Authorization": "Bearer good",
		"Origin":        "https://poll.perfectgift.app",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("CORS must NOT be set on JWT routes, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Generation — 202 async + Idempotency-Key passthrough
// ---------------------------------------------------------------------------

func TestRequestGeneration_Returns202AndForwardsIdempotencyKey(t *testing.T) {
	f := newFakes()
	var gotKey, gotUser string
	f.surprise.requestGeneration = func(in *surprisev1.RequestGenerationRequest) (*surprisev1.RequestGenerationResponse, error) {
		gotKey = in.GetIdempotencyKey()
		gotUser = in.GetUserId()
		return &surprisev1.RequestGenerationResponse{RequestId: "req-1", Status: "queued"}, nil
	}
	rec := do(t, f.handler(t), http.MethodPost, "/v1/generations",
		`{"holiday_id":"h1","budget_band":"mid","preferences_text":"cozy"}`,
		map[string]string{"Authorization": "Bearer good", "Idempotency-Key": "idem-42"})

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	var out map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["request_id"] != "req-1" {
		t.Fatalf("request_id = %q, want req-1", out["request_id"])
	}
	if gotKey != "idem-42" {
		t.Fatalf("Idempotency-Key forwarded = %q, want idem-42", gotKey)
	}
	if gotUser != "user-123" {
		t.Fatalf("user_id = %q, want JWT subject", gotUser)
	}
}

// ---------------------------------------------------------------------------
// Generation status aggregation (BFF)
// ---------------------------------------------------------------------------

func TestGetGeneration_AggregatesIdeasWhenReady(t *testing.T) {
	f := newFakes()
	f.surprise.getGenerationStatus = func(*surprisev1.GetGenerationStatusRequest) (*surprisev1.GetGenerationStatusResponse, error) {
		return &surprisev1.GetGenerationStatusResponse{Status: "ready"}, nil
	}
	ideasCalled := false
	f.surprise.getIdeas = func(*surprisev1.GetIdeasRequest) (*surprisev1.GetIdeasResponse, error) {
		ideasCalled = true
		return &surprisev1.GetIdeasResponse{Ideas: []*surprisev1.Idea{{Id: "i1", Title: "Picnic", Rank: 1}}}, nil
	}
	rec := do(t, f.handler(t), http.MethodGet, "/v1/generations/req-1", "", authHdr())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !ideasCalled {
		t.Fatal("expected GetIdeas to be called when status is ready")
	}
	var out generationStatusResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Ideas) != 1 || out.Ideas[0].Title != "Picnic" {
		t.Fatalf("ideas not aggregated: %s", rec.Body.String())
	}
}

func TestGetGeneration_NoIdeasWhenNotReady(t *testing.T) {
	f := newFakes()
	f.surprise.getGenerationStatus = func(*surprisev1.GetGenerationStatusRequest) (*surprisev1.GetGenerationStatusResponse, error) {
		return &surprisev1.GetGenerationStatusResponse{Status: "running", Progress: 42}, nil
	}
	f.surprise.getIdeas = func(*surprisev1.GetIdeasRequest) (*surprisev1.GetIdeasResponse, error) {
		t.Fatal("GetIdeas must not be called while running")
		return nil, nil
	}
	rec := do(t, f.handler(t), http.MethodGet, "/v1/generations/req-1", "", authHdr())
	var out generationStatusResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Status != "running" || out.Progress != 42 || len(out.Ideas) != 0 {
		t.Fatalf("unexpected: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Error mapping from downstream gRPC
// ---------------------------------------------------------------------------

func TestDownstreamError_MapsToHTTPStatus(t *testing.T) {
	f := newFakes()
	f.poll.getResponses = func(*pollv1.GetResponsesRequest) (*pollv1.GetResponsesResponse, error) {
		return nil, status.Error(codes.PermissionDenied, "not your poll")
	}
	rec := do(t, f.handler(t), http.MethodGet, "/v1/polls/p1/responses", "", authHdr())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	env := decodeEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != "permission_denied" {
		t.Fatalf("envelope code = %q", env.Error.Code)
	}
}

// ---------------------------------------------------------------------------
// A couple more routes to prove full wiring
// ---------------------------------------------------------------------------

func TestCreatePoll_InjectsOwnerFromJWT_Returns201(t *testing.T) {
	f := newFakes()
	var owner string
	f.poll.createPoll = func(in *pollv1.CreatePollRequest) (*pollv1.CreatePollResponse, error) {
		owner = in.GetOwnerUserId()
		return &pollv1.CreatePollResponse{PollId: "p1", LinkToken: "tok", LinkUrl: "https://x/tok"}, nil
	}
	rec := do(t, f.handler(t), http.MethodPost, "/v1/polls",
		`{"title":"Q","questions":[{"id":"q1","text":"fav color"}]}`, authHdr())
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if owner != "user-123" {
		t.Fatalf("owner = %q, want JWT subject", owner)
	}
}

func TestRegisterDevice_Returns201(t *testing.T) {
	f := newFakes()
	var gotUser, gotToken string
	f.notification.registerDevice = func(in *notificationv1.RegisterDeviceRequest) (*notificationv1.RegisterDeviceResponse, error) {
		gotUser = in.GetUserId()
		gotToken = in.GetPushToken()
		return &notificationv1.RegisterDeviceResponse{DeviceId: "dev-1"}, nil
	}
	rec := do(t, f.handler(t), http.MethodPost, "/v1/devices",
		`{"platform":"ios","push_token":"tok","app_version":"1.0.0"}`, authHdr())
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if gotUser != "user-123" || gotToken != "tok" {
		t.Fatalf("register mapped wrong: user=%q token=%q", gotUser, gotToken)
	}
	var out map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["device_id"] != "dev-1" {
		t.Fatalf("device_id = %q", out["device_id"])
	}
}
