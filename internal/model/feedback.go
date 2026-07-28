package model

// FeedbackSchema marks feedback.json's shape.
const FeedbackSchema = "feedback-register/v1"

// FeedbackEntry is one register row. ID and Criticality are always engine-derived; a caller
// never supplies either directly.
type FeedbackEntry struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	SourceTaskID     string `json:"source_task_id,omitempty"`
	Feedback         string `json:"feedback"`
	ProposedSolution string `json:"proposed_solution,omitempty"`
	WhyItMatters     string `json:"why_it_matters,omitempty"`
	Impact           int    `json:"impact"`
	Urgency          int    `json:"urgency"`
	Criticality      int    `json:"criticality"`
	Added            string `json:"added,omitempty"`
}

// FeedbackRegister is the canonical feedback.json document: an append-only, project-scoped
// list of FeedbackEntry.
type FeedbackRegister struct {
	Schema  string          `json:"schema"`
	Project string          `json:"project,omitempty"`
	Entries []FeedbackEntry `json:"entries"`
}
