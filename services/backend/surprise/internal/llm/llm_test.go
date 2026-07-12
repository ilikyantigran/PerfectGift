package llm

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/resilience"
)

func TestFakeGenerateIdeasCount(t *testing.T) {
	f := &FakeClient{}
	ideas, err := f.GenerateIdeas(context.Background(), GenerateParams{Holiday: "Valentine", BudgetBand: "mid", N: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(ideas) != 4 {
		t.Fatalf("expected 4 ideas, got %d", len(ideas))
	}
	if ideas[0].Title == "" || ideas[0].WhyItFits == "" {
		t.Fatal("ideas should be populated")
	}
}

func TestFakeModerateBlocksUnsafe(t *testing.T) {
	f := &FakeClient{}
	if ok, _ := f.Moderate(context.Background(), "a lovely picnic"); !ok {
		t.Fatal("expected wholesome text approved")
	}
	if ok, _ := f.Moderate(context.Background(), "something UNSAFE here"); ok {
		t.Fatal("expected unsafe text rejected")
	}
}

func TestResilientRetriesThenSucceeds(t *testing.T) {
	calls := 0
	inner := &FakeClient{GenerateFunc: func(ctx context.Context, p GenerateParams) ([]Idea, error) {
		calls++
		if calls < 2 {
			return nil, errors.New("transient upstream")
		}
		return []Idea{{Title: "ok"}}, nil
	}}
	r := NewResilient(inner, resilience.NewBreaker(5, time.Minute), resilience.RetryConfig{MaxAttempts: 3, BaseBackoff: time.Millisecond})
	ideas, err := r.GenerateIdeas(context.Background(), GenerateParams{N: 1})
	if err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if len(ideas) != 1 || calls != 2 {
		t.Fatalf("expected 1 idea after 2 calls, got %d ideas / %d calls", len(ideas), calls)
	}
}

func TestEmitIdeasInputToleratesStringifiedIdeas(t *testing.T) {
	// The tool schema declares "ideas" as a JSON array (the expected/happy path).
	arrayJSON := []byte(`{"ideas":[{"title":"Picnic","why_it_fits":"outdoorsy","rough_cost":"$50","how_to":"pack a basket"}]}`)
	var fromArray emitIdeasInput
	if err := json.Unmarshal(arrayJSON, &fromArray); err != nil {
		t.Fatalf("decode array shape: %v", err)
	}

	// Production has seen Claude return "ideas" as a JSON-encoded string
	// containing that same array instead of a nested array.
	stringJSON := []byte(`{"ideas":"[{\"title\":\"Picnic\",\"why_it_fits\":\"outdoorsy\",\"rough_cost\":\"$50\",\"how_to\":\"pack a basket\"}]"}`)
	var fromString emitIdeasInput
	if err := json.Unmarshal(stringJSON, &fromString); err != nil {
		t.Fatalf("decode stringified shape: %v", err)
	}

	if !reflect.DeepEqual(fromArray.Ideas, fromString.Ideas) {
		t.Fatalf("expected both shapes to decode identically, got %#v vs %#v", fromArray.Ideas, fromString.Ideas)
	}
	if len(fromArray.Ideas) != 1 || fromArray.Ideas[0].Title != "Picnic" {
		t.Fatalf("unexpected decoded ideas: %#v", fromArray.Ideas)
	}
}

func TestEmitIdeasInputRejectsUnrecognizedIdeasShape(t *testing.T) {
	badJSON := []byte(`{"ideas": 42}`)
	var in emitIdeasInput
	if err := json.Unmarshal(badJSON, &in); err == nil {
		t.Fatal("expected decode error for an ideas field that is neither an array nor a stringified array")
	}
}

func TestResilientOpensBreaker(t *testing.T) {
	inner := &FakeClient{GenerateFunc: func(context.Context, GenerateParams) ([]Idea, error) {
		return nil, errors.New("down")
	}}
	breaker := resilience.NewBreaker(2, time.Minute)
	// No retries so each call is one breaker attempt.
	r := NewResilient(inner, breaker, resilience.RetryConfig{MaxAttempts: 1, BaseBackoff: time.Millisecond})
	_, _ = r.GenerateIdeas(context.Background(), GenerateParams{})
	_, _ = r.GenerateIdeas(context.Background(), GenerateParams{})
	if breaker.State() != resilience.StateOpen {
		t.Fatalf("expected breaker open after repeated failures, got %v", breaker.State())
	}
	// Next call should fast-fail with ErrOpen.
	_, err := r.GenerateIdeas(context.Background(), GenerateParams{})
	if !errors.Is(err, resilience.ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
}
