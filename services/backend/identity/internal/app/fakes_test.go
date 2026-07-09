package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// fakeUsers is an in-memory implementation of the Users store for unit tests.
type fakeUsers struct {
	mu     sync.Mutex
	byID   map[string]User
	byMail map[string]string // lower(email) -> id
	oauth  map[string]string // provider|subject -> id
	hashes map[string]string // id -> password hash
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{
		byID:   map[string]User{},
		byMail: map[string]string{},
		oauth:  map[string]string{},
		hashes: map[string]string{},
	}
}

func (f *fakeUsers) UpsertOAuthUser(_ context.Context, provider, subject, email, displayName string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := provider + "|" + subject
	if id, ok := f.oauth[key]; ok {
		return f.byID[id], nil
	}
	// Link to an existing user by email, else create a fresh one.
	var u User
	if email != "" {
		if id, ok := f.byMail[strings.ToLower(email)]; ok {
			u = f.byID[id]
		}
	}
	if u.ID == "" {
		u = User{ID: uuid.NewString(), Email: email, DisplayName: displayName, Status: "active", CreatedAt: time.Now()}
		f.byID[u.ID] = u
		if email != "" {
			f.byMail[strings.ToLower(email)] = u.ID
		}
	}
	f.oauth[key] = u.ID
	return u, nil
}

func (f *fakeUsers) GetByID(_ context.Context, id string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (User, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byMail[strings.ToLower(email)]
	if !ok {
		return User{}, false, nil
	}
	return f.byID[id], true, nil
}

func (f *fakeUsers) CreateEmailUser(_ context.Context, email, displayName, passwordHash string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byMail[strings.ToLower(email)]; ok {
		return User{}, errors.New("email exists")
	}
	u := User{ID: uuid.NewString(), Email: email, DisplayName: displayName, Status: "active", CreatedAt: time.Now()}
	f.byID[u.ID] = u
	f.byMail[strings.ToLower(email)] = u.ID
	f.hashes[u.ID] = passwordHash
	return u, nil
}

func (f *fakeUsers) GetPasswordHash(_ context.Context, userID string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	h, ok := f.hashes[userID]
	return h, ok, nil
}

// fakeSessions is an in-memory Sessions store.
type fakeSessions struct {
	mu sync.Mutex
	m  map[string]Session
}

func newFakeSessions() *fakeSessions { return &fakeSessions{m: map[string]Session{}} }

func (f *fakeSessions) Create(_ context.Context, s Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[s.ID] = s
	return nil
}

func (f *fakeSessions) Get(_ context.Context, id string) (Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.m[id]
	return s, ok, nil
}

func (f *fakeSessions) Update(_ context.Context, s Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.m[s.ID]; !ok {
		return errors.New("no such session")
	}
	f.m[s.ID] = s
	return nil
}

func (f *fakeSessions) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, id)
	return nil
}

// fakeLimiter blocks a key after max Allow calls.
type fakeLimiter struct {
	mu    sync.Mutex
	max   int
	count map[string]int
}

func newFakeLimiter(max int) *fakeLimiter { return &fakeLimiter{max: max, count: map[string]int{}} }

func (f *fakeLimiter) Allow(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count[key]++
	return f.count[key] <= f.max, nil
}
