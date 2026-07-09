package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/domain/model"
	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/catalog/pkg/api/catalog/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fakes ---

type fakeRef struct {
	holidays  []model.Holiday
	cats      []model.Category
	bands     []model.BudgetBand
	gotFilter model.HolidayFilter
	gotKind   string
	listCalls int
	failWith  error
}

func (f *fakeRef) ListHolidays(_ context.Context, flt model.HolidayFilter) ([]model.Holiday, error) {
	f.listCalls++
	f.gotFilter = flt
	if f.failWith != nil {
		return nil, f.failWith
	}
	return f.holidays, nil
}

func (f *fakeRef) GetCategories(_ context.Context, kind string) ([]model.Category, []model.BudgetBand, error) {
	f.gotKind = kind
	if f.failWith != nil {
		return nil, nil, f.failWith
	}
	return f.cats, f.bands, nil
}

type fakeCorpus struct {
	snippets   []model.Snippet
	upsertID   string
	gotEmbed   []float32
	gotFilter  model.SearchFilter
	gotTopK    int
	gotUpsert  model.Inspiration
	gotUpEmbed []float32
	searchErr  error
	upsertErr  error
}

func (f *fakeCorpus) SearchInspiration(_ context.Context, emb []float32, flt model.SearchFilter, topK int) ([]model.Snippet, error) {
	f.gotEmbed = emb
	f.gotFilter = flt
	f.gotTopK = topK
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.snippets, nil
}

func (f *fakeCorpus) UpsertInspiration(_ context.Context, in model.Inspiration, emb []float32) (string, error) {
	f.gotUpsert = in
	f.gotUpEmbed = emb
	if f.upsertErr != nil {
		return "", f.upsertErr
	}
	return f.upsertID, nil
}

type fakeCache struct {
	data            map[string][]byte
	invalidateCalls int
	failGet         bool
}

func newFakeCache() *fakeCache { return &fakeCache{data: map[string][]byte{}} }

func (f *fakeCache) GetJSON(_ context.Context, key string, dst any) (bool, error) {
	if f.failGet {
		return false, errors.New("boom")
	}
	raw, ok := f.data[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

func (f *fakeCache) SetJSON(_ context.Context, key string, v any, _ time.Duration) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f.data[key] = raw
	return nil
}

func (f *fakeCache) Invalidate(_ context.Context) error {
	f.invalidateCalls++
	f.data = map[string][]byte{}
	return nil
}

type fakeEmbedder struct {
	dim      int
	calls    int
	gotTexts []string
	failWith error
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.calls++
	f.gotTexts = append(f.gotTexts, texts...)
	if f.failWith != nil {
		return nil, f.failWith
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, f.dim)
		for j := range v {
			v[j] = float32(i + 1)
		}
		out[i] = v
	}
	return out, nil
}
func (f *fakeEmbedder) Model() string  { return "fake" }
func (f *fakeEmbedder) Dimension() int { return f.dim }

func newTestServer(ref *fakeRef, corpus *fakeCorpus, cache *fakeCache, emb *fakeEmbedder) *Server {
	return NewServer(ref, corpus, cache, emb, Tuning{
		ReferenceCacheTTL: time.Minute,
		DefaultTopK:       8,
		MaxTopK:           20,
	})
}

func boolp(b bool) *bool { return &b }

// --- ListHolidays ---

