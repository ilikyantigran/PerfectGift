package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/model"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/token"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/infra/auth"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/ports"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/poll/pkg/api/poll/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Tuning holds the server's runtime knobs, sourced from config.
type Tuning struct {
	DefaultTTL     time.Duration
	PerTokenBudget int
	PerTokenWindow time.Duration
	PerIPBudget    int
	PerIPWindow    time.Duration
	AllowedOrigin  string // link_url base + the only CORS origin for public routes
	LinkPath       string // template with {token}, e.g. "/p/{token}"
}

// Server implements the PollService RPCs. It depends only on the ports (Repo,
// RateLimiter, Publisher) so it is unit-testable with in-memory fakes.
type Server struct {
	pollv1.UnimplementedPollServiceServer

	repo ports.Repo
	rl   ports.RateLimiter
	pub  ports.Publisher
	tune Tuning
	now  func() time.Time
}

func NewServer(repo ports.Repo, rl ports.RateLimiter, pub ports.Publisher, tune Tuning) *Server {
	if tune.DefaultTTL <= 0 {
		tune.DefaultTTL = 7 * 24 * time.Hour
	}
	if tune.PerTokenBudget <= 0 {
		tune.PerTokenBudget = 5
	}
	if tune.PerTokenWindow <= 0 {
		tune.PerTokenWindow = time.Hour
	}
	if tune.PerIPBudget <= 0 {
		tune.PerIPBudget = 30
	}
	if tune.PerIPWindow <= 0 {
		tune.PerIPWindow = time.Hour
	}
	if tune.LinkPath == "" {
		tune.LinkPath = "/p/{token}"
	}
	return &Server{repo: repo, rl: rl, pub: pub, tune: tune, now: time.Now}
}

// CreatePoll mints a poll and its opaque expiring link (owner-scoped).
func (s *Server) CreatePoll(ctx context.Context, in *pollv1.CreatePollRequest) (*pollv1.CreatePollResponse, error) {
	owner, ok := auth.SubjectFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if strings.TrimSpace(in.GetTitle()) == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	questions := questionsFromProto(in.GetQuestions())
	if err := model.ValidateQuestions(questions); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	ttl := s.tune.DefaultTTL
	if in.GetTtlSeconds() > 0 {
		ttl = time.Duration(in.GetTtlSeconds()) * time.Second
	}
	now := s.now()
	expires := now.Add(ttl)

	raw, hash, err := token.New()
	if err != nil {
		return nil, status.Error(codes.Internal, "could not mint token")
	}

	p := model.Poll{
		ID:                uuid.NewString(),
		OwnerUserID:       owner,
		SurpriseRequestID: in.GetSurpriseRequestId(),
		Title:             in.GetTitle(),
		Questions:         questions,
		Status:            model.StatusActive,
		ExpiresAt:         expires,
		CreatedAt:         now,
	}
	if err := s.repo.CreatePoll(ctx, p, hash, expires); err != nil {
		return nil, status.Error(codes.Internal, "could not create poll")
	}

	return &pollv1.CreatePollResponse{
		PollId:    p.ID,
		LinkToken: raw, // returned exactly once
		LinkUrl:   s.buildLinkURL(raw),
		ExpiresAt: expires.UTC().Format(time.RFC3339),
	}, nil
}

// GetPollByToken resolves a link token to its poll for the anonymous Subject.
func (s *Server) GetPollByToken(ctx context.Context, in *pollv1.GetPollByTokenRequest) (*pollv1.GetPollByTokenResponse, error) {
	raw := in.GetToken()
	if raw == "" {
		return nil, notFound()
	}
	// Anonymous fetch rate limit (per IP) — protects the public surface.
	if err := s.limit(ctx, "rl:fetch:ip:"+s.clientIP(ctx), s.tune.PerIPBudget, s.tune.PerIPWindow); err != nil {
		return nil, err
	}

	lp, err := s.repo.GetByTokenHash(ctx, token.Hash(raw))
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, notFound()
		}
		return nil, status.Error(codes.Internal, "lookup failed")
	}
	if !s.fetchable(lp) {
		return nil, notFound() // expired / revoked / consumed — uniform
	}

	return &pollv1.GetPollByTokenResponse{
		PollId:    lp.Poll.ID,
		Title:     lp.Poll.Title,
		Questions: questionsToProto(lp.Poll.Questions),
	}, nil
}

