package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/events"
	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/surprise/pkg/api/surprise/v1"
)

// ServerConfig carries the request-path tuning knobs.
type ServerConfig struct {
	StatusTTL      time.Duration
	IdempotencyTTL time.Duration
}

// Server implements surprisev1.SurpriseServiceServer. It only does fast
// request-path work — persist + enqueue + cheap reads — and never calls the LLM
// (that happens in the worker). All state goes through the domain interfaces so
// the handlers are unit-tested against fakes.
type Server struct {
	surprisev1.UnimplementedSurpriseServiceServer
	repo  domain.Repository
	cache domain.Cache
	pub   events.Publisher
	cfg   ServerConfig
}

// NewServer builds the gRPC service implementation.
func NewServer(repo domain.Repository, cache domain.Cache, pub events.Publisher, cfg ServerConfig) *Server {
	if cfg.StatusTTL == 0 {
		cfg.StatusTTL = time.Hour
	}
	if cfg.IdempotencyTTL == 0 {
		cfg.IdempotencyTTL = 24 * time.Hour
	}
	return &Server{repo: repo, cache: cache, pub: pub, cfg: cfg}
}

// RequestGeneration validates, dedupes on the idempotency key, persists the
// request (status queued), enqueues the GenerationRequested job, and returns fast
// (HTTP 202). Re-submitting the same idempotency key returns the original
// request without a second enqueue.
func (s *Server) RequestGeneration(ctx context.Context, req *surprisev1.RequestGenerationRequest) (*surprisev1.RequestGenerationResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GetHolidayId() == "" {
		return nil, status.Error(codes.InvalidArgument, "holiday_id is required")
	}
	if req.GetBudgetBand() == "" {
		return nil, status.Error(codes.InvalidArgument, "budget_band is required")
	}
	if req.GetIdempotencyKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}

	// Fast dedupe via Valkey (holds the winning request id); the DB unique index
	// is the durable backstop. Generate the id up front so the cache slot records
	// the real winner.
	newID := uuid.NewString()
	stored, existingID, err := s.cache.SetIdempotencyIfAbsent(ctx, req.GetIdempotencyKey(), newID, s.cfg.IdempotencyTTL)
	if err != nil {
		return nil, status.Error(codes.Internal, "idempotency check failed")
	}
	if !stored {
		return s.existingResponse(ctx, existingID, req.GetIdempotencyKey())
	}

	tier := tierFromProto(req.GetTier())
	r := &domain.Request{
		ID:              newID,
		UserID:          req.GetUserId(),
		HolidayID:       req.GetHolidayId(),
		BudgetBand:      req.GetBudgetBand(),
		PreferencesText: req.GetPreferencesText(),
		PollID:          req.GetPollId(),
		IdempotencyKey:  req.GetIdempotencyKey(),
		Status:          domain.StatusQueued,
		Tier:            tier,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.repo.CreateRequest(ctx, r); err != nil {
		if errors.Is(err, domain.ErrDuplicate) {
			if existing, gerr := s.repo.GetRequestByIdempotencyKey(ctx, req.GetIdempotencyKey()); gerr == nil {
				return &surprisev1.RequestGenerationResponse{RequestId: existing.ID, Status: statusToProto(existing.Status)}, nil
			}
		}
		return nil, status.Error(codes.Internal, "persist request failed")
	}

	_ = s.cache.SetStatus(ctx, r.ID, domain.StatusInfo{Status: domain.StatusQueued, Progress: 0}, s.cfg.StatusTTL)

	if err := s.pub.PublishGenerationRequested(ctx, events.GenerationRequested{RequestID: r.ID, Tier: string(tier)}); err != nil {
		return nil, status.Error(codes.Internal, "enqueue job failed")
	}
	return &surprisev1.RequestGenerationResponse{RequestId: r.ID, Status: surprisev1.GenerationStatus_GENERATION_STATUS_QUEUED}, nil
}

func (s *Server) existingResponse(ctx context.Context, existingID, idemKey string) (*surprisev1.RequestGenerationResponse, error) {
	// The cache slot holds the winning request id; resolve its current status.
	if existingID != "" {
		if r, err := s.repo.GetRequest(ctx, existingID); err == nil {
			return &surprisev1.RequestGenerationResponse{RequestId: r.ID, Status: statusToProto(r.Status)}, nil
		}
	}
	if r, err := s.repo.GetRequestByIdempotencyKey(ctx, idemKey); err == nil {
		return &surprisev1.RequestGenerationResponse{RequestId: r.ID, Status: statusToProto(r.Status)}, nil
	}
	return &surprisev1.RequestGenerationResponse{RequestId: existingID, Status: surprisev1.GenerationStatus_GENERATION_STATUS_QUEUED}, nil
}

// GetGenerationStatus is a cheap poll backed by Valkey, falling back to the DB.
func (s *Server) GetGenerationStatus(ctx context.Context, req *surprisev1.GetGenerationStatusRequest) (*surprisev1.GetGenerationStatusResponse, error) {
	if req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	r, err := s.repo.GetRequest(ctx, req.GetRequestId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "request not found")
	}
	if err := s.requireOwner(ctx, r.UserID); err != nil {
		return nil, err
	}
	if info, cerr := s.cache.GetStatus(ctx, req.GetRequestId()); cerr == nil {
		return &surprisev1.GetGenerationStatusResponse{Status: statusToProto(info.Status), Progress: uint32(info.Progress)}, nil
	}
	return &surprisev1.GetGenerationStatusResponse{Status: statusToProto(r.Status)}, nil
}

