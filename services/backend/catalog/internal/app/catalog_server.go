package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/domain/model"
	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/embedding"
	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/catalog/pkg/api/catalog/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ReferenceStore serves cached client reference reads.
type ReferenceStore interface {
	ListHolidays(ctx context.Context, f model.HolidayFilter) ([]model.Holiday, error)
	GetCategories(ctx context.Context, kind string) ([]model.Category, []model.BudgetBand, error)
}

// InspirationStore owns the curated corpus and its pgvector search/write path.
type InspirationStore interface {
	SearchInspiration(ctx context.Context, embedding []float32, f model.SearchFilter, topK int) ([]model.Snippet, error)
	UpsertInspiration(ctx context.Context, in model.Inspiration, embedding []float32) (string, error)
}

// Cache is the best-effort reference-read cache. Any error is treated as a miss.
type Cache interface {
	GetJSON(ctx context.Context, key string, dst any) (bool, error)
	SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error
	Invalidate(ctx context.Context) error
}

// Tuning holds the catalog knobs.
type Tuning struct {
	ReferenceCacheTTL time.Duration
	DefaultTopK       int
	MaxTopK           int
}

// Server implements catalogv1.CatalogServiceServer.
type Server struct {
	catalogv1.UnimplementedCatalogServiceServer

	ref      ReferenceStore
	corpus   InspirationStore
	cache    Cache
	embedder embedding.Embedder
	tune     Tuning
}

// NewServer builds the RPC implementation with sane tuning defaults.
func NewServer(ref ReferenceStore, corpus InspirationStore, cache Cache, embedder embedding.Embedder, tune Tuning) *Server {
	if tune.DefaultTopK <= 0 {
		tune.DefaultTopK = 8
	}
	if tune.MaxTopK <= 0 {
		tune.MaxTopK = 50
	}
	if tune.ReferenceCacheTTL <= 0 {
		tune.ReferenceCacheTTL = time.Hour
	}
	return &Server{ref: ref, corpus: corpus, cache: cache, embedder: embedder, tune: tune}
}

// ListHolidays returns reference holidays, cache-first.
func (s *Server) ListHolidays(ctx context.Context, in *catalogv1.ListHolidaysRequest) (*catalogv1.ListHolidaysResponse, error) {
	f := model.HolidayFilter{Region: in.GetRegion(), OnOrAfter: in.GetOnOrAfter()}
	if in.Active != nil {
		v := in.GetActive()
		f.Active = &v
	}

	cacheKey := fmt.Sprintf("holidays:%s:%s:%s", f.Region, activeKey(f.Active), f.OnOrAfter)

	var holidays []model.Holiday
	if hit := s.cacheGet(ctx, cacheKey, &holidays); !hit {
		var err error
		holidays, err = s.ref.ListHolidays(ctx, f)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list holidays: %v", err)
		}
		s.cacheSet(ctx, cacheKey, holidays)
	}

	resp := &catalogv1.ListHolidaysResponse{Holidays: make([]*catalogv1.Holiday, 0, len(holidays))}
	for _, h := range holidays {
		resp.Holidays = append(resp.Holidays, holidayToProto(h))
	}
	return resp, nil
}

// GetCategories returns categories + budget bands, cache-first.
func (s *Server) GetCategories(ctx context.Context, in *catalogv1.GetCategoriesRequest) (*catalogv1.GetCategoriesResponse, error) {
	kind := kindFromProto(in.GetKind())
	cacheKey := "categories:" + kind

	type payload struct {
		Categories []model.Category
		Bands      []model.BudgetBand
	}
	var p payload
	if hit := s.cacheGet(ctx, cacheKey, &p); !hit {
		cats, bands, err := s.ref.GetCategories(ctx, kind)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get categories: %v", err)
		}
		p = payload{Categories: cats, Bands: bands}
		s.cacheSet(ctx, cacheKey, p)
	}

	resp := &catalogv1.GetCategoriesResponse{
		Categories:  make([]*catalogv1.Category, 0, len(p.Categories)),
		BudgetBands: make([]*catalogv1.BudgetBand, 0, len(p.Bands)),
	}
	for _, c := range p.Categories {
		resp.Categories = append(resp.Categories, categoryToProto(c))
	}
	for _, b := range p.Bands {
		resp.BudgetBands = append(resp.BudgetBands, bandToProto(b))
	}
	return resp, nil
}

