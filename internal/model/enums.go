// Package model is the wire-format contract this engine reads and writes: plan.json,
// execution.json, archive.json, and feedback.json. Every JSON tag here is part of the frozen
// I/O surface the CLI commands expose — changing one changes the contract, not an
// implementation detail.
package model

// Model is a pinned full model ID. Validity against the live roster is checked at runtime
// (internal/planops); the enum here only fixes the wire type.
type Model string

// Effort is a reasoning-effort tier; availability is model-dependent (roster-derived).
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortXHigh  Effort = "xhigh"
	EffortMax    Effort = "max"
)

func (e Effort) Known() bool {
	switch e {
	case EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax:
		return true
	}
	return false
}

// ModelInherit is the dispatch sentinel meaning "inherit the caller's own model", exempt from
// per-model effort-availability checks.
const ModelInherit Model = "inherit"

// DeliverableKind selects the per-task build path. The set is closed.
type DeliverableKind string

const (
	KindCode DeliverableKind = "code"
	KindDocs DeliverableKind = "docs"
)

func (k DeliverableKind) Known() bool {
	switch k {
	case KindCode, KindDocs:
		return true
	}
	return false
}

// Resolve maps the empty/unset kind to the code default.
func (k DeliverableKind) Resolve() DeliverableKind {
	if k == "" {
		return KindCode
	}
	return k
}

// FileSurfaceKind is the match semantics pinned to one file_surface entry.
type FileSurfaceKind string

const (
	FSFile FileSurfaceKind = "file"
	FSGlob FileSurfaceKind = "glob"
	FSDir  FileSurfaceKind = "dir"
)

func (k FileSurfaceKind) Known() bool {
	switch k {
	case FSFile, FSGlob, FSDir:
		return true
	}
	return false
}

// Resolve maps the empty/unset kind to the file default.
func (k FileSurfaceKind) Resolve() FileSurfaceKind {
	if k == "" {
		return FSFile
	}
	return k
}

// Status is a task's execution lifecycle state. The set is closed.
type Status string

const (
	StatusNotStarted Status = "not-started"
	StatusInProgress Status = "in-progress"
	StatusBlocked    Status = "blocked"
	StatusFailed     Status = "failed"
	StatusDone       Status = "done"
	StatusSuperseded Status = "superseded"
)

func (s Status) Known() bool {
	switch s {
	case StatusNotStarted, StatusInProgress, StatusBlocked, StatusFailed, StatusDone, StatusSuperseded:
		return true
	}
	return false
}

// Terminal reports whether a task in this status is finished for scheduling purposes.
func (s Status) Terminal() bool { return s == StatusDone || s == StatusSuperseded }

// Emoji is the execution.md mirror glyph for s.
func (s Status) Emoji() string {
	switch s {
	case StatusNotStarted:
		return "🟢"
	case StatusInProgress:
		return "🟡"
	case StatusBlocked:
		return "⛔"
	case StatusFailed:
		return "🔴"
	case StatusDone:
		return "✅"
	case StatusSuperseded:
		return "⊘"
	}
	return "🟢"
}

// PauseReason categorizes why a build paused. git|state|merge are tooling-forced (mechanical);
// design-level|approval|budget|signing are operator-facing gates.
type PauseReason string

const (
	PauseGit         PauseReason = "git"
	PauseState       PauseReason = "state"
	PauseMerge       PauseReason = "merge"
	PauseDesignLevel PauseReason = "design-level"
	PauseApproval    PauseReason = "approval"
	PauseBudget      PauseReason = "budget"
	PauseSigning     PauseReason = "signing"
)

func (r PauseReason) Known() bool {
	switch r {
	case PauseGit, PauseState, PauseMerge, PauseDesignLevel, PauseApproval, PauseBudget, PauseSigning:
		return true
	}
	return false
}

// Mechanical reports whether r is a tooling-forced pause (git|state|merge).
func (r PauseReason) Mechanical() bool {
	switch r {
	case PauseGit, PauseState, PauseMerge:
		return true
	}
	return false
}
