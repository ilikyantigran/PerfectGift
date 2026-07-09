package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func fixedNow() func() time.Time {
	t := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func TestHandlePollCompleted_EnqueuesOutbox(t *testing.T) {
	store := newFakeStore()
	c := NewConsumer(store, fixedNow())

	ev := PollCompletedEvent{PollID: "poll-1", SurpriseRequestID: "req-9", OwnerUserID: "user-1"}
	data, _ := json.Marshal(ev)

	if err := c.HandlePollCompleted(context.Background(), data); err != nil {
		t.Fatalf("handle: %v", err)
	}

	o := store.getByKey("poll_completed:poll-1:user-1")
	if o.Type != TypePollCompleted {
		t.Errorf("type = %q, want poll_completed", o.Type)
	}
	if o.UserID != "user-1" {
		t.Errorf("user = %q, want user-1", o.UserID)
	}
	if o.Status != StatusPending {
		t.Errorf("status = %q, want pending", o.Status)
	}
	// payload carries the poll id
	var p map[string]any
	_ = json.Unmarshal(o.Payload, &p)
	if p["poll_id"] != "poll-1" {
		t.Errorf("payload poll_id = %v, want poll-1", p["poll_id"])
	}
}

func TestHandleIdeasReady_EnqueuesOutbox(t *testing.T) {
	store := newFakeStore()
	c := NewConsumer(store, fixedNow())

	ev := IdeasReadyEvent{RequestID: "req-1", UserID: "user-2", IdeaCount: 5}
	data, _ := json.Marshal(ev)

	if err := c.HandleIdeasReady(context.Background(), data); err != nil {
		t.Fatalf("handle: %v", err)
	}

	o := store.getByKey("ideas_ready:req-1:user-2")
	if o.Type != TypeIdeasReady || o.UserID != "user-2" {
		t.Errorf("got type=%q user=%q, want ideas_ready/user-2", o.Type, o.UserID)
	}
	var p map[string]any
	_ = json.Unmarshal(o.Payload, &p)
	if p["idea_count"].(float64) != 5 {
		t.Errorf("payload idea_count = %v, want 5", p["idea_count"])
	}
}

// The core "never double-sent at the source" guarantee: the same event
// redelivered by the bus must create exactly one outbox row.
func TestHandle_DuplicateEvent_Idempotent(t *testing.T) {
	store := newFakeStore()
	c := NewConsumer(store, fixedNow())
	ev := IdeasReadyEvent{RequestID: "req-1", UserID: "user-2", IdeaCount: 3}
	data, _ := json.Marshal(ev)

	for i := 0; i < 3; i++ {
		if err := c.HandleIdeasReady(context.Background(), data); err != nil {
			t.Fatalf("handle %d: %v", i, err)
		}
	}
	if n := store.outboxCount(); n != 1 {
		t.Fatalf("outbox rows = %d, want 1 (dedupe)", n)
	}
}

func TestHandle_BadJSON_ReturnsError(t *testing.T) {
	c := NewConsumer(newFakeStore(), fixedNow())
	if err := c.HandlePollCompleted(context.Background(), []byte("{not json")); err == nil {
		t.Fatal("want decode error, got nil")
	}
}

func TestHandle_MissingRequiredFields_ReturnsError(t *testing.T) {
	c := NewConsumer(newFakeStore(), fixedNow())
	// no owner_user_id
	data, _ := json.Marshal(PollCompletedEvent{PollID: "p1"})
	if err := c.HandlePollCompleted(context.Background(), data); err == nil {
		t.Fatal("want validation error for missing owner_user_id")
	}
}

func TestProcess_AcksOnSuccess_NaksOnFailure(t *testing.T) {
	store := newFakeStore()
	c := NewConsumer(store, fixedNow())

	good, _ := json.Marshal(IdeasReadyEvent{RequestID: "r", UserID: "u", IdeaCount: 1})
	okMsg := &fakeMessage{data: good}
	Process(context.Background(), okMsg, c.HandleIdeasReady)
	if !okMsg.acked || okMsg.nakked {
		t.Errorf("good message: acked=%v nakked=%v, want acked", okMsg.acked, okMsg.nakked)
	}

	badMsg := &fakeMessage{data: []byte("garbage")}
	Process(context.Background(), badMsg, c.HandleIdeasReady)
	if badMsg.acked || !badMsg.nakked {
		t.Errorf("bad message: acked=%v nakked=%v, want nakked", badMsg.acked, badMsg.nakked)
	}
}
