// Package postgres is the Poll service's owner of long-term state: the polls,
// poll_links (hashed), and poll_responses tables in the `poll` schema. It
// implements the ports.Repo port. No cross-schema access — the service reads other
// services' data only via their APIs.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/model"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/ports"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/migrations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

// NewStore opens a connection pool and verifies connectivity.
func NewStore(ctx context.Context, dsn string) (*Store, error) {
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

func (s *Store) Close() { s.pool.Close() }

// Migrate applies the embedded SQL migrations in lexical order. They are written
// idempotently (IF NOT EXISTS), so re-running is safe.
func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".sql" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		sqlBytes, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) CreatePoll(ctx context.Context, p model.Poll, tokenHash string, linkExpiresAt time.Time) error {
	questions, err := json.Marshal(p.Questions)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO poll.polls (id, owner_user_id, surprise_request_id, title, questions, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.ID, p.OwnerUserID, nullable(p.SurpriseRequestID), p.Title, questions, string(p.Status), p.ExpiresAt, p.CreatedAt,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO poll.poll_links (id, poll_id, token_hash, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, $4, false, $5)`,
		uuid.NewString(), p.ID, tokenHash, linkExpiresAt, p.CreatedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetByTokenHash(ctx context.Context, tokenHash string) (ports.LinkedPoll, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT p.id, p.owner_user_id, COALESCE(p.surprise_request_id, ''), p.title, p.questions,
		       p.status, p.expires_at, p.created_at, l.revoked, l.expires_at
		FROM poll.poll_links l
		JOIN poll.polls p ON p.id = l.poll_id
		WHERE l.token_hash = $1`, tokenHash)

	var (
		lp            ports.LinkedPoll
		questions     []byte
		statusStr     string
		linkRevoked   bool
		linkExpiresAt time.Time
	)
	p := &lp.Poll
	if err := row.Scan(&p.ID, &p.OwnerUserID, &p.SurpriseRequestID, &p.Title, &questions,
		&statusStr, &p.ExpiresAt, &p.CreatedAt, &linkRevoked, &linkExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.LinkedPoll{}, ports.ErrNotFound
		}
		return ports.LinkedPoll{}, err
	}
	if err := json.Unmarshal(questions, &p.Questions); err != nil {
		return ports.LinkedPoll{}, err
	}
	p.Status = model.Status(statusStr)
	lp.LinkRevoked = linkRevoked
	lp.LinkExpiresAt = linkExpiresAt
	return lp, nil
}

func (s *Store) CompleteWithResponse(ctx context.Context, pollID string, answers []model.Answer, fingerprint string, at time.Time) (string, error) {
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		return "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	// One-response guard: only an active poll can be completed. If no row is
	// updated, the poll was already completed/expired/absent.
	tag, err := tx.Exec(ctx, `
		UPDATE poll.polls SET status = 'completed'
		WHERE id = $1 AND status = 'active'`, pollID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", ports.ErrAlreadyCompleted
	}

	respID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO poll.poll_responses (id, poll_id, answers, submitted_at, client_fingerprint)
		VALUES ($1, $2, $3, $4, $5)`,
		respID, pollID, answersJSON, at, nullable(fingerprint),
	); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return respID, nil
}

func (s *Store) GetPollByID(ctx context.Context, pollID string) (model.Poll, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, COALESCE(surprise_request_id, ''), title, questions, status, expires_at, created_at
		FROM poll.polls WHERE id = $1`, pollID)

	var (
		p         model.Poll
		questions []byte
		statusStr string
	)
	if err := row.Scan(&p.ID, &p.OwnerUserID, &p.SurpriseRequestID, &p.Title, &questions, &statusStr, &p.ExpiresAt, &p.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Poll{}, ports.ErrNotFound
		}
		return model.Poll{}, err
	}
	if err := json.Unmarshal(questions, &p.Questions); err != nil {
		return model.Poll{}, err
	}
	p.Status = model.Status(statusStr)
	return p, nil
}

func (s *Store) GetResponses(ctx context.Context, pollID string) ([]model.Response, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, answers, submitted_at
		FROM poll.poll_responses WHERE poll_id = $1 ORDER BY submitted_at`, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Response
	for rows.Next() {
		var (
			r       model.Response
			answers []byte
		)
		if err := rows.Scan(&r.ID, &answers, &r.SubmittedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(answers, &r.Answers); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
