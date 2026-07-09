package model

import (
	"fmt"
	"strings"
)

// ValidateAnswers checks a Subject's submission against the poll's questions.
// It returns a descriptive error (mapped to InvalidArgument at the edge) when:
//   - an answer targets an unknown question id,
//   - the same question is answered more than once,
//   - a choice answer selects an option that isn't offered,
//   - a single-choice answer selects other than exactly one option,
//   - free text is supplied for a choice question (or choices for a text question),
//   - a required question is left effectively blank.
func ValidateAnswers(questions []Question, answers []Answer) error {
	byID := make(map[string]Question, len(questions))
	for _, q := range questions {
		byID[q.ID] = q
	}

	answered := make(map[string]bool, len(answers))
	for _, a := range answers {
		q, ok := byID[a.QuestionID]
		if !ok {
			return fmt.Errorf("answer for unknown question %q", a.QuestionID)
		}
		if answered[a.QuestionID] {
			return fmt.Errorf("duplicate answer for question %q", a.QuestionID)
		}
		answered[a.QuestionID] = true

		switch q.Type {
		case TypeText:
			if len(a.ChoiceIDs) > 0 {
				return fmt.Errorf("question %q is free text; choices not allowed", q.ID)
			}
		case TypeSingleChoice, TypeMultiChoice:
			if strings.TrimSpace(a.Text) != "" {
				return fmt.Errorf("question %q is a choice question; free text not allowed", q.ID)
			}
			if q.Type == TypeSingleChoice && len(a.ChoiceIDs) > 1 {
				return fmt.Errorf("question %q accepts a single choice", q.ID)
			}
			opts := optionSet(q)
			for _, c := range a.ChoiceIDs {
				if !opts[c] {
					return fmt.Errorf("question %q: %q is not a valid option", q.ID, c)
				}
			}
		default:
			return fmt.Errorf("question %q has unknown type %q", q.ID, q.Type)
		}
	}

	for _, q := range questions {
		if !q.Required {
			continue
		}
		if err := requireAnswered(q, answers); err != nil {
			return err
		}
	}
	return nil
}

func requireAnswered(q Question, answers []Answer) error {
	for _, a := range answers {
		if a.QuestionID != q.ID {
			continue
		}
		switch q.Type {
		case TypeText:
			if strings.TrimSpace(a.Text) == "" {
				return fmt.Errorf("question %q is required", q.ID)
			}
		default:
			if len(a.ChoiceIDs) == 0 {
				return fmt.Errorf("question %q is required", q.ID)
			}
		}
		return nil
	}
	return fmt.Errorf("question %q is required", q.ID)
}

func optionSet(q Question) map[string]bool {
	set := make(map[string]bool, len(q.Options))
	for _, o := range q.Options {
		set[o.ID] = true
	}
	return set
}
