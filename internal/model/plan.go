package model

import (
	"bytes"
	"encoding/json"
)

// Plan is the canonical, immutable build spec: milestone → phase → task, hierarchically IDed
// (M1 / M1.P1 / M1.P1.T1). Live per-task status never lives here — that is ExecState's job.
type Plan struct {
	Goal            string      `json:"goal"`
	SuccessCriteria []string    `json:"success_criteria"`
	Assumptions     []string    `json:"assumptions,omitempty"`
	Tradeoffs       []Tradeoff  `json:"tradeoffs,omitempty"`
	Milestones      []Milestone `json:"milestones"`
	Risks           []Risk      `json:"risks,omitempty"`
	OpenQuestions   []string    `json:"open_questions,omitempty"`
	Provenance      *Provenance `json:"provenance,omitempty"`
}

// Tradeoff is one consequential design fork the architect resolved.
type Tradeoff struct {
	Decision       string   `json:"decision"`
	Options        []string `json:"options,omitempty"`
	Recommendation string   `json:"recommendation"`
	Why            string   `json:"why"`
}

// Risk maps to a build-time hazard; ForcesPause feeds the critical-surface escape hatch.
type Risk struct {
	Risk        string `json:"risk"`
	Mitigation  string `json:"mitigation,omitempty"`
	ForcesPause bool   `json:"forces_pause,omitempty"`
}

type Milestone struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Phases []Phase `json:"phases"`
}

type Phase struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Tasks []Task `json:"tasks"`
}

// Task is the smallest unit of build work. OrchestratorOnly marks a task that must run inline
// in the orchestrator, never as a subagent dispatch.
type Task struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Summary          string             `json:"summary"`
	Deliverable      string             `json:"deliverable"`
	Kind             DeliverableKind    `json:"deliverable_kind,omitempty"`
	Model            Model              `json:"model"`
	Effort           Effort             `json:"effort"`
	Thinking         string             `json:"thinking,omitempty"`
	TestStrategy     string             `json:"test_strategy"`
	Deps             []string           `json:"deps,omitempty"`
	Acceptance       []string           `json:"acceptance"`
	FileSurface      []FileSurfaceEntry `json:"file_surface,omitempty"`
	OrchestratorOnly bool               `json:"orchestrator_only,omitempty"`
}

// FileSurfaceEntry is one file/glob/dir a task may read or write.
type FileSurfaceEntry struct {
	Path     string          `json:"path"`
	Required bool            `json:"required,omitempty"`
	Kind     FileSurfaceKind `json:"kind,omitempty"`
}

// UnmarshalJSON accepts either the typed object form {path,required,kind} or a bare JSON
// string (the legacy shorthand), so every existing plain-string plan.json stays parseable.
func (e *FileSurfaceEntry) UnmarshalJSON(data []byte) error {
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*e = FileSurfaceEntry{Path: s}
		return nil
	}
	type alias FileSurfaceEntry
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*e = FileSurfaceEntry(a)
	return nil
}

// Provenance records the upstream timestamps a derivative was built from, for staleness
// detection. Optional in plan.json; always present in execution.json.
type Provenance struct {
	DesignUpdated string `json:"design_updated"`
	PlanUpdated   string `json:"plan_updated"`
	DerivedAt     string `json:"derived_at,omitempty"`
}
