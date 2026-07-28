package model

// ArchiveSchema marks archive.json's shape so the doc-shape sniffer routes it ahead of the
// plan/exec fork (archive.json also carries a top-level "milestones" key).
const ArchiveSchema = "archive/v1"

// ArchiveDoc is the full-fidelity preserved record the explicit `archive` op writes/extends.
type ArchiveDoc struct {
	Schema     string              `json:"schema"`
	Milestones []ArchivedMilestone `json:"milestones,omitempty"`
}

type ArchivedMilestone struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Phases []ArchivedPhase `json:"phases"`
}

type ArchivedPhase struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Tasks []ArchivedTask `json:"tasks"`
}

// ArchivedTask is one archived task's full preserved record: the union of its immutable
// plan-slice and frozen exec-slice.
type ArchivedTask struct {
	Summary      string             `json:"summary"`
	Deliverable  string             `json:"deliverable"`
	Kind         DeliverableKind    `json:"deliverable_kind,omitempty"`
	Model        Model              `json:"model"`
	Effort       Effort             `json:"effort"`
	Thinking     string             `json:"thinking,omitempty"`
	TestStrategy string             `json:"test_strategy"`
	Deps         []string           `json:"deps,omitempty"`
	Acceptance   []string           `json:"acceptance"`
	FileSurface  []FileSurfaceEntry `json:"file_surface,omitempty"`

	ID        string  `json:"id"`
	Status    Status  `json:"status"`
	Test      string  `json:"test,omitempty"`
	Review    string  `json:"review,omitempty"`
	Commit    string  `json:"commit,omitempty"`
	CostUSD   float64 `json:"cost_usd"`
	TokensOut int64   `json:"tokens_out"`
	Usage     *Usage  `json:"usage,omitempty"`
	Updated   string  `json:"updated"`
	Notes     string  `json:"notes,omitempty"`
}
