package valkey

import (
	"context"
	"os"
	"testing"
	"time"
)

// Integration test against a real Valkey/Redis. Skips unless POLL_TEST_VALKEY_ADDR
// is set, keeping `go test ./...` hermetic.
//
//	docker run --rm -p 6379:6379 -d valkey/valkey:8
//	POLL_TEST_VALKEY_ADDR=127.0.0.1:6379 go test ./internal/domain/valkey/ -run Integration -v
func TestIntegration_AllowFixedWindow(t *testing.T) {
	addr := os.Getenv("POLL_TEST_VALKEY_ADDR")
	if addr == "" {
		t.Skip("set POLL_TEST_VALKEY_ADDR to run the Valkey integration test")
	}
	store, err := NewStore(addr)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	key := "poll:test:rl:" + time.Now().Format("150405.000000")
	budget := 3

	for i := 1; i <= budget; i++ {
		ok, err := store.Allow(ctx, key, budget, time.Minute)
		if err != nil {
			t.Fatalf("Allow #%d: %v", i, err)
		}
		if !ok {
			t.Fatalf("Allow #%d: want within budget", i)
		}
	}
	ok, err := store.Allow(ctx, key, budget, time.Minute)
	if err != nil {
		t.Fatalf("Allow over budget: %v", err)
	}
	if ok {
		t.Fatal("expected denial once budget is exhausted")
	}
}
