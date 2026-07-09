package model

import (
	"fmt"
	"strings"
)

// ValidateQuestions checks a poll definition at creation time: at least one
// question, unique non-empty ids, non-blank prompts, a known type, and — for
// choice questions — a non-empty set of options with unique ids.
func ValidateQuestions(questions []Question) error {
	if len(questions) == 0 {
		return fmt.Errorf("a poll needs at least one question")
	}
	seen := make(map[string]bool, len(questions))
	for _, q := range questions {
		if strings.TrimSpace(q.ID) == "" {
			return fmt.Errorf("question id must not be empty")
		}
		if seen[q.ID] {
			return fmt.Errorf("duplicate question id %q", q.ID)
		}
		seen[q.ID] = true
		if strings.TrimSpace(q.Prompt) == "" {
			return fmt.Errorf("question %q has a blank prompt", q.ID)
		}
		switch q.Type {
		case TypeText:
			// no options expected
		case TypeSingleChoice, TypeMultiChoice:
			if len(q.Options) == 0 {
				return fmt.Errorf("choice question %q needs options", q.ID)
			}
			optSeen := make(map[string]bool, len(q.Options))
			for _, o := range q.Options {
				if strings.TrimSpace(o.ID) == "" {
					return fmt.Errorf("question %q has an option with empty id", q.ID)
				}
				if optSeen[o.ID] {
					return fmt.Errorf("question %q has duplicate option id %q", q.ID, o.ID)
				}
				optSeen[o.ID] = true
			}
		default:
			return fmt.Errorf("question %q has unknown type %q", q.ID, q.Type)
		}
	}
	return nil
}
