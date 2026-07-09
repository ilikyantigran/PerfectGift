package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests require a real Postgres. They are skipped unless
// IDENTITY_TEST_DATABASE_DSN points at a disposable database, so the default
// `go test ./...` run stays hermetic (no live DB, no network).
//
// Example:
//
//	docker run --rm -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:16
//	IDENTITY_TEST_DATABASE_DSN='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable' \
//	    go test ./internal/domain/postgres/...
func testRepo(t *testing.T) *Repo {
	t.Helper()
	dsn := os.Getenv("IDENTITY_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set IDENTITY_TEST_DATABASE_DSN to run Postgres integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Clean slate for a deterministic test.
	if _, err := pool.Exec(ctx, `TRUNCATE identity.users, identity.credentials, identity.oauth_links CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return NewWithPool(pool)
}

func TestUpsertOAuthUserCreatesThenReuses(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()

	u1, err := r.UpsertOAuthUser(ctx, "google", "sub-1", "a@example.com", "Alice")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if u1.ID == "" || u1.Email != "a@example.com" {
		t.Fatalf("unexpected user: %+v", u1)
	}

	u2, err := r.UpsertOAuthUser(ctx, "google", "sub-1", "a@example.com", "Alice")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if u2.ID != u1.ID {
		t.Errorf("same subject should map to same user: %s != %s", u1.ID, u2.ID)
	}
}

func TestEmailUserCredentialRoundTrip(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()

	u, err := r.CreateEmailUser(ctx, "b@example.com", "Bob", "hash-xyz")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, ok, err := r.GetByEmail(ctx, "B@EXAMPLE.COM") // citext = case-insensitive
	if err != nil || !ok {
		t.Fatalf("GetByEmail: ok=%v err=%v", ok, err)
	}
	if got.ID != u.ID {
		t.Errorf("id mismatch")
	}

	hash, ok, err := r.GetPasswordHash(ctx, u.ID)
	if err != nil || !ok || hash != "hash-xyz" {
		t.Fatalf("GetPasswordHash: %q ok=%v err=%v", hash, ok, err)
	}
}
