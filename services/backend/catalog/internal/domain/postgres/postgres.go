// Package postgres is the sole owner of the catalog schema: reference data
// (holidays, categories, budget bands) and the curated inspiration corpus with
// its pgvector embeddings. It exposes narrow, intent-named methods; the gRPC
// server goes through these and never touches the driver directly.
//
// Embeddings are passed as []float32 and rendered to pgvector's text literal
// form ("[v1,v2,...]") and cast with ::vector in SQL, so the package needs only
// the pgx driver — no extra pgvector codec dependency.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/domain/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store owns the catalog schema via a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore opens a pooled connection to Postgres and verifies connectivity.
func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the underlying pool (used by the integration test to seed data).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// ListHolidays returns holidays matching the filter, ordered by name.
func (s *Store) ListHolidays(ctx context.Context, f model.HolidayFilter) ([]model.Holiday, error) {
	const q = `
		SELECT id::text, name, date_rule, region, tags, active
		FROM catalog.holidays
		WHERE ($1 = '' OR region = $1)
		  AND ($2::boolean IS NULL OR active = $2::boolean)
		ORDER BY name`
	rows, err := s.pool.Query(ctx, q, f.Region, f.Active)
	if err != nil {
		return nil, fmt.Errorf("query holidays: %w", err)
	}
	defer rows.Close()

	var out []model.Holiday
	for rows.Next() {
		var h model.Holiday
		var tags []byte
		if err := rows.Scan(&h.ID, &h.Name, &h.DateRule, &h.Region, &tags, &h.Active); err != nil {
			return nil, fmt.Errorf("scan holiday: %w", err)
		}
		if h.Tags, err = decodeTags(tags); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetCategories returns categories (optionally filtered by kind) and all budget
// bands. An empty kind returns every category.
func (s *Store) GetCategories(ctx context.Context, kind string) ([]model.Category, []model.BudgetBand, error) {
	const catQ = `
		SELECT id::text, name, kind, COALESCE(parent_id::text, '')
		FROM catalog.categories
		WHERE ($1 = '' OR kind = $1)
		ORDER BY name`
	catRows, err := s.pool.Query(ctx, catQ, kind)
	if err != nil {
		return nil, nil, fmt.Errorf("query categories: %w", err)
	}
	defer catRows.Close()

	var cats []model.Category
	for catRows.Next() {
		var c model.Category
		if err := catRows.Scan(&c.ID, &c.Name, &c.Kind, &c.ParentID); err != nil {
			return nil, nil, fmt.Errorf("scan category: %w", err)
		}
		cats = append(cats, c)
	}
	if err := catRows.Err(); err != nil {
		return nil, nil, err
	}
	catRows.Close()

	const bandQ = `
		SELECT id::text, label, min_cents, max_cents, currency
		FROM catalog.budget_bands
		ORDER BY min_cents`
	bandRows, err := s.pool.Query(ctx, bandQ)
	if err != nil {
		return nil, nil, fmt.Errorf("query budget bands: %w", err)
	}
	defer bandRows.Close()

	var bands []model.BudgetBand
	for bandRows.Next() {
		var b model.BudgetBand
		if err := bandRows.Scan(&b.ID, &b.Label, &b.MinCents, &b.MaxCents, &b.Currency); err != nil {
			return nil, nil, fmt.Errorf("scan budget band: %w", err)
		}
		bands = append(bands, b)
	}
	return cats, bands, bandRows.Err()
}

// SearchInspiration returns up to topK active corpus rows nearest to embedding by
// cosine distance, optionally filtered by category / budget band. Score is cosine
// similarity in [0,1] (1 - cosine distance).
func (s *Store) SearchInspiration(ctx context.Context, embedding []float32, f model.SearchFilter, topK int) ([]model.Snippet, error) {
	vec := VectorLiteral(embedding)
	const q = `
		SELECT id::text, title, body,
		       COALESCE(category_id::text, ''),
		       COALESCE(budget_band_id::text, ''),
		       tags,
		       1 - (embedding <=> $1::vector) AS score
		FROM catalog.inspiration
		WHERE active = true
		  AND ($2 = '' OR category_id = $2::uuid)
		  AND ($3 = '' OR budget_band_id = $3::uuid)
		ORDER BY embedding <=> $1::vector
		LIMIT $4`
	rows, err := s.pool.Query(ctx, q, vec, f.CategoryID, f.BudgetBandID, topK)
	if err != nil {
		return nil, fmt.Errorf("query inspiration: %w", err)
	}
	defer rows.Close()

	var out []model.Snippet
	for rows.Next() {
		var sn model.Snippet
		var tags []byte
		if err := rows.Scan(&sn.ID, &sn.Title, &sn.Body, &sn.CategoryID, &sn.BudgetBandID, &tags, &sn.Score); err != nil {
			return nil, fmt.Errorf("scan snippet: %w", err)
		}
		if sn.Tags, err = decodeTags(tags); err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// UpsertInspiration inserts or updates a corpus row with its freshly-computed
// embedding and returns the row id. An empty in.ID inserts a new row (server
// generates the uuid); a non-empty id updates in place.
func (s *Store) UpsertInspiration(ctx context.Context, in model.Inspiration, embedding []float32) (string, error) {
	tags, err := json.Marshal(nonNilTags(in.Tags))
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}
	const q = `
		INSERT INTO catalog.inspiration
			(id, title, body, category_id, budget_band_id, tags, embedding, curated_by, curated_at, active)
		VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2, $3,
			NULLIF($4, '')::uuid,
			NULLIF($5, '')::uuid,
			$6::jsonb, $7::vector, $8, now(), $9
		)
		ON CONFLICT (id) DO UPDATE SET
			title          = EXCLUDED.title,
			body           = EXCLUDED.body,
			category_id    = EXCLUDED.category_id,
			budget_band_id = EXCLUDED.budget_band_id,
			tags           = EXCLUDED.tags,
			embedding      = EXCLUDED.embedding,
			curated_by     = EXCLUDED.curated_by,
			curated_at     = now(),
			active         = EXCLUDED.active
		RETURNING id::text`
	var id string
	err = s.pool.QueryRow(ctx, q,
		in.ID, in.Title, in.Body, in.CategoryID, in.BudgetBandID,
		string(tags), VectorLiteral(embedding), in.CuratedBy, in.Active,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert inspiration: %w", err)
	}
	return id, nil
}

// VectorLiteral renders a float slice as pgvector's text form: "[v1,v2,...]".
func VectorLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

func decodeTags(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil, fmt.Errorf("decode tags: %w", err)
	}
	return tags, nil
}

func nonNilTags(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}
