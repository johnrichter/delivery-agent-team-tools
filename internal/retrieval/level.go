// Package retrieval is the level-of-detail retrieval API over plan.json, execution.json, and
// archive.json: outline (every entity) -> milestone/phase (one group's child tasks) -> task
// (one task's full record) -> field (one named field). Every level is a pure, read-only
// projection; retrieval never decides task eligibility (schedule owns that).
package retrieval

import (
	"fmt"
	"regexp"
)

// Level selects retrieval granularity.
type Level string

const (
	LevelOutline   Level = "outline"
	LevelMilestone Level = "milestone"
	LevelPhase     Level = "phase"
	LevelTask      Level = "task"
	LevelField     Level = "field"
)

func (l Level) Known() bool {
	switch l {
	case LevelOutline, LevelMilestone, LevelPhase, LevelTask, LevelField:
		return true
	}
	return false
}

// Input selects what a retrieval call projects: outline needs no id; milestone/phase/task
// require id; field additionally requires field.
type Input struct {
	Level Level
	ID    string
	Field string
}

// OutlineEntry is the L1 outline projection: one milestone, phase, or task reduced to its
// identifying fields.
type OutlineEntry struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Status string   `json:"status,omitempty"`
	Deps   []string `json:"deps,omitempty"`
}

// GroupView is the L2 milestone/phase projection: the group's own identity plus its descendant
// tasks flattened to outline entries.
type GroupView struct {
	ID    string         `json:"id"`
	Name  string         `json:"name,omitempty"`
	Tasks []OutlineEntry `json:"tasks"`
}

// FieldValue is the L4 field projection: one named field of one entity, in a self-describing
// envelope so a caller never has to guess the value's shape.
type FieldValue struct {
	ID    string `json:"id"`
	Field string `json:"field"`
	Value any    `json:"value"`
}

var (
	reMilestone = regexp.MustCompile(`^M[0-9]+$`)
	rePhase     = regexp.MustCompile(`^M[0-9]+\.P[0-9]+$`)
	reTask      = regexp.MustCompile(`^M[0-9]+\.P[0-9]+\.T[0-9]+$`)
)

// entityKind classifies id by its hierarchical shape.
func entityKind(id string) string {
	switch {
	case reTask.MatchString(id):
		return "task"
	case rePhase.MatchString(id):
		return "phase"
	case reMilestone.MatchString(id):
		return "milestone"
	default:
		return ""
	}
}

func errUnknownLevel(l Level) error {
	return fmt.Errorf("retrieve: unknown level %q (want %s|%s|%s|%s|%s)", l, LevelOutline, LevelMilestone, LevelPhase, LevelTask, LevelField)
}
