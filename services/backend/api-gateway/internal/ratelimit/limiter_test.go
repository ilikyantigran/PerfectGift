package ratelimit

import (
	"testing"
	"time"
)

func TestWindow_AllowsUnderBudget(t *testing.T) {
	l := NewWindow(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("request %d should be allowed under budget", i)
		}
	}
}

func TestWindow_BlocksOverBudget(t *testing.T) {
	l := NewWindow(2, time.Minute)
	l.Allow("k")
	l.Allow("k")
	if l.Allow("k") {
		t.Fatal("third request should be blocked over budget")
	}
}

func TestWindow_KeysAreIsolated(t *testing.T) {
	l := NewWindow(1, time.Minute)
	if !l.Allow("ip:1.1.1.1") {
		t.Fatal("first key should be allowed")
	}
	if !l.Allow("user:abc") {
		t.Fatal("a different key must have its own budget")
	}
	if l.Allow("ip:1.1.1.1") {
		t.Fatal("first key is now over budget")
	}
}

func TestWindow_ResetsAfterWindow(t *testing.T) {
	l := NewWindow(1, 20*time.Millisecond)
	if !l.Allow("k") {
		t.Fatal("first allowed")
	}
	if l.Allow("k") {
		t.Fatal("second blocked within window")
	}
	time.Sleep(30 * time.Millisecond)
	if !l.Allow("k") {
		t.Fatal("should reset after the window elapses")
	}
}

func TestWindow_ZeroBudgetDisabled(t *testing.T) {
	// A budget of 0 means "no limit configured" → always allow.
	l := NewWindow(0, time.Minute)
	for i := 0; i < 100; i++ {
		if !l.Allow("k") {
			t.Fatal("zero budget must disable limiting")
		}
	}
}

func TestNoop_AlwaysAllows(t *testing.T) {
	var l Limiter = Noop{}
	for i := 0; i < 10; i++ {
		if !l.Allow("k") {
			t.Fatal("noop always allows")
		}
	}
}
