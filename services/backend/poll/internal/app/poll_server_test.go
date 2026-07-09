package app

import (
	"context"
	"testing"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/token"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/infra/auth"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/poll/pkg/api/poll/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fixture struct {
	srv  *Server
	repo *fakeRepo
	rl   *fakeLimiter
	pub  *fakePublisher
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo := newFakeRepo()
	rl := newFakeLimiter()
	pub := newFakePublisher()
	srv := NewServer(repo, rl, pub, Tuning{
		DefaultTTL:     time.Hour,
		PerTokenBudget: 3,
		PerTokenWindow: time.Hour,
		PerIPBudget:    100,
		PerIPWindow:    time.Hour,
		AllowedOrigin:  "https://poll.example.com",
		LinkPath:       "/p/{token}",
	})
	return &fixture{srv: srv, repo: repo, rl: rl, pub: pub}
}

func ownerCtx(sub string) context.Context {
	return auth.WithSubject(context.Background(), sub)
}

func sampleQuestions() []*pollv1.Question {
	return []*pollv1.Question{
		{Id: "q1", Prompt: "Favourite memory?", Type: pollv1.QuestionType_QUESTION_TYPE_TEXT, Required: true},
		{Id: "q2", Prompt: "Pick", Type: pollv1.QuestionType_QUESTION_TYPE_SINGLE_CHOICE, Required: true,
			Options: []*pollv1.Option{{Id: "a", Label: "A"}, {Id: "b", Label: "B"}}},
	}
}

func code(err error) codes.Code { return status.Code(err) }

// createActivePoll drives CreatePoll and returns (pollID, rawToken).
func createActivePoll(t *testing.T, f *fixture, owner string) (string, string) {
	t.Helper()
	resp, err := f.srv.CreatePoll(ownerCtx(owner), &pollv1.CreatePollRequest{
		Title:             "For my partner",
		Questions:         sampleQuestions(),
		SurpriseRequestId: "sr-1",
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}
	return resp.GetPollId(), resp.GetLinkToken()
}

// ---- CreatePoll ----

func TestCreatePoll_RequiresAuth(t *testing.T) {
	f := newFixture(t)
	_, err := f.srv.CreatePoll(context.Background(), &pollv1.CreatePollRequest{
		Title: "x", Questions: sampleQuestions(),
	})
	if code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestCreatePoll_StoresHashNotRawToken(t *testing.T) {
	f := newFixture(t)
	_, raw := createActivePoll(t, f, "owner-1")

	f.repo.mu.Lock()
	defer f.repo.mu.Unlock()
	if _, ok := f.repo.links[raw]; ok {
		t.Fatal("raw token must not be a key in the link store")
	}
	if _, ok := f.repo.links[token.Hash(raw)]; !ok {
		t.Fatal("link must be stored under the token hash")
	}
}

func TestCreatePoll_ReturnsLinkURLAndExpiry(t *testing.T) {
	f := newFixture(t)
	resp, err := f.srv.CreatePoll(ownerCtx("owner-1"), &pollv1.CreatePollRequest{
		Title: "t", Questions: sampleQuestions(), TtlSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}
	want := "https://poll.example.com/p/" + resp.GetLinkToken()
	if resp.GetLinkUrl() != want {
		t.Fatalf("link_url=%q want %q", resp.GetLinkUrl(), want)
	}
	if _, err := time.Parse(time.RFC3339, resp.GetExpiresAt()); err != nil {
		t.Fatalf("expires_at not RFC3339: %v", err)
	}
}

func TestCreatePoll_RejectsNoQuestions(t *testing.T) {
	f := newFixture(t)
	_, err := f.srv.CreatePoll(ownerCtx("o"), &pollv1.CreatePollRequest{Title: "t"})
	if code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

// ---- GetPollByToken ----

func TestGetPollByToken_Valid(t *testing.T) {
	f := newFixture(t)
	pollID, raw := createActivePoll(t, f, "owner-1")

	resp, err := f.srv.GetPollByToken(context.Background(), &pollv1.GetPollByTokenRequest{Token: raw})
	if err != nil {
		t.Fatalf("GetPollByToken: %v", err)
	}
	if resp.GetPollId() != pollID {
		t.Fatalf("poll_id=%q want %q", resp.GetPollId(), pollID)
	}
	if len(resp.GetQuestions()) != 2 {
		t.Fatalf("questions=%d want 2", len(resp.GetQuestions()))
	}
}

func TestGetPollByToken_UnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.srv.GetPollByToken(context.Background(), &pollv1.GetPollByTokenRequest{Token: "nope"})
	if code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestGetPollByToken_ExpiredIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, raw := createActivePoll(t, f, "owner-1")
	// advance the clock past expiry
	f.srv.now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	_, err := f.srv.GetPollByToken(context.Background(), &pollv1.GetPollByTokenRequest{Token: raw})
	if code(err) != codes.NotFound {
		t.Fatalf("want NotFound for expired, got %v", err)
	}
}

func TestGetPollByToken_RevokedIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, raw := createActivePoll(t, f, "owner-1")
	f.repo.mu.Lock()
	l := f.repo.links[token.Hash(raw)]
	l.revoked = true
	f.repo.links[token.Hash(raw)] = l
	f.repo.mu.Unlock()

	_, err := f.srv.GetPollByToken(context.Background(), &pollv1.GetPollByTokenRequest{Token: raw})
	if code(err) != codes.NotFound {
		t.Fatalf("want NotFound for revoked, got %v", err)
	}
}

func TestGetPollByToken_NoOwnerDataLeaked(t *testing.T) {
	// The response type simply has no owner/surprise fields; this asserts the
	// contract at compile time + that a fetch never surfaces them.
	f := newFixture(t)
	_, raw := createActivePoll(t, f, "owner-secret")
	resp, err := f.srv.GetPollByToken(context.Background(), &pollv1.GetPollByTokenRequest{Token: raw})
	if err != nil {
		t.Fatalf("GetPollByToken: %v", err)
	}
	// Marshalled response must not contain the owner id.
	if got := resp.String(); contains(got, "owner-secret") || contains(got, "sr-1") {
		t.Fatalf("owner/surprise data leaked in response: %q", got)
	}
}

// ---- SubmitResponse ----

func validAnswers() []*pollv1.Answer {
	return []*pollv1.Answer{
		{QuestionId: "q1", Text: "the beach"},
		{QuestionId: "q2", ChoiceIds: []string{"a"}},
	}
}

func TestSubmitResponse_HappyPathEmitsEvent(t *testing.T) {
	f := newFixture(t)
	pollID, raw := createActivePoll(t, f, "owner-1")

	resp, err := f.srv.SubmitResponse(context.Background(), &pollv1.SubmitResponseRequest{
		Token: raw, Answers: validAnswers(),
	})
	if err != nil {
		t.Fatalf("SubmitResponse: %v", err)
	}
	if !resp.GetOk() {
		t.Fatal("expected ok=true")
	}
	if f.pub.count() != 1 {
		t.Fatalf("expected 1 PollCompleted event, got %d", f.pub.count())
	}
	ev := f.pub.events[0]
	if ev.PollID != pollID || ev.OwnerUserID != "owner-1" || ev.SurpriseRequestID != "sr-1" {
		t.Fatalf("event payload wrong: %+v", ev)
	}
	if ev.CompletedAt.IsZero() {
		t.Fatal("event missing completed_at")
	}
}

func TestSubmitResponse_SecondSubmitIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, raw := createActivePoll(t, f, "owner-1")

	if _, err := f.srv.SubmitResponse(context.Background(), &pollv1.SubmitResponseRequest{
		Token: raw, Answers: validAnswers(),
	}); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	// second submit: poll now completed -> uniform NotFound (consumed)
	_, err := f.srv.SubmitResponse(context.Background(), &pollv1.SubmitResponseRequest{
		Token: raw, Answers: validAnswers(),
	})
	if code(err) != codes.NotFound {
		t.Fatalf("want NotFound on second submit, got %v", err)
	}
	if f.pub.count() != 1 {
		t.Fatalf("second submit must not emit another event; count=%d", f.pub.count())
	}
}

func TestSubmitResponse_InvalidAnswersRejected(t *testing.T) {
	f := newFixture(t)
	_, raw := createActivePoll(t, f, "owner-1")
	// omit required q1
	_, err := f.srv.SubmitResponse(context.Background(), &pollv1.SubmitResponseRequest{
		Token: raw, Answers: []*pollv1.Answer{{QuestionId: "q2", ChoiceIds: []string{"a"}}},
	})
	if code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
	if f.pub.count() != 0 {
		t.Fatal("invalid submit must not emit an event")
	}
}

func TestSubmitResponse_BadTokenIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.srv.SubmitResponse(context.Background(), &pollv1.SubmitResponseRequest{
		Token: "ghost", Answers: validAnswers(),
	})
	if code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestSubmitResponse_RateLimited(t *testing.T) {
	f := newFixture(t)
	_, raw := createActivePoll(t, f, "owner-1")
	// per-token budget is 3; exhaust it with invalid submits that still consume
	// the limiter but keep the poll active.
	for i := 0; i < 3; i++ {
		_, _ = f.srv.SubmitResponse(context.Background(), &pollv1.SubmitResponseRequest{
			Token: raw, Answers: []*pollv1.Answer{{QuestionId: "q1", Text: ""}}, // invalid, keeps poll active
		})
	}
	_, err := f.srv.SubmitResponse(context.Background(), &pollv1.SubmitResponseRequest{
		Token: raw, Answers: validAnswers(),
	})
	if code(err) != codes.ResourceExhausted {
		t.Fatalf("want ResourceExhausted after budget, got %v", err)
	}
}

// ---- GetResponses ----

func TestGetResponses_OwnerSeesResponses(t *testing.T) {
	f := newFixture(t)
	pollID, raw := createActivePoll(t, f, "owner-1")
	if _, err := f.srv.SubmitResponse(context.Background(), &pollv1.SubmitResponseRequest{
		Token: raw, Answers: validAnswers(),
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	resp, err := f.srv.GetResponses(ownerCtx("owner-1"), &pollv1.GetResponsesRequest{PollId: pollID})
	if err != nil {
		t.Fatalf("GetResponses: %v", err)
	}
	if len(resp.GetResponses()) != 1 {
		t.Fatalf("responses=%d want 1", len(resp.GetResponses()))
	}
	if len(resp.GetResponses()[0].GetAnswers()) != 2 {
		t.Fatalf("answers=%d want 2", len(resp.GetResponses()[0].GetAnswers()))
	}
}

func TestGetResponses_RequiresAuth(t *testing.T) {
	f := newFixture(t)
	pollID, _ := createActivePoll(t, f, "owner-1")
	_, err := f.srv.GetResponses(context.Background(), &pollv1.GetResponsesRequest{PollId: pollID})
	if code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestGetResponses_NonOwnerIsNotFound(t *testing.T) {
	f := newFixture(t)
	pollID, _ := createActivePoll(t, f, "owner-1")
	_, err := f.srv.GetResponses(ownerCtx("intruder"), &pollv1.GetResponsesRequest{PollId: pollID})
	if code(err) != codes.NotFound {
		t.Fatalf("want NotFound for non-owner, got %v", err)
	}
}

func TestGetResponses_UnknownPollIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.srv.GetResponses(ownerCtx("owner-1"), &pollv1.GetResponsesRequest{PollId: "missing"})
	if code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
