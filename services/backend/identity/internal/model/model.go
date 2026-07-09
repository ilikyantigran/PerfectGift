// Package model holds the plain value types that cross the boundary between the
// server logic (internal/app) and the state owners (internal/domain/*). Keeping
// them here lets a domain store satisfy an interface defined in internal/app
// without an import cycle.
package model

import "time"

// User is the basic profile Identity owns (its Postgres `users` row).
type User struct {
	ID          string
	Email       string
	DisplayName string
	Status      string
	CreatedAt   time.Time
}

// Session is a login session stored in Valkey. Only the hash of the refresh
// token's secret is kept, so a leaked store cannot mint refresh tokens.
type Session struct {
	ID          string
	UserID      string
	RefreshHash string
	Device      string
	IssuedAt    time.Time
	ExpiresAt   time.Time
}
