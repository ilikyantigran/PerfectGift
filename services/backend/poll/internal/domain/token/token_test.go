package token

import (
	"strings"
	"testing"
)

func TestNew_RawIsNotTheHash(t *testing.T) {
	raw, hash, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatal("empty raw or hash")
	}
	if raw == hash {
		t.Fatal("raw token must not equal its stored hash")
	}
	if len(raw) < 32 {
		t.Fatalf("raw token too short: %d", len(raw))
	}
}

func TestNew_HashMatchesRaw(t *testing.T) {
	raw, hash, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := Hash(raw); got != hash {
		t.Fatalf("Hash(raw)=%s want %s", got, hash)
	}
}

func TestNew_RawsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		raw, _, err := New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if seen[raw] {
			t.Fatalf("duplicate raw token generated: %s", raw)
		}
		seen[raw] = true
	}
}

func TestHash_Deterministic(t *testing.T) {
	a := Hash("hello")
	b := Hash("hello")
	if a != b {
		t.Fatalf("Hash not deterministic: %s vs %s", a, b)
	}
	if a == Hash("world") {
		t.Fatal("different inputs hashed to same value")
	}
	// hex sha256 is 64 chars
	if len(a) != 64 || strings.ContainsAny(a, "+/=") {
		t.Fatalf("unexpected hash encoding: %q", a)
	}
}
