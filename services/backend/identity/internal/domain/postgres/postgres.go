// Package postgres owns Identity's durable state: users, password credentials,
// and oauth links, all in the `identity` schema. It implements the app's Users
// interface.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested user does not exist.
var ErrNotFound = errors.New("user not found")

// Repo is the Postgres-backed user store.
type Repo struct {
	pool *pgxpool.Pool
}

// New connects to Postgres using the given DSN.
func New(ctx context.Context, dsn string) (*Repo, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return &Repo{pool: pool}, nil
}

// NewWithPool wraps an existing pool (used by tests).
func NewWithPool(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Close() { r.pool.Close() }

// UpsertOAuthUser resolves (provider, subject) to a user, creating the user and
// oauth link on first login. When email matches an existing user, the link is
// attached to that user instead of creating a duplicate. Runs in one tx.
func (r *Repo) UpsertOAuthUser(ctx context.Context, provider, subject, email, displayName string) (model.User, error) {
	var out model.User
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// 1. Existing link?
		var userID string
		err := tx.QueryRow(ctx,
			`SELECT user_id FROM identity.oauth_links WHERE provider=$1 AND provider_subject=$2`,
			provider, subject,
		).Scan(&userID)
		if err == nil {
			u, err := getByID(ctx, tx, userID)
			if err != nil {
				return err
			}
			out = u
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		// 2. Existing user by email? (only if provider gave us one)
		if email != "" {
			var existingID string
			err := tx.QueryRow(ctx, `SELECT id FROM identity.users WHERE email=$1`, email).Scan(&existingID)
			switch {
			case err == nil:
				userID = existingID
			case errors.Is(err, pgx.ErrNoRows):
				// fall through to create
			default:
				return err
			}
		}

		// 3. Create the user if still unknown.
		if userID == "" {
			id := uuid.NewString()
			var emailArg any
			if email != "" {
				emailArg = email
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO identity.users (id, email, display_name) VALUES ($1, $2, $3)`,
				id, emailArg, displayName,
			); err != nil {
				return err
			}
			userID = id
		}

		// 4. Link the external identity.
		if _, err := tx.Exec(ctx,
			`INSERT INTO identity.oauth_links (user_id, provider, provider_subject) VALUES ($1, $2, $3)`,
			userID, provider, subject,
		); err != nil {
			return err
		}

		u, err := getByID(ctx, tx, userID)
		if err != nil {
			return err
		}
		out = u
		return nil
	})
	return out, err
}

// CreateEmailUser creates a user and its password credential in one tx.
func (r *Repo) CreateEmailUser(ctx context.Context, email, displayName, passwordHash string) (model.User, error) {
	var out model.User
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		id := uuid.NewString()
		if _, err := tx.Exec(ctx,
			`INSERT INTO identity.users (id, email, display_name) VALUES ($1, $2, $3)`,
			id, email, displayName,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO identity.credentials (user_id, type, password_hash) VALUES ($1, 'password', $2)`,
			id, passwordHash,
		); err != nil {
			return err
		}
		u, err := getByID(ctx, tx, id)
		if err != nil {
			return err
		}
		out = u
		return nil
	})
	return out, err
}

func (r *Repo) GetByID(ctx context.Context, id string) (model.User, error) {
	u, err := getByID(ctx, r.pool, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	return u, err
}

func (r *Repo) GetByEmail(ctx context.Context, email string) (model.User, bool, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, selectUser+` WHERE email=$1`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, err
	}
	return u, true, nil
}

func (r *Repo) GetPasswordHash(ctx context.Context, userID string) (string, bool, error) {
	var hash string
	err := r.pool.QueryRow(ctx,
		`SELECT password_hash FROM identity.credentials WHERE user_id=$1 AND type='password'`, userID,
	).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return hash, true, nil
}

// --- helpers ---

const selectUser = `SELECT id, COALESCE(email::text, ''), display_name, status, created_at FROM identity.users`

// querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func getByID(ctx context.Context, q querier, id string) (model.User, error) {
	return scanUser(q.QueryRow(ctx, selectUser+` WHERE id=$1`, id))
}

func scanUser(row pgx.Row) (model.User, error) {
	var u model.User
	if err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt); err != nil {
		return model.User{}, err
	}
	return u, nil
}
