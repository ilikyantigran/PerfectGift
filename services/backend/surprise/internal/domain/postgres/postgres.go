// Package postgres is the transactional idea ledger: the concrete
// domain.Repository backed by PostgreSQL + pgvector. It owns the `surprise`
// schema and its migrations (embedded). The whole test suite runs against the
// in-memory store instead, so this package needs no live DB to build.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store implements domain.Repository over a pgx pool.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to Postgres and returns the store. Call Close on shutdown.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Migrate applies the embedded schema migrations in lexical order.
func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) CreateRequest(ctx context.Context, r *domain.Request) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO surprise.surprise_requests
            (id, user_id, holiday_id, budget_band, preferences_text, poll_id, idempotency_key, status, model_tier, refinement, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, now())`,
		r.ID, r.UserID, r.HolidayID, r.BudgetBand, r.PreferencesText, nullIfEmpty(r.PollID),
		r.IdempotencyKey, string(r.Status), string(r.Tier), r.Refinement)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (s *Store) GetRequest(ctx context.Context, id string) (*domain.Request, error) {
	return s.scanRequest(ctx, `WHERE id = $1`, id)
}

func (s *Store) GetRequestByIdempotencyKey(ctx context.Context, key string) (*domain.Request, error) {
	return s.scanRequest(ctx, `WHERE idempotency_key = $1`, key)
}

func (s *Store) scanRequest(ctx context.Context, where string, arg any) (*domain.Request, error) {
	var r domain.Request
	var pollID *string
	var st, tier string
	err := s.pool.QueryRow(ctx, `
        SELECT id, user_id, holiday_id, budget_band, preferences_text, poll_id,
               idempotency_key, status, model_tier, refinement, created_at
        FROM surprise.surprise_requests `+where, arg).
		Scan(&r.ID, &r.UserID, &r.HolidayID, &r.BudgetBand, &r.PreferencesText, &pollID,
			&r.IdempotencyKey, &st, &tier, &r.Refinement, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if pollID != nil {
		r.PollID = *pollID
	}
	r.Status = domain.Status(st)
	r.Tier = domain.Tier(tier)
	return &r, nil
}

func (s *Store) SetRequestStatus(ctx context.Context, id string, st domain.Status) error {
	_, err := s.pool.Exec(ctx, `UPDATE surprise.surprise_requests SET status = $2 WHERE id = $1`, id, string(st))
	return err
}

func (s *Store) MarkRefinement(ctx context.Context, id, refinement string) error {
	_, err := s.pool.Exec(ctx, `
        UPDATE surprise.surprise_requests SET refinement = $2, status = 'queued' WHERE id = $1`, id, refinement)
	return err
}

func (s *Store) SaveIdeas(ctx context.Context, requestID string, ideas []domain.Idea) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM surprise.generated_ideas WHERE request_id = $1`, requestID); err != nil {
		return err
	}
	for _, i := range ideas {
		id := i.ID
		if id == "" {
			id = uuid.NewString()
		}
		if _, err := tx.Exec(ctx, `
            INSERT INTO surprise.generated_ideas
                (id, request_id, title, why_it_fits, rough_cost, how_to, rank, moderation_status, embedding, created_at)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::vector, now())`,
			id, requestID, i.Title, i.WhyItFits, i.RoughCost, i.HowTo, i.Rank,
			string(i.Moderation), vectorLiteral(i.Embedding)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetIdeas(ctx context.Context, requestID string) ([]domain.Idea, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT id, request_id, title, why_it_fits, rough_cost, how_to, rank, moderation_status, created_at
        FROM surprise.generated_ideas WHERE request_id = $1 ORDER BY rank ASC`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Idea
	for rows.Next() {
		var i domain.Idea
		var mod string
		if err := rows.Scan(&i.ID, &i.RequestID, &i.Title, &i.WhyItFits, &i.RoughCost, &i.HowTo, &i.Rank, &mod, &i.CreatedAt); err != nil {
			return nil, err
		}
		i.Moderation = domain.Moderation(mod)
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) GetIdea(ctx context.Context, ideaID string) (*domain.Idea, error) {
	var i domain.Idea
	var mod string
	err := s.pool.QueryRow(ctx, `
        SELECT id, request_id, title, why_it_fits, rough_cost, how_to, rank, moderation_status, created_at
        FROM surprise.generated_ideas WHERE id = $1`, ideaID).
		Scan(&i.ID, &i.RequestID, &i.Title, &i.WhyItFits, &i.RoughCost, &i.HowTo, &i.Rank, &mod, &i.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	i.Moderation = domain.Moderation(mod)
	return &i, nil
}

func (s *Store) SaveIdeaForUser(ctx context.Context, userID, ideaID string) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO surprise.saved_ideas (id, user_id, idea_id, saved_at)
        VALUES ($1,$2,$3, now())
        ON CONFLICT (user_id, idea_id) DO NOTHING`, uuid.NewString(), userID, ideaID)
	return err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// vectorLiteral renders a []float32 as pgvector's text input format, or NULL.
func vectorLiteral(v []float32) any {
	if len(v) == 0 {
		return nil
	}
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
