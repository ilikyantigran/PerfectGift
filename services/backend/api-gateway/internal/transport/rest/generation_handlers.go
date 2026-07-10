package rest

import (
	"net/http"

	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/surprise/v1"
)

type requestGenerationRequest struct {
	HolidayID       string `json:"holiday_id"`
	BudgetBand      string `json:"budget_band"`
	PreferencesText string `json:"preferences_text"`
	PollID          string `json:"poll_id"`
	Tier            string `json:"tier"`
}

type ideaDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	WhyItFits string `json:"why_it_fits"`
	RoughCost string `json:"rough_cost"`
	HowTo     string `json:"how_to"`
	Rank      int32  `json:"rank"`
}

func ideaToDTO(i *surprisev1.Idea) ideaDTO {
	return ideaDTO{
		ID:        i.GetId(),
		Title:     i.GetTitle(),
		WhyItFits: i.GetWhyItFits(),
		RoughCost: i.GetRoughCost(),
		HowTo:     i.GetHowTo(),
		Rank:      int32(i.GetRank()),
	}
}

// POST /v1/generations → Surprise.RequestGeneration.
// Returns 202 Accepted {request_id} immediately (never blocks on the LLM). The
// Idempotency-Key header is forwarded to Surprise. user_id comes from the JWT.
func (s *Server) handleRequestGeneration(w http.ResponseWriter, r *http.Request) {
	var body requestGenerationRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Surprise.RequestGeneration(ctx, &surprisev1.RequestGenerationRequest{
		UserId:          subjectFrom(r.Context()),
		HolidayId:       body.HolidayID,
		BudgetBand:      body.BudgetBand,
		PreferencesText: body.PreferencesText,
		PollId:          body.PollID,
		Tier:            surprisev1.ModelTier(surprisev1.ModelTier_value[body.Tier]),
		IdempotencyKey:  r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"request_id": resp.GetRequestId(),
		"status":     resp.GetStatus().String(),
	})
}

type generationStatusResponse struct {
	RequestID string    `json:"request_id"`
	Status    string    `json:"status"`
	Progress  int32     `json:"progress"`
	Ideas     []ideaDTO `json:"ideas,omitempty"`
}

// GET /v1/generations/{id} → Surprise.GetGenerationStatus, and when the status is
// "ready" also Surprise.GetIdeas — aggregated into one mobile-friendly payload (BFF).
func (s *Server) handleGetGeneration(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("id")
	ctx, cancel := reqCtx(r)
	defer cancel()

	// user_id is not on these requests — surprise reads it from "x-user-id" metadata (forwarded by reqCtx).
	statusResp, err := s.opts.Surprise.GetGenerationStatus(ctx, &surprisev1.GetGenerationStatusRequest{
		RequestId: requestID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	out := generationStatusResponse{
		RequestID: requestID,
		Status:    statusResp.GetStatus().String(),
		Progress:  int32(statusResp.GetProgress()),
	}

	if statusResp.GetStatus() == surprisev1.GenerationStatus_GENERATION_STATUS_READY {
		ideasResp, err := s.opts.Surprise.GetIdeas(ctx, &surprisev1.GetIdeasRequest{
			RequestId: requestID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		out.Ideas = make([]ideaDTO, 0, len(ideasResp.GetIdeas()))
		for _, i := range ideasResp.GetIdeas() {
			out.Ideas = append(out.Ideas, ideaToDTO(i))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type refineRequest struct {
	Refinement string `json:"refinement"`
}

// POST /v1/generations/{id}/refine → Surprise.Refine (async → 202 {request_id})
func (s *Server) handleRefine(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("id")
	var body refineRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Surprise.Refine(ctx, &surprisev1.RefineRequest{
		RequestId:  requestID,
		Refinement: body.Refinement,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"request_id": resp.GetRequestId(),
		"status":     resp.GetStatus().String(),
	})
}

// POST /v1/ideas/{id}/save → Surprise.SaveIdea
func (s *Server) handleSaveIdea(w http.ResponseWriter, r *http.Request) {
	ideaID := r.PathValue("id")
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Surprise.SaveIdea(ctx, &surprisev1.SaveIdeaRequest{
		UserId: subjectFrom(r.Context()),
		IdeaId: ideaID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": resp.GetOk()})
}
