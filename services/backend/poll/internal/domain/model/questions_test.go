package model

import "testing"

func TestValidateQuestions_Valid(t *testing.T) {
	if err := ValidateQuestions(questions()); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateQuestions_Empty(t *testing.T) {
	if err := ValidateQuestions(nil); err == nil {
		t.Fatal("expected error for no questions")
	}
}

func TestValidateQuestions_BlankPrompt(t *testing.T) {
	qs := []Question{{ID: "q1", Prompt: "  ", Type: TypeText}}
	if err := ValidateQuestions(qs); err == nil {
		t.Fatal("expected error for blank prompt")
	}
}

func TestValidateQuestions_DuplicateIDs(t *testing.T) {
	qs := []Question{
		{ID: "dup", Prompt: "a", Type: TypeText},
		{ID: "dup", Prompt: "b", Type: TypeText},
	}
	if err := ValidateQuestions(qs); err == nil {
		t.Fatal("expected error for duplicate question ids")
	}
}

func TestValidateQuestions_ChoiceNeedsOptions(t *testing.T) {
	qs := []Question{{ID: "q1", Prompt: "pick", Type: TypeSingleChoice}}
	if err := ValidateQuestions(qs); err == nil {
		t.Fatal("expected error: choice question without options")
	}
}

func TestValidateQuestions_DuplicateOptionIDs(t *testing.T) {
	qs := []Question{{ID: "q1", Prompt: "pick", Type: TypeSingleChoice,
		Options: []Option{{ID: "o", Label: "A"}, {ID: "o", Label: "B"}}}}
	if err := ValidateQuestions(qs); err == nil {
		t.Fatal("expected error: duplicate option ids")
	}
}

func TestValidateQuestions_UnknownType(t *testing.T) {
	qs := []Question{{ID: "q1", Prompt: "x", Type: QuestionType("weird")}}
	if err := ValidateQuestions(qs); err == nil {
		t.Fatal("expected error for unknown question type")
	}
}
