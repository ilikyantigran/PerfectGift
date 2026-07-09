package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/domain/model"
)

// These tests exercise the real SQL (including the pgvector `<=>` search path)
// against a live Postgres+pgvector instance. They are HERMETIC BY DEFAULT: with
// no CATALOG_TEST_DB_DSN set they skip, so `go test ./...` stays green with no DB,
// network, or Docker. To run them, point CATALOG_TEST_DB_DSN at a database that
// has had migrations/0001_init.sql applied, e.g.:
//
//	CATALOG_TEST_DB_DSN='postgres://postgres:postgres@localhost:5432/catalog?sslmode=disable' \
//	    go test ./internal/domain/postgres/...
//
// The test seeds and cleans its own rows; it does not run migrations.
func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("CATALOG_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CATALOG_TEST_DB_DSN not set; skipping Postgres integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := NewStore(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestVectorLiteral(t *testing.T) {
	got := VectorLiteral([]float32{0.5, -1, 2.25})
	want := "[0.5,-1,2.25]"
	if got != want {
		t.Fatalf("VectorLiteral = %q, want %q", got, want)
	}
	if VectorLiteral(nil) != "[]" {
		t.Fatalf("VectorLiteral(nil) = %q, want []", VectorLiteral(nil))
	}
}

func TestIntegration_HolidaysAndCategories(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Pool().Exec(ctx, `INSERT INTO catalog.holidays (name, date_rule, region, tags, active)
		VALUES ('IT_Valentines','fixed','US','["romance"]'::jsonb, true),
		       ('IT_Inactive','fixed','US','[]'::jsonb, false)`)
	if err != nil {
		t.Fatalf("seed holidays: %v", err)
	}
	t.Cleanup(func() { _, _ = s.Pool().Exec(ctx, `DELETE FROM catalog.holidays WHERE name LIKE 'IT_%'`) })

	active := true
	hs, err := s.ListHolidays(ctx, model.HolidayFilter{Region: "US", Active: &active})
	if err != nil {
		t.Fatalf("ListHolidays: %v", err)
	}
	var foundActive, foundInactive bool
	for _, h := range hs {
		if h.Name == "IT_Valentines" {
			foundActive = true
			if len(h.Tags) != 1 || h.Tags[0] != "romance" {
				t.Fatalf("tags not decoded: %+v", h.Tags)
			}
		}
		if h.Name == "IT_Inactive" {
			foundInactive = true
		}
	}
	if !foundActive || foundInactive {
		t.Fatalf("active filter wrong: foundActive=%v foundInactive=%v", foundActive, foundInactive)
	}

	_, err = s.Pool().Exec(ctx, `INSERT INTO catalog.categories (name, kind) VALUES ('IT_Jewelry','gift')`)
	if err != nil {
		t.Fatalf("seed category: %v", err)
	}
	_, err = s.Pool().Exec(ctx, `INSERT INTO catalog.budget_bands (label, min_cents, max_cents, currency) VALUES ('IT_Under50',0,5000,'USD')`)
	if err != nil {
		t.Fatalf("seed band: %v", err)
	}
	t.Cleanup(func() {
		_, _ = s.Pool().Exec(ctx, `DELETE FROM catalog.categories WHERE name LIKE 'IT_%'`)
		_, _ = s.Pool().Exec(ctx, `DELETE FROM catalog.budget_bands WHERE label LIKE 'IT_%'`)
	})

	cats, bands, err := s.GetCategories(ctx, "gift")
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}
	if !hasCategory(cats, "IT_Jewelry") {
		t.Fatalf("gift category not returned: %+v", cats)
	}
	if !hasBand(bands, "IT_Under50") {
		t.Fatalf("budget band not returned: %+v", bands)
	}
}

func TestIntegration_UpsertAndSearchInspiration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	dim := 1536

	near := make([]float32, dim)
	near[0] = 1
	far := make([]float32, dim)
	far[1] = 1

	idA, err := s.UpsertInspiration(ctx, model.Inspiration{
		Title: "IT_Near", Body: "close vector", Tags: []string{"a"}, CuratedBy: "test", Active: true,
	}, near)
	if err != nil {
		t.Fatalf("upsert near: %v", err)
	}
	idB, err := s.UpsertInspiration(ctx, model.Inspiration{
		Title: "IT_Far", Body: "far vector", Active: true,
	}, far)
	if err != nil {
		t.Fatalf("upsert far: %v", err)
	}
	t.Cleanup(func() { _, _ = s.Pool().Exec(ctx, `DELETE FROM catalog.inspiration WHERE title LIKE 'IT_%'`) })

	if idA == "" || idB == "" || idA == idB {
		t.Fatalf("bad ids: %q %q", idA, idB)
	}

	// Query with the exact "near" vector: IT_Near must rank first with score ~1.
	snips, err := s.SearchInspiration(ctx, near, model.SearchFilter{}, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(snips) == 0 || snips[0].Title != "IT_Near" {
		t.Fatalf("nearest first failed: %+v", snips)
	}
	if snips[0].Score < 0.99 {
		t.Fatalf("expected score ~1.0 for identical vector, got %v", snips[0].Score)
	}

	// Update in place: same id, new body.
	idA2, err := s.UpsertInspiration(ctx, model.Inspiration{
		ID: idA, Title: "IT_Near", Body: "updated body", Active: true,
	}, near)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if idA2 != idA {
		t.Fatalf("update changed id: %q -> %q", idA, idA2)
	}
}

func hasCategory(cs []model.Category, name string) bool {
	for _, c := range cs {
		if c.Name == name {
			return true
		}
	}
	return false
}

func hasBand(bs []model.BudgetBand, label string) bool {
	for _, b := range bs {
		if b.Label == label {
			return true
		}
	}
	return false
}