func TestListHolidays_ReturnsRowsAndPassesFilter(t *testing.T) {
	ref := &fakeRef{holidays: []model.Holiday{
		{ID: "h1", Name: "Valentine's Day", DateRule: model.DateRuleFixed, Region: "US", Tags: []string{"romance"}, Active: true},
	}}
	s := newTestServer(ref, &fakeCorpus{}, newFakeCache(), &fakeEmbedder{dim: 4})

	resp, err := s.ListHolidays(context.Background(), &catalogv1.ListHolidaysRequest{
		Region: "US", Active: boolp(true),
	})
	if err != nil {
		t.Fatalf("ListHolidays: %v", err)
	}
	if len(resp.GetHolidays()) != 1 {
		t.Fatalf("got %d holidays, want 1", len(resp.GetHolidays()))
	}
	h := resp.GetHolidays()[0]
	if h.GetName() != "Valentine's Day" || h.GetDateRule() != catalogv1.DateRule_DATE_RULE_FIXED {
		t.Fatalf("bad mapping: %+v", h)
	}
	if ref.gotFilter.Region != "US" || ref.gotFilter.Active == nil || *ref.gotFilter.Active != true {
		t.Fatalf("filter not passed through: %+v", ref.gotFilter)
	}
}

func TestListHolidays_CacheHitSkipsStore(t *testing.T) {
	cache := newFakeCache()
	ref := &fakeRef{failWith: errors.New("store must not be called on hit")}
	s := newTestServer(ref, &fakeCorpus{}, cache, &fakeEmbedder{dim: 4})

	// Pre-seed the cache under the exact key ListHolidays will compute.
	cached := []model.Holiday{{ID: "h9", Name: "Cached", DateRule: model.DateRuleFixed}}
	if err := cache.SetJSON(context.Background(), "holidays::any:", cached, time.Minute); err != nil {
		t.Fatal(err)
	}

	resp, err := s.ListHolidays(context.Background(), &catalogv1.ListHolidaysRequest{})
	if err != nil {
		t.Fatalf("ListHolidays: %v", err)
	}
	if len(resp.GetHolidays()) != 1 || resp.GetHolidays()[0].GetName() != "Cached" {
		t.Fatalf("expected cached result, got %+v", resp.GetHolidays())
	}
	if ref.listCalls != 0 {
		t.Fatalf("store was called %d times on cache hit", ref.listCalls)
	}
}

func TestListHolidays_CacheMissPopulates(t *testing.T) {
	cache := newFakeCache()
	ref := &fakeRef{holidays: []model.Holiday{{ID: "h1", Name: "X", DateRule: model.DateRuleFixed}}}
	s := newTestServer(ref, &fakeCorpus{}, cache, &fakeEmbedder{dim: 4})

	if _, err := s.ListHolidays(context.Background(), &catalogv1.ListHolidaysRequest{}); err != nil {
		t.Fatalf("ListHolidays: %v", err)
	}
	if _, ok := cache.data["holidays::any:"]; !ok {
		t.Fatalf("cache not populated after miss; keys=%v", keys(cache.data))
	}
	if ref.listCalls != 1 {
		t.Fatalf("store called %d times, want 1", ref.listCalls)
	}
}

func TestListHolidays_StoreErrorIsInternal(t *testing.T) {
	ref := &fakeRef{failWith: errors.New("db down")}
	s := newTestServer(ref, &fakeCorpus{}, newFakeCache(), &fakeEmbedder{dim: 4})
	_, err := s.ListHolidays(context.Background(), &catalogv1.ListHolidaysRequest{})
	assertCode(t, err, codes.Internal)
}

// --- GetCategories ---

func TestGetCategories_ReturnsCatsAndBandsWithKindFilter(t *testing.T) {
	ref := &fakeRef{
		cats:  []model.Category{{ID: "c1", Name: "Jewelry", Kind: model.KindGift}},
		bands: []model.BudgetBand{{ID: "b1", Label: "Under $50", MinCents: 0, MaxCents: 5000, Currency: "USD"}},
	}
	s := newTestServer(ref, &fakeCorpus{}, newFakeCache(), &fakeEmbedder{dim: 4})

	resp, err := s.GetCategories(context.Background(), &catalogv1.GetCategoriesRequest{
		Kind: catalogv1.CategoryKind_CATEGORY_KIND_GIFT,
	})
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}
	if len(resp.GetCategories()) != 1 || resp.GetCategories()[0].GetKind() != catalogv1.CategoryKind_CATEGORY_KIND_GIFT {
		t.Fatalf("bad categories: %+v", resp.GetCategories())
	}
	if len(resp.GetBudgetBands()) != 1 || resp.GetBudgetBands()[0].GetMaxCents() != 5000 {
		t.Fatalf("bad bands: %+v", resp.GetBudgetBands())
	}
	if ref.gotKind != model.KindGift {
		t.Fatalf("kind filter = %q, want gift", ref.gotKind)
	}
}

