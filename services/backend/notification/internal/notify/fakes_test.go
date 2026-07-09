package notify

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// fakeStore is an in-memory Store used by the unit tests. It reproduces the two
// behaviors the guarantees depend on: EnqueueOutbox is idempotent on
// dedupe_key, and ClaimPending atomically leases rows (a claimed row's
// next_attempt_at is pushed to now+lease so no concurrent claim — and no re-run
// before the lease elapses — can grab it again).
type fakeStore struct {
	mu sync.Mutex

	devices map[string]*Device // keyed by push_token
	outbox  map[string]*Outbox // keyed by id
	byKey   map[string]string  // dedupe_key -> id
	seq     int

	// hooks for fault injection
	claimErr   error
	resolveErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		devices: make(map[string]*Device),
		outbox:  make(map[string]*Outbox),
		byKey:   make(map[string]string),
	}
}

func (s *fakeStore) UpsertDevice(_ context.Context, d Device, now time.Time) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.devices[d.PushToken]; ok {
		existing.UserID = d.UserID
		existing.Platform = d.Platform
		existing.AppVersion = d.AppVersion
		existing.LastSeenAt = now
		existing.Active = true
		return *existing, nil
	}
	s.seq++
	d.ID = fmt.Sprintf("dev-%d", s.seq)
	d.RegisteredAt = now
	d.LastSeenAt = now
	d.Active = true
	cp := d
	s.devices[d.PushToken] = &cp
	return cp, nil
}

func (s *fakeStore) DeactivateDeviceByToken(_ context.Context, pushToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.devices[pushToken]; ok {
		d.Active = false
	}
	return nil
}

func (s *fakeStore) ActiveDevicesForUser(_ context.Context, userID string) ([]Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	var out []Device
	for _, d := range s.devices {
		if d.UserID == userID && d.Active {
			out = append(out, *d)
		}
	}
	return out, nil
}

func (s *fakeStore) EnqueueOutbox(_ context.Context, o Outbox) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byKey[o.DedupeKey]; ok {
		return false, nil // idempotent: dedupe_key already present
	}
	s.seq++
	o.ID = fmt.Sprintf("out-%d", s.seq)
	if o.Status == "" {
		o.Status = StatusPending
	}
	cp := o
	s.outbox[o.ID] = &cp
	s.byKey[o.DedupeKey] = o.ID
	return true, nil
}

func (s *fakeStore) ClaimPending(_ context.Context, now time.Time, lease time.Duration, limit int) ([]Outbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	var out []Outbox
	for _, o := range s.outbox {
		if len(out) >= limit {
			break
		}
		if o.Status == StatusPending && !o.NextAttemptAt.After(now) {
			o.NextAttemptAt = now.Add(lease) // lease it
			out = append(out, *o)
		}
	}
	return out, nil
}

func (s *fakeStore) MarkSent(_ context.Context, id string, sentAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.outbox[id]
	if !ok {
		return errors.New("not found")
	}
	o.Status = StatusSent
	o.SentAt = &sentAt
	return nil
}

func (s *fakeStore) Reschedule(_ context.Context, id string, attempts int, nextAttemptAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.outbox[id]
	if !ok {
		return errors.New("not found")
	}
	o.Attempts = attempts
	o.NextAttemptAt = nextAttemptAt
	o.Status = StatusPending
	return nil
}

func (s *fakeStore) MarkFailed(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.outbox[id]
	if !ok {
		return errors.New("not found")
	}
	o.Status = StatusFailed
	return nil
}

// test helpers (assume caller isn't holding the lock)

func (s *fakeStore) get(id string) Outbox {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.outbox[id]
}

func (s *fakeStore) getByKey(key string) Outbox {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.outbox[s.byKey[key]]
}

func (s *fakeStore) outboxCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.outbox)
}

func (s *fakeStore) device(token string) Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.devices[token]
}

// fakePusher records pushes and can be programmed to fail.
type fakePusher struct {
	mu sync.Mutex

	calls   []pushCall
	failFor map[string]error // push_token -> error to return (nil map = all succeed)
}

type pushCall struct {
	token string
	msg   PushMessage
}

func newFakePusher() *fakePusher {
	return &fakePusher{failFor: make(map[string]error)}
}

func (p *fakePusher) Push(_ context.Context, device Device, msg PushMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, pushCall{token: device.PushToken, msg: msg})
	if err, ok := p.failFor[device.PushToken]; ok {
		return err
	}
	return nil
}

func (p *fakePusher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func (p *fakePusher) countFor(token string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, c := range p.calls {
		if c.token == token {
			n++
		}
	}
	return n
}

// fakeMessage is a bus Message for consumer tests.
type fakeMessage struct {
	data    []byte
	acked   bool
	nakked  bool
}

func (m *fakeMessage) Data() []byte { return m.data }
func (m *fakeMessage) Ack() error   { m.acked = true; return nil }
func (m *fakeMessage) Nak() error   { m.nakked = true; return nil }
