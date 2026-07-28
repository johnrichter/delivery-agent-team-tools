package planops

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/johnrichter/claude-shared-tooling/go/roster"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

type TierIssue struct {
	ID    string `json:"id"`
	Issue string `json:"issue"`
}

type TierResult struct {
	OK     bool        `json:"ok"`
	Issues []TierIssue `json:"issues"`
}

// modelSelectable reports whether id is a valid plan-pinnable model: either the roster's
// selectable=="new-work" projection, or a declared dispatch sentinel (e.g. "inherit"), which
// the roster reports via SentinelError rather than a Selectable value.
func modelSelectable(id string) bool {
	sel, err := roster.Selectable(id)
	if err != nil {
		var sentinelErr *roster.SentinelError
		return errors.As(err, &sentinelErr)
	}
	return sel == roster.SelectableNewWork
}

func effortNames(efforts []roster.Effort) string {
	names := make([]string, len(efforts))
	for i, e := range efforts {
		names[i] = string(e)
	}
	return strings.Join(names, ", ")
}

// CheckTiers validates each task's (model, effort) combo against the live model roster: model
// validity is modelSelectable; effort validity is the model's roster-declared effort_available
// list (an effort-exempt model or dispatch sentinel accepts every level).
func CheckTiers(p model.Plan) TierResult {
	issues := []TierIssue{}
	for _, r := range WalkTasks(p) {
		t := r.Task
		if !modelSelectable(string(t.Model)) {
			issues = append(issues, TierIssue{t.ID, fmt.Sprintf("model '%s' not in the selectable set", t.Model)})
		}
		if !t.Effort.Known() {
			issues = append(issues, TierIssue{t.ID, fmt.Sprintf("effort '%s' not a valid tier", t.Effort)})
			continue
		}
		avail, err := roster.EffortAvailable(string(t.Model))
		if err != nil {
			continue
		}
		if !slices.Contains(avail, roster.Effort(t.Effort)) {
			issues = append(issues, TierIssue{t.ID, fmt.Sprintf("effort '%s' is not available for model '%s' (roster allows: %s)", t.Effort, t.Model, effortNames(avail))})
		}
	}
	return TierResult{OK: len(issues) == 0, Issues: issues}
}
