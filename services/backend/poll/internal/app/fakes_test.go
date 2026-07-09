package app

import (
	"context"
	"sync"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/model"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/ports"
)

// fakeRepo is an in-memory Repo so the server suite needs no live Postgres.
type fakeRepo struct {
	mu         sync.Mutex
	polls      map[string]model.Poll       // pollID -> poll
	links      map[string]link             // tokenHash -> link
	responses  map[string][]model.Response // pollID -> responses
	failCreate bool
}

type link struct {
	pollID    string
	revoked   bool
	expiresAt time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		polls:     map[string]model.Poll{},
		links:     map[string]link{},
		responses: map[string][]model.Response{},
	}
}

func (r *fakeRepo) CreatePoll(_ context.Context, p model.Poll, tokenHash string, linkExpiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failCreate {
		return context.DeadlineExceeded
	}
	r.polls[p.ID] = p
	r.links[tokenHash] = link{pollID: p.ID, expiresAt: linkExpiresAt}
	return nil
}

func (r *fakeRepo) GetByTokenHash(_ context.Context, tokenHash string) (ports.LinkedPoll, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.links[tokenHash]
	if !ok {
		return ports.LinkedPoll{}, ports.ErrNotFound
	}
	p, ok := r.polls[l.pollID]
	if !ok {
		return ports.LinkedPoll{}, ports.ErrNotFound
	}
	return ports.LinkedPoll{Poll: p, LinkRevoked: l.revoked, LinkExpiresAt: l.expiresAt}, nil
}

func (r *fakeRepo) CompleteWithResponse(_ context.Context, pollID string, answers []model.Answer, fp string, at time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.polls[pollID]
	if !ok {
		return "", ports.ErrNotFound
	}
	if p.Status != model.StatusActive {
		return "", ports.ErrAlreadyCompleted // one-response guard
	}
	p.Status = model.StatusCompleted
	r.polls[pollID] = p
	id := "resp-" + pollID
	r.responses[pollID] = append(r.responses[pollID], model.Response{
		ID: id, Answers: answers, SubmittedAt: at,
	})
	return id, nil
}

func (r *fakeRepo) GetPollByID(_ context.Context, pollID string) (model.Poll, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.polls[pollID]
	if !ok {
		return model.Poll{}, ports.ErrNotFound
	}
	return p, nil
}

func (r *fakeRepo) GetResponses(_ context.Context, pollID string) ([]model.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.responses[pollID], nil
}

// fakeLimiter is a fixed-window in-memory rate limiter. If deny is set for a key
// prefix, that key is always denied; otherwise budget applies.
type fakeLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	fail   bool
}

func newFakeLimiter() *fakeLimiter { return &fakeLimiter{counts: map[string]int{}} }

func (l *fakeLimiter) Allow(_ context.Context, key string, budget int, _ time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fail {
		return false, context.DeadlineExceeded
	}
	l.counts[key]++
	return l.counts[key] <= budget, nil
}

// fakePublisher records published events.
type fakePublisher struct {
	mu     sync.Mutex
	events []ports.PollCompleted
	fail   bool
}

func newFakePublisher() *fakePublisher { return &fakePublisher{} }

func (p *fakePublisher) PublishPollCompleted(_ context.Context, ev ports.PollCompleted) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail {
		return context.DeadlineExceeded
	}
	p.events = append(p.events, ev)
	return nil
}

func (p *fakePublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.events)
}
