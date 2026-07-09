// Package model holds the service's plain domain types, shared by the store
// (which reads/writes them) and the gRPC server (which maps them to/from proto).
// Keeping them in a neutral package avoids the app package depending on a concrete
// store implementation just for its types.
package model

// DateRule values (stored as text in Postgres).
const (
	DateRuleFixed    = "fixed"
	DateRuleRelative = "relative"
)

// CategoryKind values (stored as text in Postgres).
const (
	KindGift = "gift"
	KindDate = "date"
)

// Holiday is a reference-data holiday row.
type Holiday struct {
	ID       string
	Name     string
	DateRule string // "fixed" | "relative"
	Region   string
	Tags     []string
	Active   bool
}

// Category is a gift/date taxonomy node.
type Category struct {
	ID       string
	Name     string
	Kind     string // "gift" | "date"
	ParentID string // "" when top-level
}

// BudgetBand is a spend bracket.
type BudgetBand struct {
	ID       string
	Label    string
	MinCents int32
	MaxCents int32
	Currency string
}

// Inspiration is a curated corpus row (write side). The embedding is passed
// separately to the store, computed by the Embedder — it is never part of the API.
type Inspiration struct {
	ID           string // "" on create
	Title        string
	Body         string
	CategoryID   string // "" = none
	BudgetBandID string // "" = none
	Tags         []string
	CuratedBy    string
	Active       bool
}

// Snippet is a grounding search result: a corpus row plus its similarity score.
type Snippet struct {
	ID           string
	Title        string
	Body         string
	CategoryID   string
	BudgetBandID string
	Tags         []string
	Score        float64 // cosine similarity in [0,1]; higher = closer
}

// HolidayFilter narrows ListHolidays. A nil Active means "either".
type HolidayFilter struct {
	Region    string
	Active    *bool
	OnOrAfter string // ISO date; reserved for future date-aware filtering
}

// SearchFilter narrows SearchInspiration. Empty strings mean "any".
type SearchFilter struct {
	CategoryID   string
	BudgetBandID string
}
