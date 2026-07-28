// Package schedule picks what runs next: `next` (single candidate) and `batch` (an
// independent, file_surface-disjoint parallel group), both scoped by run_config.pause_mode.
package schedule

import (
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
	"github.com/johnrichter/delivery-agent-team-tools/internal/planops"
)

// archiveAwareStatus resolves a task id's status from live Tasks first, falling back to the
// Archived tombstone index, and defaulting to not-started when the id is in neither. A live
// task depending on an archived (necessarily terminal) task must resolve that dependency as
// done, or it would stall forever on resume.
func archiveAwareStatus(ex model.ExecState) func(id string) model.Status {
	statusOf := make(map[string]model.Status, len(ex.Tasks)+len(ex.Archived))
	for _, t := range ex.Tasks {
		statusOf[t.ID] = t.Status
	}
	for _, a := range ex.Archived {
		if _, ok := statusOf[a.ID]; !ok {
			statusOf[a.ID] = a.Status
		}
	}
	return func(id string) model.Status {
		if s, ok := statusOf[id]; ok {
			return s
		}
		return model.StatusNotStarted
	}
}

// scheduleState is the shared view next/batch both compute once: the plan's dependency-derived
// topological order, each task's plan context, and its live/archived status.
type scheduleState struct {
	topo     planops.TopoResult
	refByID  map[string]planops.TaskRef
	rowByID  map[string]model.ExecTask
	statusOf func(id string) model.Status
}

func newScheduleState(ex model.ExecState, p model.Plan) scheduleState {
	refByID := map[string]planops.TaskRef{}
	for _, r := range planops.WalkTasks(p) {
		refByID[r.Task.ID] = r
	}
	rowByID := map[string]model.ExecTask{}
	for _, t := range ex.Tasks {
		rowByID[t.ID] = t
	}
	return scheduleState{
		topo: planops.TopoOrder(p), refByID: refByID, rowByID: rowByID, statusOf: archiveAwareStatus(ex),
	}
}

func (s scheduleState) eligible(id string) bool {
	if s.statusOf(id).Terminal() {
		return false
	}
	for _, d := range s.refByID[id].Task.Deps {
		if s.statusOf(d) != model.StatusDone {
			return false
		}
	}
	return true
}

// anyUnfinished/blockedReason mirror the "done vs blocked" outcome next and batch both report
// identically when no live task is eligible.
func (s scheduleState) anyUnfinished() bool {
	for _, id := range s.topo.Order {
		if !s.statusOf(id).Terminal() {
			return true
		}
	}
	return false
}

func (s scheduleState) unfinishedIDs() []string {
	var out []string
	for _, id := range s.topo.Order {
		if !s.statusOf(id).Terminal() {
			out = append(out, id)
		}
	}
	return out
}

func (s scheduleState) blockedReason() string {
	reason := "no task has all deps done — dependency stall"
	if len(s.topo.Cycle) > 0 {
		reason += " (cycle: " + strings.Join(s.topo.Cycle, ", ") + ")"
	}
	return reason
}
