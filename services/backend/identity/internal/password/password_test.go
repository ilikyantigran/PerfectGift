package password

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Errorf("hash is not a PHC argon2id string: %q", h)
	}
	ok, err := Verify("correct horse battery staple", h)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("correct password did not verify")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	h, _ := Hash("s3cret")
	ok, err := Verify("not-the-password", h)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("wrong password verified as correct")
	}
}

func TestHashIsSalted(t *testing.T) {
	h1, _ := Hash("same")
	h2, _ := Hash("same")
	if h1 == h2 {
		t.Error("two hashes of the same password are identical — salt missing")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, bad := range []string{
		"",
		"plaintext",
		"$argon2id$v=19$m=65536,t=1", // truncated
		"$bcrypt$whatever",
	} {
		if _, err := Verify("x", bad); err == nil {
			t.Errorf("expected error for malformed hash %q", bad)
		}
	}
}
