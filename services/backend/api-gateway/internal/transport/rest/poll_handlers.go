package rest

import (
	"net/http"

	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/poll/v1"
)

// --- JSON DTOs (aligned to poll.v1: prompt/type/options + text/choice_ids) ---

type optionDTO struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type questionDTO struct {
	ID       string      `json:"id"`
	Prompt   string      `json:"prompt"`
	Type     string      `json:"type,omitempty"` // QUESTION_TYPE_TEXT | _SINGLE_CHOICE | _MULTI_CHOICE
	Options  []optionDTO `json:"options,omitempty"`
	Required bool        `json:"required"`
}

type answerDTO struct {
	QuestionID string   `json:"question_id"`
	Text       string   `json:"text,omitempty"`       // for TEXT questions
	ChoiceIDs  []string `json:"choice_ids,omitempty"` // for CHOICE questions
}

func questionToProto(q questionDTO) *pollv1.Question {
	opts := make([]*pollv1.Option, 0, len(q.Options))
	for _, o := range q.Options {
		opts = append(opts, &pollv1.Option{Id: o.ID, Label: o.Label})
	}
	return &pollv1.Question{
		Id:       q.ID,
		Prompt:   q.Prompt,
		Type:     pollv1.QuestionType(pollv1.QuestionType_value[q.Type]),
		Options:  opts,
		Required: q.Required,
	}
}

func questionToDTO(q *pollv1.Question) questionDTO {
	opts := make([]optionDTO, 0, len(q.GetOptions()))
	for _, o := range q.GetOptions() {
		opts = append(opts, optionDTO{ID: o.GetId(), Label: o.GetLabel()})
	}
	return questionDTO{
		ID:       q.GetId(),
		Prompt:   q.GetPrompt(),
		Type:     q.GetType().String(),
		Options:  opts,
		Required: q.GetRequired(),
	}
}

func answerToProto(a answerDTO) *pollv1.Answer {
	return &pollv1.Answer{QuestionId: a.QuestionID, Text: a.Text, ChoiceIds: a.ChoiceIDs}
}

func answerToDTO(a *pollv1.Answer) answerDTO {
	return answerDTO{QuestionID: a.GetQuestionId(), Text: a.GetText(), ChoiceIDs: a.GetChoiceIds()}
}

type createPollRequest struct {
	Title             string        `json:"title"`
	Questions         []questionDTO `json:"questions"`
	SurpriseRequestID string        `json:"surprise_request_id"`
	TTLSeconds        int64         `json:"ttl_seconds"`
}

type createPollResponse struct {
	PollID    string `json:"poll_id"`
	LinkToken string `json:"link_token"`
	LinkURL   string `json:"link_url"`
	ExpiresAt string `json:"expires_at"`
}

// POST /v1/polls → Poll.CreatePoll (owner_user_id from JWT subject)
func (s *Server) handleCreatePoll(w http.ResponseWriter, r *http.Request) {
	var body createPollRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	questions := make([]*pollv1.Question, 0, len(body.Questions))
	for _, q := range body.Questions {
		questions = append(questions, questionToProto(q))
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Poll.CreatePoll(ctx, &pollv1.CreatePollRequest{
		Title:             body.Title,
		Questions:         questions,
		SurpriseRequestId: body.SurpriseRequestID,
		TtlSeconds:        body.TTLSeconds,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, createPollResponse{
		PollID:    resp.GetPollId(),
		LinkToken: resp.GetLinkToken(),
		LinkURL:   resp.GetLinkUrl(),
		ExpiresAt: resp.GetExpiresAt(),
	})
}

type pollResponseDTO struct {
	ID          string      `json:"id"`
	Answers     []answerDTO `json:"answers"`
	SubmittedAt string      `json:"submitted_at"`
}

// GET /v1/polls/{id}/responses → Poll.GetResponses (owner-scoped by JWT subject)
func (s *Server) handleGetResponses(w http.ResponseWriter, r *http.Request) {
	pollID := r.PathValue("id")
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Poll.GetResponses(ctx, &pollv1.GetResponsesRequest{
		PollId: pollID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := make([]pollResponseDTO, 0, len(resp.GetResponses()))
	for _, pr := range resp.GetResponses() {
		answers := make([]answerDTO, 0, len(pr.GetAnswers()))
		for _, a := range pr.GetAnswers() {
			answers = append(answers, answerToDTO(a))
		}
		out = append(out, pollResponseDTO{ID: pr.GetId(), Answers: answers, SubmittedAt: pr.GetSubmittedAt()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"responses": out})
}

type getPollByTokenResponse struct {
	PollID    string        `json:"poll_id"`
	Title     string        `json:"title"`
	Questions []questionDTO `json:"questions"`
}

// GET /v1/polls/token/{t} → Poll.GetPollByToken (anonymous; opaque token; NOT JWT)
func (s *Server) handleGetPollByToken(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("t")
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Poll.GetPollByToken(ctx, &pollv1.GetPollByTokenRequest{Token: token})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	questions := make([]questionDTO, 0, len(resp.GetQuestions()))
	for _, q := range resp.GetQuestions() {
		questions = append(questions, questionToDTO(q))
	}
	writeJSON(w, http.StatusOK, getPollByTokenResponse{
		PollID:    resp.GetPollId(),
		Title:     resp.GetTitle(),
		Questions: questions,
	})
}

type submitResponseRequest struct {
	Answers []answerDTO `json:"answers"`
}

// POST /v1/polls/token/{t}/responses → Poll.SubmitResponse (anonymous; rate-limited)
func (s *Server) handleSubmitResponse(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("t")
	var body submitResponseRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	answers := make([]*pollv1.Answer, 0, len(body.Answers))
	for _, a := range body.Answers {
		answers = append(answers, answerToProto(a))
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Poll.SubmitResponse(ctx, &pollv1.SubmitResponseRequest{
		Token:   token,
		Answers: answers,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": resp.GetOk()})
}