// GetIdeas returns the ranked ideas for an owned request (empty until ready).
func (s *Server) GetIdeas(ctx context.Context, req *surprisev1.GetIdeasRequest) (*surprisev1.GetIdeasResponse, error) {
	if req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	r, err := s.repo.GetRequest(ctx, req.GetRequestId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "request not found")
	}
	if err := s.requireOwner(ctx, r.UserID); err != nil {
		return nil, err
	}
	ideas, err := s.repo.GetIdeas(ctx, req.GetRequestId())
	if err != nil {
		return nil, status.Error(codes.Internal, "load ideas failed")
	}
	out := make([]*surprisev1.Idea, 0, len(ideas))
	for _, i := range ideas {
		out = append(out, ideaToProto(i))
	}
	return &surprisev1.GetIdeasResponse{Ideas: out}, nil
}

// SaveIdea favorites an idea for the caller (owner-scoped on the idea's request).
func (s *Server) SaveIdea(ctx context.Context, req *surprisev1.SaveIdeaRequest) (*surprisev1.SaveIdeaResponse, error) {
	if req.GetUserId() == "" || req.GetIdeaId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and idea_id are required")
	}
	if err := s.requireOwner(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	idea, err := s.repo.GetIdea(ctx, req.GetIdeaId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "idea not found")
	}
	owner, err := s.repo.GetRequest(ctx, idea.RequestID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "idea's request not found")
	}
	if owner.UserID != req.GetUserId() {
		return nil, status.Error(codes.PermissionDenied, "idea does not belong to caller")
	}
	if err := s.repo.SaveIdeaForUser(ctx, req.GetUserId(), req.GetIdeaId()); err != nil {
		return nil, status.Error(codes.Internal, "save idea failed")
	}
	return &surprisev1.SaveIdeaResponse{Ok: true}, nil
}

// Refine re-queues a request with an adjustment (owner-scoped).
func (s *Server) Refine(ctx context.Context, req *surprisev1.RefineRequest) (*surprisev1.RefineResponse, error) {
	if req.GetRequestId() == "" || req.GetRefinement() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id and refinement are required")
	}
	r, err := s.repo.GetRequest(ctx, req.GetRequestId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "request not found")
	}
	if err := s.requireOwner(ctx, r.UserID); err != nil {
		return nil, err
	}
	if err := s.repo.MarkRefinement(ctx, r.ID, req.GetRefinement()); err != nil {
		return nil, status.Error(codes.Internal, "mark refinement failed")
	}
	_ = s.cache.SetStatus(ctx, r.ID, domain.StatusInfo{Status: domain.StatusQueued, Progress: 0}, s.cfg.StatusTTL)
	if err := s.pub.PublishGenerationRequested(ctx, events.GenerationRequested{RequestID: r.ID, Tier: string(r.Tier)}); err != nil {
		return nil, status.Error(codes.Internal, "enqueue refine job failed")
	}
	return &surprisev1.RefineResponse{RequestId: r.ID, Status: surprisev1.GenerationStatus_GENERATION_STATUS_QUEUED}, nil
}

// requireOwner enforces that the caller (JWT subject, injected by the gateway as
// metadata "x-user-id") owns the target. When no caller identity is present
// (e.g. trusted internal call) the check is skipped.
func (s *Server) requireOwner(ctx context.Context, ownerUserID string) error {
	caller := callerID(ctx)
	if caller == "" {
		return nil
	}
	if caller != ownerUserID {
		return status.Error(codes.PermissionDenied, "caller does not own this resource")
	}
	return nil
}

func callerID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if v := md.Get("x-user-id"); len(v) > 0 {
		return v[0]
	}
	return ""
}

// --- proto <-> domain mapping ---

func tierFromProto(t surprisev1.ModelTier) domain.Tier {
	if t == surprisev1.ModelTier_MODEL_TIER_OPUS {
		return domain.TierOpus
	}
	return domain.TierSonnet
}

func statusToProto(s domain.Status) surprisev1.GenerationStatus {
	switch s {
	case domain.StatusQueued:
		return surprisev1.GenerationStatus_GENERATION_STATUS_QUEUED
	case domain.StatusRunning:
		return surprisev1.GenerationStatus_GENERATION_STATUS_RUNNING
	case domain.StatusReady:
		return surprisev1.GenerationStatus_GENERATION_STATUS_READY
	case domain.StatusFailed:
		return surprisev1.GenerationStatus_GENERATION_STATUS_FAILED
	default:
		return surprisev1.GenerationStatus_GENERATION_STATUS_UNSPECIFIED
	}
}

func moderationToProto(m domain.Moderation) surprisev1.ModerationStatus {
	switch m {
	case domain.ModerationApproved:
		return surprisev1.ModerationStatus_MODERATION_STATUS_APPROVED
	case domain.ModerationRejected:
		return surprisev1.ModerationStatus_MODERATION_STATUS_REJECTED
	default:
		return surprisev1.ModerationStatus_MODERATION_STATUS_UNSPECIFIED
	}
}

func ideaToProto(i domain.Idea) *surprisev1.Idea {
	return &surprisev1.Idea{
		Id:               i.ID,
		RequestId:        i.RequestID,
		Title:            i.Title,
		WhyItFits:        i.WhyItFits,
		RoughCost:        i.RoughCost,
		HowTo:            i.HowTo,
		Rank:             uint32(i.Rank),
		ModerationStatus: moderationToProto(i.Moderation),
	}
}
