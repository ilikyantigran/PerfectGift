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

func TestEmitIdeasInputToleratesObjectShapes(t *testing.T) {
	// (a) Wrapper object: "ideas" arrives as an object that itself carries the
	// ideas array — {"ideas":{"ideas":[...]}} at the tool-input level.
	wrapperJSON := []byte(`{"ideas":{"ideas":[{"title":"Picnic","why_it_fits":"outdoorsy","rough_cost":"$50","how_to":"pack a basket"}]}}`)
	var fromWrapper emitIdeasInput
	if err := json.Unmarshal(wrapperJSON, &fromWrapper); err != nil {
		t.Fatalf("decode wrapper-object shape: %v", err)
	}
	if len(fromWrapper.Ideas) != 1 || fromWrapper.Ideas[0].Title != "Picnic" {
		t.Fatalf("unexpected decoded ideas from wrapper object: %#v", fromWrapper.Ideas)
	}

	// (b) Single idea object: "ideas" arrives as one idea object rather than an
	// array — {"ideas":{"title":...,...}}.
	singleJSON := []byte(`{"ideas":{"title":"Stargazing","why_it_fits":"loves the night sky","rough_cost":"$0","how_to":"drive out of town"}}`)
	var fromSingle emitIdeasInput
	if err := json.Unmarshal(singleJSON, &fromSingle); err != nil {
		t.Fatalf("decode single-idea-object shape: %v", err)
	}
	if len(fromSingle.Ideas) != 1 || fromSingle.Ideas[0].Title != "Stargazing" || fromSingle.Ideas[0].WhyItFits == "" {
		t.Fatalf("unexpected decoded ideas from single object: %#v", fromSingle.Ideas)
	}
}

func TestEmitIdeasInputRejectsUnrecognizedIdeasShape(t *testing.T) {
	badJSON := []byte(`{"ideas": 42}`)
	var in emitIdeasInput
	if err := json.Unmarshal(badJSON, &in); err == nil {
		t.Fatal("expected decode error for an ideas field that is neither an array nor a stringified array")
	}
}

// A partial tool input where "ideas" is absent or explicitly null must decode to
// zero ideas WITHOUT an error — matching the pre-tolerance struct-tag behavior and
// avoiding a misleading "unexpected end of JSON input" from unmarshaling nil.
func TestEmitIdeasInputAbsentOrNullIsEmptyNoError(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"missing key", `{}`},
		{"explicit null", `{"ideas":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var in emitIdeasInput
			if err := json.Unmarshal([]byte(tc.in), &in); err != nil {
				t.Fatalf("want no error, got %v", err)
			}
			if len(in.Ideas) != 0 {
				t.Fatalf("want zero ideas, got %#v", in.Ideas)
			}
		})
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