// SearchInspiration runs pgvector similarity search for the Surprise service.
// Exactly one of query_text / query_embedding must be provided.
func (s *Server) SearchInspiration(ctx context.Context, in *catalogv1.SearchInspirationRequest) (*catalogv1.SearchInspirationResponse, error) {
	text := in.GetQueryText()
	given := in.GetQueryEmbedding()

	switch {
	case text == "" && len(given) == 0:
		return nil, status.Error(codes.InvalidArgument, "provide query_text or query_embedding")
	case text != "" && len(given) > 0:
		return nil, status.Error(codes.InvalidArgument, "provide exactly one of query_text or query_embedding, not both")
	}

	var vec []float32
	if len(given) > 0 {
		if len(given) != s.embedder.Dimension() {
			return nil, status.Errorf(codes.InvalidArgument,
				"query_embedding has dimension %d, expected %d", len(given), s.embedder.Dimension())
		}
		vec = given
	} else {
		embedded, err := s.embedder.Embed(ctx, []string{text})
		if err != nil {
			return nil, status.Errorf(codes.Unavailable, "embed query: %v", err)
		}
		if len(embedded) != 1 {
			return nil, status.Error(codes.Internal, "embedder returned no vector")
		}
		vec = embedded[0]
	}

	topK := int(in.GetTopK())
	if topK <= 0 {
		topK = s.tune.DefaultTopK
	}
	if topK > s.tune.MaxTopK {
		topK = s.tune.MaxTopK
	}

	filter := model.SearchFilter{}
	if fp := in.GetFilters(); fp != nil {
		filter.CategoryID = fp.GetCategoryId()
		filter.BudgetBandID = fp.GetBudgetBandId()
	}

	snippets, err := s.corpus.SearchInspiration(ctx, vec, filter, topK)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search inspiration: %v", err)
	}

	resp := &catalogv1.SearchInspirationResponse{Snippets: make([]*catalogv1.Snippet, 0, len(snippets))}
	for _, sn := range snippets {
		resp.Snippets = append(resp.Snippets, snippetToProto(sn))
	}
	return resp, nil
}

// UpsertInspiration (re)computes the embedding, writes the row, and invalidates
// the reference cache.
func (s *Server) UpsertInspiration(ctx context.Context, in *catalogv1.UpsertInspirationRequest) (*catalogv1.UpsertInspirationResponse, error) {
	insp := in.GetInspiration()
	if insp == nil {
		return nil, status.Error(codes.InvalidArgument, "inspiration is required")
	}
	if insp.GetTitle() == "" || insp.GetBody() == "" {
		return nil, status.Error(codes.InvalidArgument, "inspiration title and body are required")
	}

	embedded, err := s.embedder.Embed(ctx, []string{embedText(insp.GetTitle(), insp.GetBody())})
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "embed inspiration: %v", err)
	}
	if len(embedded) != 1 {
		return nil, status.Error(codes.Internal, "embedder returned no vector")
	}

	m := model.Inspiration{
		ID:           insp.GetId(),
		Title:        insp.GetTitle(),
		Body:         insp.GetBody(),
		CategoryID:   insp.GetCategoryId(),
		BudgetBandID: insp.GetBudgetBandId(),
		Tags:         insp.GetTags(),
		CuratedBy:    insp.GetCuratedBy(),
		Active:       insp.GetActive(),
	}
	id, err := s.corpus.UpsertInspiration(ctx, m, embedded[0])
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert inspiration: %v", err)
	}

	// Best-effort cache invalidation on the rare editorial write (per contract).
	if err := s.cache.Invalidate(ctx); err != nil {
		slog.WarnContext(ctx, "cache invalidate after upsert failed", "err", err)
	}

	return &catalogv1.UpsertInspirationResponse{Id: id}, nil
}

// --- cache helpers (best-effort: any error is a miss / no-op) ---

func (s *Server) cacheGet(ctx context.Context, key string, dst any) bool {
	if s.cache == nil {
		return false
	}
	hit, err := s.cache.GetJSON(ctx, key, dst)
	if err != nil {
		slog.WarnContext(ctx, "cache get failed", "key", key, "err", err)
		return false
	}
	return hit
}

func (s *Server) cacheSet(ctx context.Context, key string, v any) {
	if s.cache == nil {
		return
	}
	if err := s.cache.SetJSON(ctx, key, v, s.tune.ReferenceCacheTTL); err != nil {
		slog.WarnContext(ctx, "cache set failed", "key", key, "err", err)
	}
}

func embedText(title, body string) string { return title + "\n" + body }

func activeKey(a *bool) string {
	if a == nil {
		return "any"
	}
	if *a {
		return "true"
	}
	return "false"
}
