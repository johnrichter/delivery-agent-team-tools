package execops

import (
	"fmt"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
	"github.com/johnrichter/delivery-agent-team-tools/internal/planops"
)

// Reconcile applies a plan diff to execution state, preserving completed work: carried rows
// keep their status+commit, changed/added rows reset to not-started, and removed rows become
// superseded (kept for history). Archived task ids are excluded from the rebuild — an
// unchanged, already-archived milestone the new plan still declares is never re-admitted as a
// fresh not-started live row.
func Reconcile(ex *model.ExecState, oldP, newP model.Plan, designUpdated, planUpdated, at string) {
	d := planops.Diff(oldP, newP)
	changed := map[string]bool{}
	for _, c := range d.Changed {
		changed[c.ID] = true
	}
	added := map[string]bool{}
	for _, a := range d.Added {
		added[a.ID] = true
	}
	oldByID := map[string]model.ExecTask{}
	for _, t := range ex.Tasks {
		oldByID[t.ID] = t
	}
	archived := map[string]bool{}
	for _, x := range ex.Archived {
		archived[x.ID] = true
	}

	rows := []model.ExecTask{}
	for _, r := range planops.WalkTasks(newP) {
		spec := r.Task
		if archived[spec.ID] {
			continue
		}
		prior, hadPrior := oldByID[spec.ID]
		switch {
		case added[spec.ID] || !hadPrior:
			rows = append(rows, model.ExecTask{ID: spec.ID, Summary: spec.Summary, Kind: spec.Kind.Resolve(), Model: spec.Model, Effort: spec.Effort, Status: model.StatusNotStarted, Updated: at, Notes: "added by design change"})
		case changed[spec.ID]:
			note := fmt.Sprintf("changed by design @ %s", at)
			if prior.Status == model.StatusDone {
				commit := prior.Commit
				if commit == "" {
					commit = "?"
				}
				note += fmt.Sprintf(" (was ✅ — rebuilt; prior commit %s may be orphaned)", commit)
			}
			rows = append(rows, model.ExecTask{ID: spec.ID, Summary: spec.Summary, Kind: spec.Kind.Resolve(), Model: spec.Model, Effort: spec.Effort, Status: model.StatusNotStarted, Updated: at, Notes: note})
		default: // carried: keep status + commit; refresh tier/summary/kind display
			prior.Summary = spec.Summary
			prior.Kind = spec.Kind.Resolve()
			prior.Model = spec.Model
			prior.Effort = spec.Effort
			rows = append(rows, prior)
		}
	}
	for _, rem := range d.Removed {
		prior, ok := oldByID[rem.ID]
		if !ok {
			continue
		}
		prior.Status = model.StatusSuperseded
		prior.Updated = at
		prior.Notes = fmt.Sprintf("superseded — removed by design @ %s", at)
		if oldByID[rem.ID].Status == model.StatusDone {
			prior.Notes += " (built work may be orphaned — operator review)"
		}
		rows = append(rows, prior)
	}
	ex.Tasks = rows
	RecomputeTotals(ex)
	if designUpdated != "" {
		ex.Provenance.DesignUpdated = designUpdated
	}
	if planUpdated != "" {
		ex.Provenance.PlanUpdated = planUpdated
	}
	ex.Provenance.DerivedAt = at
	ex.Updated = at
	ex.Log = append(ex.Log, fmt.Sprintf("%s reconcile — carried %d, changed %d, added %d, superseded %d", at, len(d.Carried), len(d.Changed), len(d.Added), len(d.Removed)))
}
