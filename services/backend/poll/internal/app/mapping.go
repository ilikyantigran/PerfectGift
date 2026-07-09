package app

import (
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/model"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/poll/pkg/api/poll/v1"
)

func questionTypeFromProto(t pollv1.QuestionType) model.QuestionType {
	switch t {
	case pollv1.QuestionType_QUESTION_TYPE_TEXT:
		return model.TypeText
	case pollv1.QuestionType_QUESTION_TYPE_SINGLE_CHOICE:
		return model.TypeSingleChoice
	case pollv1.QuestionType_QUESTION_TYPE_MULTI_CHOICE:
		return model.TypeMultiChoice
	default:
		return model.QuestionType("")
	}
}

func questionTypeToProto(t model.QuestionType) pollv1.QuestionType {
	switch t {
	case model.TypeText:
		return pollv1.QuestionType_QUESTION_TYPE_TEXT
	case model.TypeSingleChoice:
		return pollv1.QuestionType_QUESTION_TYPE_SINGLE_CHOICE
	case model.TypeMultiChoice:
		return pollv1.QuestionType_QUESTION_TYPE_MULTI_CHOICE
	default:
		return pollv1.QuestionType_QUESTION_TYPE_UNSPECIFIED
	}
}

func questionsFromProto(in []*pollv1.Question) []model.Question {
	out := make([]model.Question, 0, len(in))
	for _, q := range in {
		opts := make([]model.Option, 0, len(q.GetOptions()))
		for _, o := range q.GetOptions() {
			opts = append(opts, model.Option{ID: o.GetId(), Label: o.GetLabel()})
		}
		out = append(out, model.Question{
			ID:       q.GetId(),
			Prompt:   q.GetPrompt(),
			Type:     questionTypeFromProto(q.GetType()),
			Options:  opts,
			Required: q.GetRequired(),
		})
	}
	return out
}

func questionsToProto(in []model.Question) []*pollv1.Question {
	out := make([]*pollv1.Question, 0, len(in))
	for _, q := range in {
		opts := make([]*pollv1.Option, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, &pollv1.Option{Id: o.ID, Label: o.Label})
		}
		out = append(out, &pollv1.Question{
			Id:       q.ID,
			Prompt:   q.Prompt,
			Type:     questionTypeToProto(q.Type),
			Options:  opts,
			Required: q.Required,
		})
	}
	return out
}

func answersFromProto(in []*pollv1.Answer) []model.Answer {
	out := make([]model.Answer, 0, len(in))
	for _, a := range in {
		out = append(out, model.Answer{
			QuestionID: a.GetQuestionId(),
			Text:       a.GetText(),
			ChoiceIDs:  a.GetChoiceIds(),
		})
	}
	return out
}

func answersToProto(in []model.Answer) []*pollv1.Answer {
	out := make([]*pollv1.Answer, 0, len(in))
	for _, a := range in {
		out = append(out, &pollv1.Answer{
			QuestionId: a.QuestionID,
			Text:       a.Text,
			ChoiceIds:  a.ChoiceIDs,
		})
	}
	return out
}
