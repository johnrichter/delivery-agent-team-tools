package schedule

import (
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// TaskInfo identifies one task the build loop should run.
type TaskInfo struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Summary string       `json:"summary"`
	Model   model.Model  `json:"model"`
	Effort  model.Effort `json:"effort"`
}

// NextResult is exactly one of: a runnable task, an orchestrator-only refusal, done, or
// blocked. OrchestratorOnly is set instead of Task (never both) when the next eligible task in
// dependency order must run inline in the orchestrator — a subagent-dispatch attempt on it is
// a structural refusal, never a prose caveat a caller could ignore.
type NextResult struct {
	Task             *TaskInfo `json:"task,omitempty"`
	OrchestratorOnly *TaskInfo `json:"orchestrator_only,omitempty"`
	Done             bool      `json:"done,omitempty"`
	DepsMet          bool      `json:"deps_met,omitempty"`
	Blocked          []string  `json:"blocked,omitempty"`
	Reason           string    `json:"reason,omitempty"`
}

// Next returns the first task in dependency order that is not terminal and has every dep done.
func Next(ex model.ExecState, p model.Plan) NextResult {
	s := newScheduleState(ex, p)
	if !s.anyUnfinished() {
		if len(s.topo.Cycle) > 0 {
			return NextResult{Blocked: s.topo.Cycle, Reason: "unschedulable (cycle/dangling deps): " + strings.Join(s.topo.Cycle, ", ")}
		}
		return NextResult{Done: true}
	}
	for _, id := range s.topo.Order {
		if !s.eligible(id) {
			continue
		}
		ref := s.refByID[id]
		row := s.rowByID[id]
		info := &TaskInfo{ID: id, Name: ref.Task.Name, Summary: row.Summary, Model: row.Model, Effort: row.Effort}
		if ref.Task.OrchestratorOnly {
			return NextResult{OrchestratorOnly: info, Reason: "task " + id + " is orchestrator_only — run inline, refused for subagent dispatch"}
		}
		return NextResult{Task: info, DepsMet: true}
	}
	return NextResult{Blocked: s.unfinishedIDs(), Reason: s.blockedReason()}
}
