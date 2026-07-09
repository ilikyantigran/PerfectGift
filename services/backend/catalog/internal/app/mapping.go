package app

import (
	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/domain/model"
	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/catalog/pkg/api/catalog/v1"
)

// Mappers between the neutral domain model and the generated proto types. Kept in
// one place so the enum/string translations live together.

func holidayToProto(h model.Holiday) *catalogv1.Holiday {
	return &catalogv1.Holiday{
		Id:       h.ID,
		Name:     h.Name,
		DateRule: dateRuleToProto(h.DateRule),
		Region:   h.Region,
		Tags:     h.Tags,
		Active:   h.Active,
	}
}

func categoryToProto(c model.Category) *catalogv1.Category {
	return &catalogv1.Category{
		Id:       c.ID,
		Name:     c.Name,
		Kind:     kindToProto(c.Kind),
		ParentId: c.ParentID,
	}
}

func bandToProto(b model.BudgetBand) *catalogv1.BudgetBand {
	return &catalogv1.BudgetBand{
		Id:       b.ID,
		Label:    b.Label,
		MinCents: b.MinCents,
		MaxCents: b.MaxCents,
		Currency: b.Currency,
	}
}

func snippetToProto(s model.Snippet) *catalogv1.Snippet {
	return &catalogv1.Snippet{
		Id:           s.ID,
		Title:        s.Title,
		Body:         s.Body,
		CategoryId:   s.CategoryID,
		BudgetBandId: s.BudgetBandID,
		Tags:         s.Tags,
		Score:        s.Score,
	}
}

func dateRuleToProto(s string) catalogv1.DateRule {
	switch s {
	case model.DateRuleFixed:
		return catalogv1.DateRule_DATE_RULE_FIXED
	case model.DateRuleRelative:
		return catalogv1.DateRule_DATE_RULE_RELATIVE
	default:
		return catalogv1.DateRule_DATE_RULE_UNSPECIFIED
	}
}

func kindToProto(s string) catalogv1.CategoryKind {
	switch s {
	case model.KindGift:
		return catalogv1.CategoryKind_CATEGORY_KIND_GIFT
	case model.KindDate:
		return catalogv1.CategoryKind_CATEGORY_KIND_DATE
	default:
		return catalogv1.CategoryKind_CATEGORY_KIND_UNSPECIFIED
	}
}

func kindFromProto(k catalogv1.CategoryKind) string {
	switch k {
	case catalogv1.CategoryKind_CATEGORY_KIND_GIFT:
		return model.KindGift
	case catalogv1.CategoryKind_CATEGORY_KIND_DATE:
		return model.KindDate
	default:
		return ""
	}
}
