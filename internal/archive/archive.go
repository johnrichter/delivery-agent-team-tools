// Package archive is the explicit, operator-invoked archive op: it moves every task under each
// named, wholly-terminal milestone out of the live plan/execution docs into the preserved
// archive doc.
package archive

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/execops"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// Options is the explicit, operator-supplied request: which milestone ids to move.
type Options struct {
	MilestoneIDs []string
}

// Outcome is what the archive op replaces plan/execution/archive with, plus which requested
// ids actually moved (Archived) vs. were already archived (Skipped — the idempotent no-op case).
type Outcome struct {
	Plan     model.Plan
	Exec     model.ExecState
	Archive  model.ArchiveDoc
	Archived []string
	Skipped  []string
}

func findMilestoneIndex(p model.Plan, id string) int {
	for i, m := range p.Milestones {
		if m.ID == id {
			return i
		}
	}
	return -1
}

// Run moves every task under each named live milestone into the preserved archive doc.
// Preconditions, refused with no partial result on violation: every task under a named
// milestone must be terminal (done|superseded); re-naming an id already archived is a no-op
// (idempotent); naming an id that is neither live nor already archived is a hard error.
func Run(p model.Plan, ex model.ExecState, existing model.ArchiveDoc, opt Options, at string) (Outcome, error) {
	if len(opt.MilestoneIDs) == 0 {
		return Outcome{}, fmt.Errorf("archive: at least one --milestone id is required (explicit-only; no implicit 'archive everything done')")
	}

	alreadyArchived := map[string]bool{}
	for _, m := range existing.Milestones {
		alreadyArchived[m.ID] = true
	}
	rowByID := map[string]model.ExecTask{}
	for _, t := range ex.Tasks {
		rowByID[t.ID] = t
	}
	alreadyTombstoned := map[string]model.Status{}
	for _, x := range ex.Archived {
		alreadyTombstoned[x.ID] = x.Status
	}

	var toArchive []model.Milestone
	var skipped []string
	requested := map[string]bool{}
	for _, id := range opt.MilestoneIDs {
		if requested[id] {
			continue
		}
		requested[id] = true
		switch mi := findMilestoneIndex(p, id); {
		case mi >= 0:
			toArchive = append(toArchive, p.Milestones[mi])
		case alreadyArchived[id]:
			skipped = append(skipped, id)
		default:
			return Outcome{}, fmt.Errorf("archive: milestone %q is not in the live plan and not already archived", id)
		}
	}

	var blocking []string
	for _, m := range toArchive {
		for _, ph := range m.Phases {
			for _, t := range ph.Tasks {
				st := model.StatusNotStarted
				if row, ok := rowByID[t.ID]; ok {
					st = row.Status
				} else if ts, ok := alreadyTombstoned[t.ID]; ok {
					st = ts
				}
				if !st.Terminal() {
					blocking = append(blocking, fmt.Sprintf("%s (%s)", t.ID, st))
				}
			}
		}
	}
	if len(blocking) > 0 {
		return Outcome{}, fmt.Errorf("archive: refused — milestone not wholly terminal, non-terminal task(s): %s", strings.Join(blocking, ", "))
	}

	if len(toArchive) == 0 {
		return Outcome{Plan: p, Exec: ex, Archive: existing, Skipped: skipped}, nil
	}

	archiveIDs := map[string]bool{}
	taskIDs := map[string]bool{}
	var archivedIDs []string
	var newGroups []model.ArchivedMilestone
	var newTombstones []model.Tombstone
	for _, m := range toArchive {
		archiveIDs[m.ID] = true
		archivedIDs = append(archivedIDs, m.ID)
		groupAlreadyStored := alreadyArchived[m.ID]
		var phases []model.ArchivedPhase
		for _, ph := range m.Phases {
			var tasks []model.ArchivedTask
			for _, t := range ph.Tasks {
				taskIDs[t.ID] = true
				row := rowByID[t.ID]
				if !groupAlreadyStored {
					tasks = append(tasks, model.ArchivedTask{
						Summary: t.Summary, Deliverable: t.Deliverable, Kind: t.Kind.Resolve(),
						Model: t.Model, Effort: t.Effort, Thinking: t.Thinking, TestStrategy: t.TestStrategy,
						Deps: t.Deps, Acceptance: t.Acceptance, FileSurface: t.FileSurface,
						ID: t.ID, Status: row.Status, Test: row.Test, Review: row.Review, Commit: row.Commit,
						CostUSD: row.CostUSD, TokensOut: row.TokensOut, Usage: row.Usage, Updated: row.Updated, Notes: row.Notes,
					})
				}
				if _, tombExists := alreadyTombstoned[t.ID]; !tombExists {
					newTombstones = append(newTombstones, model.Tombstone{
						ID: t.ID, Summary: row.Summary, Status: row.Status, Commit: row.Commit,
						CostUSD: row.CostUSD, TokensOut: row.TokensOut, Usage: row.Usage,
					})
				}
			}
			if !groupAlreadyStored {
				phases = append(phases, model.ArchivedPhase{ID: ph.ID, Name: ph.Name, Tasks: tasks})
			}
		}
		if !groupAlreadyStored {
			newGroups = append(newGroups, model.ArchivedMilestone{ID: m.ID, Name: m.Name, Phases: phases})
		}
	}

	newPlan := p
	newPlan.Milestones = nil
	for _, m := range p.Milestones {
		if !archiveIDs[m.ID] {
			newPlan.Milestones = append(newPlan.Milestones, m)
		}
	}

	newEx := ex
	newEx.Tasks = nil
	for _, t := range ex.Tasks {
		if !taskIDs[t.ID] {
			newEx.Tasks = append(newEx.Tasks, t)
		}
	}
	newEx.Archived = append(append([]model.Tombstone{}, ex.Archived...), newTombstones...)
	newEx.Updated = at
	execops.RecomputeTotals(&newEx)
	newEx.Log = append(append([]string{}, ex.Log...), fmt.Sprintf("%s archive — moved %s to archive.json (%d task(s)); live docs shrink to active+pending",
		at, strings.Join(archivedIDs, ", "), len(newTombstones)))

	newArchive := model.ArchiveDoc{
		Schema:     cmp.Or(existing.Schema, model.ArchiveSchema),
		Milestones: append(append([]model.ArchivedMilestone{}, existing.Milestones...), newGroups...),
	}

	return Outcome{Plan: newPlan, Exec: newEx, Archive: newArchive, Archived: archivedIDs, Skipped: skipped}, nil
}
