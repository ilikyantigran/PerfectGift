package model

import "testing"

func questions() []Question {
	return []Question{
		{ID: "q_txt", Prompt: "Favourite memory?", Type: TypeText, Required: true},
		{ID: "q_one", Prompt: "Pick one", Type: TypeSingleChoice, Required: true,
			Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}},
		{ID: "q_many", Prompt: "Pick some", Type: TypeMultiChoice, Required: false,
			Options: []Option{{ID: "x", Label: "X"}, {ID: "y", Label: "Y"}}},
	}
}

func TestValidateAnswers_Valid(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_txt", Text: "the beach"},
		{QuestionID: "q_one", ChoiceIDs: []string{"a"}},
		{QuestionID: "q_many", ChoiceIDs: []string{"x", "y"}},
	}
	if err := ValidateAnswers(questions(), ans); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateAnswers_OptionalOmitted(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_txt", Text: "the beach"},
		{QuestionID: "q_one", ChoiceIDs: []string{"b"}},
	}
	if err := ValidateAnswers(questions(), ans); err != nil {
		t.Fatalf("optional question may be omitted, got %v", err)
	}
}

func TestValidateAnswers_MissingRequired(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_one", ChoiceIDs: []string{"a"}},
	}
	if err := ValidateAnswers(questions(), ans); err == nil {
		t.Fatal("expected error for missing required text question")
	}
}

func TestValidateAnswers_RequiredTextEmpty(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_txt", Text: "   "},
		{QuestionID: "q_one", ChoiceIDs: []string{"a"}},
	}
	if err := ValidateAnswers(questions(), ans); err == nil {
		t.Fatal("expected error: required text answer is blank")
	}
}

func TestValidateAnswers_UnknownQuestion(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_txt", Text: "x"},
		{QuestionID: "q_one", ChoiceIDs: []string{"a"}},
		{QuestionID: "ghost", Text: "boo"},
	}
	if err := ValidateAnswers(questions(), ans); err == nil {
		t.Fatal("expected error for unknown question id")
	}
}

func TestValidateAnswers_ChoiceNotAnOption(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_txt", Text: "x"},
		{QuestionID: "q_one", ChoiceIDs: []string{"z"}},
	}
	if err := ValidateAnswers(questions(), ans); err == nil {
		t.Fatal("expected error: choice not among options")
	}
}

func TestValidateAnswers_SingleChoiceMultipleSelected(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_txt", Text: "x"},
		{QuestionID: "q_one", ChoiceIDs: []string{"a", "b"}},
	}
	if err := ValidateAnswers(questions(), ans); err == nil {
		t.Fatal("expected error: single choice with multiple selections")
	}
}

func TestValidateAnswers_DuplicateAnswer(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_txt", Text: "x"},
		{QuestionID: "q_one", ChoiceIDs: []string{"a"}},
		{QuestionID: "q_one", ChoiceIDs: []string{"b"}},
	}
	if err := ValidateAnswers(questions(), ans); err == nil {
		t.Fatal("expected error: duplicate answer for same question")
	}
}

func TestValidateAnswers_TextOnChoiceQuestion(t *testing.T) {
	ans := []Answer{
		{QuestionID: "q_txt", Text: "x"},
		{QuestionID: "q_one", Text: "freeform", ChoiceIDs: []string{"a"}},
	}
	if err := ValidateAnswers(questions(), ans); err == nil {
		t.Fatal("expected error: free text supplied for a choice question")
	}
}