// SubmitResponse records the Subject's answers (anonymous, rate-limited, one-shot).
func (s *Server) SubmitResponse(ctx context.Context, in *pollv1.SubmitResponseRequest) (*pollv1.SubmitResponseResponse, error) {
	raw := in.GetToken()
	if raw == "" {
		return nil, notFound()
	}
	th := token.Hash(raw)
	ip := s.clientIP(ctx)

	// Rate limit BEFORE any DB work so link-spam bursts never reach Postgres.
	if err := s.limit(ctx, "rl:submit:token:"+th, s.tune.PerTokenBudget, s.tune.PerTokenWindow); err != nil {
		return nil, err
	}
	if err := s.limit(ctx, "rl:submit:ip:"+ip, s.tune.PerIPBudget, s.tune.PerIPWindow); err != nil {
		return nil, err
	}

	lp, err := s.repo.GetByTokenHash(ctx, th)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, notFound()
		}
		return nil, status.Error(codes.Internal, "lookup failed")
	}
	if !s.fetchable(lp) {
		return nil, notFound()
	}

	answers := answersFromProto(in.GetAnswers())
	if err := model.ValidateAnswers(lp.Poll.Questions, answers); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	now := s.now()
	_, err = s.repo.CompleteWithResponse(ctx, lp.Poll.ID, answers, s.fingerprint(ctx, ip), now)
	if err != nil {
		if errors.Is(err, ports.ErrAlreadyCompleted) {
			return nil, notFound() // already consumed — uniform
		}
		return nil, status.Error(codes.Internal, "could not record response")
	}

	// Emit PollCompleted. The DB write is the source of truth; a publish failure
	// is logged rather than surfaced (event delivery is eventually consistent).
	ev := ports.PollCompleted{
		PollID:            lp.Poll.ID,
		SurpriseRequestID: lp.Poll.SurpriseRequestID,
		OwnerUserID:       lp.Poll.OwnerUserID,
		CompletedAt:       now.UTC(),
	}
	if err := s.pub.PublishPollCompleted(ctx, ev); err != nil {
		slog.ErrorContext(ctx, "publish PollCompleted failed", "poll_id", ev.PollID, "err", err)
	}

	return &pollv1.SubmitResponseResponse{Ok: true}, nil
}

// GetResponses returns a poll's responses to its owner only.
func (s *Server) GetResponses(ctx context.Context, in *pollv1.GetResponsesRequest) (*pollv1.GetResponsesResponse, error) {
	owner, ok := auth.SubjectFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if in.GetPollId() == "" {
		return nil, status.Error(codes.InvalidArgument, "poll_id is required")
	}

	p, err := s.repo.GetPollByID(ctx, in.GetPollId())
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, notFound()
		}
		return nil, status.Error(codes.Internal, "lookup failed")
	}
	// Owner-scoped authz: a non-owner gets the same NotFound as a missing poll,
	// so ownership can't be probed.
	if p.OwnerUserID != owner {
		return nil, notFound()
	}

	resps, err := s.repo.GetResponses(ctx, in.GetPollId())
	if err != nil {
		return nil, status.Error(codes.Internal, "could not read responses")
	}

	out := make([]*pollv1.Response, 0, len(resps))
	for _, r := range resps {
		out = append(out, &pollv1.Response{
			Id:          r.ID,
			Answers:     answersToProto(r.Answers),
			SubmittedAt: r.SubmittedAt.UTC().Format(time.RFC3339),
		})
	}
	return &pollv1.GetResponsesResponse{Responses: out}, nil
}

// --- helpers ---

// fetchable applies the uniform validity rules to a resolved link+poll.
func (s *Server) fetchable(lp ports.LinkedPoll) bool {
	now := s.now()
	if lp.LinkRevoked {
		return false
	}
	if now.After(lp.LinkExpiresAt) {
		return false
	}
	if lp.Poll.Status != model.StatusActive {
		return false // draft / completed / expired all read as gone
	}
	if now.After(lp.Poll.ExpiresAt) {
		return false
	}
	return true
}

func (s *Server) limit(ctx context.Context, key string, budget int, window time.Duration) error {
	allowed, err := s.rl.Allow(ctx, key, budget, window)
	if err != nil {
		return status.Error(codes.Internal, "rate limiter unavailable")
	}
	if !allowed {
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}
	return nil
}

func (s *Server) buildLinkURL(raw string) string {
	path := strings.Replace(s.tune.LinkPath, "{token}", raw, 1)
	return strings.TrimRight(s.tune.AllowedOrigin, "/") + path
}

// clientIP extracts a coarse client IP from gateway-forwarded metadata, falling
// back to the gRPC peer address.
func (s *Server) clientIP(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := firstHeader(md, "x-forwarded-for"); v != "" {
			if i := strings.IndexByte(v, ','); i >= 0 {
				return strings.TrimSpace(v[:i])
			}
			return strings.TrimSpace(v)
		}
		if v := firstHeader(md, "x-real-ip"); v != "" {
			return v
		}
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return "unknown"
}

// fingerprint is a coarse anti-abuse marker stored with the response — a hash of
// IP + user-agent, never raw PII.
func (s *Server) fingerprint(ctx context.Context, ip string) string {
	ua := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		ua = firstHeader(md, "grpcgateway-user-agent")
		if ua == "" {
			ua = firstHeader(md, "user-agent")
		}
	}
	sum := sha256.Sum256([]byte(ip + "|" + ua))
	return hex.EncodeToString(sum[:])[:16]
}

func firstHeader(md metadata.MD, key string) string {
	if v := md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}

func notFound() error { return status.Error(codes.NotFound, "not found") }
