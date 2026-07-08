// Package model holds the transport-independent poll domain types and the
// answer-validation rules. The Postgres store persists these as JSON; the server
// maps them to/from the generated proto messages. Keeping them here means the
// database and business rules never depend on generated transport code.
package model

import "time"

type QuestionType string

const (
	TypeText         QuestionType = "text"
	TypeSingleChoice QuestionType = "single_choice"
	TypeMultiChoice  QuestionType = "multi_choice"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusExpired   Status = "expired"
)

type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Question struct {
	ID       string       `json:"id"`
	Prompt   string       `json:"prompt"`
	Type     QuestionType `json:"type"`
	Options  []Option     `json:"options,omitempty"`
	Required bool         `json:"required"`
}

type Answer struct {
	QuestionID string   `json:"question_id"`
	Text       string   `json:"text,omitempty"`
	ChoiceIDs  []string `json:"choice_ids,omitempty"`
}

type Poll struct {
	ID                string
	OwnerUserID       string
	SurpriseRequestID string
	Title             string
	Questions         []Question
	Status            Status
	ExpiresAt         time.Time
	CreatedAt         time.Time
}

type Response struct {
	ID          string
	Answers     []Answer
	SubmittedAt time.Time
}