// --- SearchInspiration ---

func TestSearchInspiration_QueryTextEmbedsThenSearches(t *testing.T) {
	corpus := &fakeCorpus{snippets: []model.Snippet{{ID: "i1", Title: "Sunset picnic", Score: 0.9}}}
	emb := &fakeEmbedder{dim: 4}
	s := newTestServer(&fakeRef{}, corpus, newFakeCache(), emb)

	resp, err := s.SearchInspiration(context.Background(), &catalogv1.SearchInspirationRequest{
		QueryText: "romantic evening",
		Filters:   &catalogv1.SearchFilters{CategoryId: "c1", BudgetBandId: "b2"},
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("SearchInspiration: %v", err)
	}
	if emb.calls != 1 || len(emb.gotTexts) != 1 || emb.gotTexts[0] != "romantic evening" {
		t.Fatalf("embedder not called with query text: %+v", emb.gotTexts)
	}
	if len(corpus.gotEmbed) != 4 {
		t.Fatalf("corpus got embedding dim %d, want 4", len(corpus.gotEmbed))
	}
	if corpus.gotTopK != 5 || corpus.gotFilter.CategoryID != "c1" || corpus.gotFilter.BudgetBandID != "b2" {
		t.Fatalf("filters/topK not passed: topK=%d filter=%+v", corpus.gotTopK, corpus.gotFilter)
	}
	if len(resp.GetSnippets()) != 1 || resp.GetSnippets()[0].GetScore() != 0.9 {
		t.Fatalf("bad snippet mapping: %+v", resp.GetSnippets())
	}
}

func TestSearchInspiration_QueryEmbeddingSkipsEmbedder(t *testing.T) {
	corpus := &fakeCorpus{}
	emb := &fakeEmbedder{dim: 3}
	s := newTestServer(&fakeRef{}, corpus, newFakeCache(), emb)

	given := []float32{0.1, 0.2, 0.3}
	_, err := s.SearchInspiration(context.Background(), &catalogv1.SearchInspirationRequest{
		QueryEmbedding: given,
	})
	if err != nil {
		t.Fatalf("SearchInspiration: %v", err)
	}
	if emb.calls != 0 {
		t.Fatalf("embedder called %d times, want 0 when embedding supplied", emb.calls)
	}
	if len(corpus.gotEmbed) != 3 || corpus.gotEmbed[1] != 0.2 {
		t.Fatalf("supplied embedding not forwarded: %+v", corpus.gotEmbed)
	}
}

func TestSearchInspiration_DimensionMismatch(t *testing.T) {
	s := newTestServer(&fakeRef{}, &fakeCorpus{}, newFakeCache(), &fakeEmbedder{dim: 4})
	_, err := s.SearchInspiration(context.Background(), &catalogv1.SearchInspirationRequest{
		QueryEmbedding: []float32{1, 2}, // dim 2 != 4
	})
	assertCode(t, err, codes.InvalidArgument)
}

func TestSearchInspiration_NeitherProvided(t *testing.T) {
	s := newTestServer(&fakeRef{}, &fakeCorpus{}, newFakeCache(), &fakeEmbedder{dim: 4})
	_, err := s.SearchInspiration(context.Background(), &catalogv1.SearchInspirationRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

func TestSearchInspiration_BothProvided(t *testing.T) {
	s := newTestServer(&fakeRef{}, &fakeCorpus{}, newFakeCache(), &fakeEmbedder{dim: 4})
	_, err := s.SearchInspiration(context.Background(), &catalogv1.SearchInspirationRequest{
		QueryText:      "x",
		QueryEmbedding: []float32{1, 2, 3, 4},
	})
	assertCode(t, err, codes.InvalidArgument)
}

func TestSearchInspiration_TopKDefaultedAndCapped(t *testing.T) {
	corpus := &fakeCorpus{}
	s := newTestServer(&fakeRef{}, corpus, newFakeCache(), &fakeEmbedder{dim: 4})

	// topK unset -> default 8
	if _, err := s.SearchInspiration(context.Background(), &catalogv1.SearchInspirationRequest{QueryText: "a"}); err != nil {
		t.Fatal(err)
	}
	if corpus.gotTopK != 8 {
		t.Fatalf("default topK = %d, want 8", corpus.gotTopK)
	}
	// topK huge -> capped at MaxTopK (20)
	if _, err := s.SearchInspiration(context.Background(), &catalogv1.SearchInspirationRequest{QueryText: "a", TopK: 1000}); err != nil {
		t.Fatal(err)
	}
	if corpus.gotTopK != 20 {
		t.Fatalf("capped topK = %d, want 20", corpus.gotTopK)
	}
}

// --- UpsertInspiration ---

func TestUpsertInspiration_EmbedsStoresInvalidates(t *testing.T) {
	corpus := &fakeCorpus{upsertID: "generated-id"}
	cache := newFakeCache()
	emb := &fakeEmbedder{dim: 4}
	s := newTestServer(&fakeRef{}, corpus, cache, emb)

	resp, err := s.UpsertInspiration(context.Background(), &catalogv1.UpsertInspirationRequest{
		Inspiration: &catalogv1.Inspiration{
			Title: "Handwritten letter", Body: "A heartfelt note...", CategoryId: "c1",
			Tags: []string{"sentimental"}, CuratedBy: "editor@pg", Active: true,
		},
	})
	if err != nil {
		t.Fatalf("UpsertInspiration: %v", err)
	}
	if resp.GetId() != "generated-id" {
		t.Fatalf("id = %q, want generated-id", resp.GetId())
	}
	if emb.calls != 1 {
		t.Fatalf("embedder calls = %d, want 1", emb.calls)
	}
	if len(corpus.gotUpEmbed) != 4 {
		t.Fatalf("stored embedding dim = %d, want 4", len(corpus.gotUpEmbed))
	}
	if corpus.gotUpsert.Title != "Handwritten letter" || corpus.gotUpsert.CategoryID != "c1" {
		t.Fatalf("model not forwarded: %+v", corpus.gotUpsert)
	}
	if cache.invalidateCalls != 1 {
		t.Fatalf("cache invalidate calls = %d, want 1", cache.invalidateCalls)
	}
}

func TestUpsertInspiration_MissingTitleOrBody(t *testing.T) {
	s := newTestServer(&fakeRef{}, &fakeCorpus{}, newFakeCache(), &fakeEmbedder{dim: 4})
	cases := []*catalogv1.Inspiration{
		{Title: "", Body: "b"},
		{Title: "t", Body: ""},
	}
	for i, c := range cases {
		_, err := s.UpsertInspiration(context.Background(), &catalogv1.UpsertInspirationRequest{Inspiration: c})
		assertCodef(t, err, codes.InvalidArgument, fmt.Sprintf("case %d", i))
	}
	_, err := s.UpsertInspiration(context.Background(), &catalogv1.UpsertInspirationRequest{})
	assertCode(t, err, codes.InvalidArgument)
}

// --- helpers ---

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	assertCodef(t, err, want, "")
}

func assertCodef(t *testing.T, err error, want codes.Code, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error with code %v, got nil", msg, want)
	}
	if status.Code(err) != want {
		t.Fatalf("%s: got code %v, want %v (err=%v)", msg, status.Code(err), want, err)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
