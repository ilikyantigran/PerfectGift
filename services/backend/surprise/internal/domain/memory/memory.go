// Package memory provides an in-memory implementation of domain.Repository and
// domain.Cache. It is the store the test suite runs against (so tests need no
// live Postgres/Valkey) and doubles as a safe default for local experiments.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain"
)

// Store implements both domain.Repository and domain.Cache backed by maps.
type Store struct {
	mu        sync.Mutex
	requests  map[string]*domain.Request
	idemByKey map[string]string // idempotency_key -> request_id
	ideas     map[string][]domain.Idea
	ideaByID  map[string]domain.Idea
	saved     map[string]map[string]bool // user_id -> set(idea_id)
	status    map[string]domain.StatusInfo
	idemCache map[string]string // Cache idempotency key -> request_id
	llmCache  map[string][]domain.Idea
}

// New returns an empty in-memory store.
func New() *Store {
	return &Store{
		requests:  map[string]*domain.Request{},
		idemByKey: map[string]string{},
		ideas:     map[string][]domain.Idea{},
		ideaByID:  map[string]domain.Idea{},
		saved:     map[string]map[string]bool{},
		status:    map[string]domain.StatusInfo{},
		idemCache: map[string]string{},
		llmCache:  map[string][]domain.Idea{},
	}
}

// --- Repository ---

func (s *Store) CreateRequest(_ context.Context, r *domain.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.idemByKey[r.IdempotencyKey]; ok {
		return domain.ErrDuplicate
	}
	cp := *r
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	s.requests[r.ID] = &cp
	s.idemByKey[r.IdempotencyKey] = r.ID
	return nil
}

func (s *Store) GetRequest(_ context.Context, id string) (*domain.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (s *Store) GetRequestByIdempotencyKey(_ context.Context, key string) (*domain.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.idemByKey[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *s.requests[id]
	return &cp, nil
}

func (s *Store) SetRequestStatus(_ context.Context, id string, st domain.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return domain.ErrNotFound
	}
	r.Status = st
	return nil
}

func (s *Store) MarkRefinement(_ context.Context, id, refinement string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return domain.ErrNotFound
	}
	r.Refinement = refinement
	r.Status = domain.StatusQueued
	return nil
}

func (s *Store) SaveIdeas(_ context.Context, requestID string, ideas []domain.Idea) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]domain.Idea, len(ideas))
	copy(cp, ideas)
	s.ideas[requestID] = cp
	for _, id := range cp {
		s.ideaByID[id.ID] = id
	}
	return nil
}

func (s *Store) GetIdeas(_ context.Context, requestID string) ([]domain.Idea, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Idea, len(s.ideas[requestID]))
	copy(out, s.ideas[requestID])
	sort.SliceStable(out, func(i, j int) bool { return out[i].Rank < out[j].Rank })
	return out, nil
}

func (s *Store) GetIdea(_ context.Context, ideaID string) (*domain.Idea, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.ideaByID[ideaID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &i, nil
}

func (s *Store) SaveIdeaForUser(_ context.Context, userID, ideaID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saved[userID] == nil {
		s.saved[userID] = map[string]bool{}
	}
	s.saved[userID][ideaID] = true
	return nil
}

// SavedFor is a test helper: returns whether an idea is saved for a user.
func (s *Store) SavedFor(userID, ideaID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saved[userID][ideaID]
}

// --- Cache ---

func (s *Store) SetStatus(_ context.Context, requestID string, info domain.StatusInfo, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[requestID] = info
	return nil
}

func (s *Store) GetStatus(_ context.Context, requestID string) (domain.StatusInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, ok := s.status[requestID]
	if !ok {
		return domain.StatusInfo{}, domain.ErrNotFound
	}
	return info, nil
}

func (s *Store) SetIdempotencyIfAbsent(_ context.Context, key, requestID string, _ time.Duration) (bool, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.idemCache[key]; ok {
		return false, existing, nil
	}
	s.idemCache[key] = requestID
	return true, requestID, nil
}

func (s *Store) GetLLMCache(_ context.Context, hash string) ([]domain.Idea, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.llmCache[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := make([]domain.Idea, len(v))
	copy(out, v)
	return out, nil
}

func (s *Store) SetLLMCache(_ context.Context, hash string, ideas []domain.Idea, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]domain.Idea, len(ideas))
	copy(cp, ideas)
	s.llmCache[hash] = cp
	return nil
}
