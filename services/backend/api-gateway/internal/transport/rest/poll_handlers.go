package rest

import (
	"net/http"
	"strings"

	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/poll/v1"
)

// --- JSON DTOs ---

type questionDTO struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Kind    string   `json:"kind,omitempty"`
	Options []string `json:"options,omitempty"`
}

type answerDTO struct {
	QuestionID string   `json:"question_id"`
	Value      string   `json:"value,omitempty"`
	Values     []string `json:"values,omitempty"`
}

func questionToProto(q questionDTO) *pollv1.Question {
	return &pollv1.Question{Id: q.ID, Text: q.Text, Kind: q.Kind, Options: q.Options}
}

func questionToDTO(q *pollv1.Question) questionDTO {
	return questionDTO{ID: q.GetId(), Text: q.GetText(), Kind: q.GetKind(), Options: q.GetOptions()}
}

func answerToProto(a answerDTO) *pollv1.Answer {
	return &pollv1.Answer{QuestionId: a.QuestionID, Value: a.Value, Values: a.Values}
}

func answerToDTO(a *pollv1.Answer) answerDTO {
	return answerDTO{QuestionID: a.GetQuestionId(), Value: a.GetValue(), Values: a.GetValues()}
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
		OwnerUserId:       subjectFrom(r.Context()),
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
		PollId:      pollID,
		OwnerUserId: subjectFrom(r.Context()),
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
	if token == "" {
		// Matched via the literal `/v1/polls/token/responses` disambiguator pattern,
		// which has no {t} wildcard: recover the token from the trailing segment.
		token = strings.TrimPrefix(r.URL.Path, "/v1/polls/token/")
	}
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
