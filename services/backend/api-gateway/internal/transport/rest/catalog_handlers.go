package rest

import (
	"net/http"
	"strconv"

	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/catalog/v1"
)

type holidayDTO struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	DateRule string   `json:"date_rule,omitempty"`
	Region   string   `json:"region,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Active   bool     `json:"active"`
}

// GET /v1/holidays → Catalog.ListHolidays. Query params: region, active, on_or_after.
func (s *Server) handleListHolidays(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var activePtr *bool // ListHolidaysRequest.active is optional (unset = all)
	if v := q.Get("active"); v != "" {
		b, _ := strconv.ParseBool(v)
		activePtr = &b
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Catalog.ListHolidays(ctx, &catalogv1.ListHolidaysRequest{
		Region:    q.Get("region"),
		Active:    activePtr,
		OnOrAfter: q.Get("on_or_after"),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := make([]holidayDTO, 0, len(resp.GetHolidays()))
	for _, h := range resp.GetHolidays() {
		out = append(out, holidayDTO{
			ID: h.GetId(), Name: h.GetName(), DateRule: h.GetDateRule().String(),
			Region: h.GetRegion(), Tags: h.GetTags(), Active: h.GetActive(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"holidays": out})
}

type categoryDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
}

type budgetBandDTO struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Min      int64  `json:"min"`
	Max      int64  `json:"max"`
	Currency string `json:"currency,omitempty"`
}

// GET /v1/categories → Catalog.GetCategories. Query param: kind.
func (s *Server) handleGetCategories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Catalog.GetCategories(ctx, &catalogv1.GetCategoriesRequest{
		Kind: catalogv1.CategoryKind(catalogv1.CategoryKind_value[r.URL.Query().Get("kind")]),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	cats := make([]categoryDTO, 0, len(resp.GetCategories()))
	for _, c := range resp.GetCategories() {
		cats = append(cats, categoryDTO{ID: c.GetId(), Name: c.GetName(), Kind: c.GetKind().String(), ParentID: c.GetParentId()})
	}
	bands := make([]budgetBandDTO, 0, len(resp.GetBudgetBands()))
	for _, b := range resp.GetBudgetBands() {
		bands = append(bands, budgetBandDTO{
			ID: b.GetId(), Label: b.GetLabel(), Min: int64(b.GetMinCents()), Max: int64(b.GetMaxCents()), Currency: b.GetCurrency(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats, "budget_bands": bands})
}
