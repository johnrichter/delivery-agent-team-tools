package execops

import (
	"fmt"
	"math"
	"strings"

	shstate "github.com/johnrichter/claude-shared-tooling/go/state"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// RecordFields carries the per-task transition; a nil field means "leave unchanged".
type RecordFields struct {
	Status    *model.Status
	Test      *string
	Review    *string
	Commit    *string
	Cost      *float64
	TokensOut *int64
	Usage     *model.Usage
	Note      *string
	RunID     *string
	Override  *string
}

func round4(f float64) float64 { return math.Round(f*1e4) / 1e4 }

// RecomputeTotals rebuilds the cumulative run-config aggregates from the per-task rows — never
// hand-summed — over both live Tasks and Archived tombstones, so an archive move never changes
// the whole-project total. Exported so the archive op (which also moves rows between Tasks and
// Archived) shares this one derivation.
func RecomputeTotals(ex *model.ExecState) {
	var usd float64
	var tok int64
	var u model.Usage
	have := false
	fold := func(cost float64, tokens int64, usage *model.Usage) {
		usd += cost
		tok += tokens
		if usage != nil {
			u.Add(*usage)
			have = true
		}
	}
	for _, x := range ex.Tasks {
		fold(x.CostUSD, x.TokensOut, x.Usage)
	}
	for _, x := range ex.Archived {
		fold(x.CostUSD, x.TokensOut, x.Usage)
	}
	ex.RunConfig.SpentUSD = round4(usd)
	ex.RunConfig.TokensOut = tok
	if have {
		ex.RunConfig.Usage = &u
	} else {
		ex.RunConfig.Usage = nil
	}
}

// Record applies a per-task transition, recomputes cumulative spend from per-task costs, bumps
// timestamps, and appends a log line. A write is refused (ex left untouched) whenever the
// RESULTING row would read status=done with no commit — the state package's own rung-1
// invariant (state.RecordTask), applied here against a narrow projection of the row so the
// engine's richer per-task fields (test/review/usage/notes/...) never bypass it.
func Record(ex *model.ExecState, taskID string, f RecordFields, at string) error {
	idx := -1
	for i := range ex.Tasks {
		if ex.Tasks[i].ID == taskID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("record: task %q not in execution state", taskID)
	}
	t := &ex.Tasks[idx]
	if f.Status != nil && !f.Status.Known() {
		return fmt.Errorf("record: invalid status %q", *f.Status)
	}

	guard := shstate.Task{ID: t.ID, Status: shstate.Status(t.Status), CommitSHA: t.Commit, CostUSD: t.CostUSD}
	var guardStatus *shstate.Status
	if f.Status != nil {
		s := shstate.Status(*f.Status)
		guardStatus = &s
	}
	if err := shstate.RecordTask(&guard, guardStatus, f.Commit, f.Cost); err != nil {
		return fmt.Errorf("record: task %q: status=done requires field %q — none supplied and none already recorded", taskID, "commit")
	}

	if f.Status != nil {
		t.Status = *f.Status
	}
	if f.Test != nil {
		t.Test = *f.Test
	}
	if f.Review != nil {
		t.Review = *f.Review
	}
	if f.Commit != nil {
		t.Commit = *f.Commit
	}
	if f.Cost != nil {
		t.CostUSD = *f.Cost
	}
	if f.TokensOut != nil {
		t.TokensOut = *f.TokensOut
	}
	if f.Usage != nil {
		t.Usage = f.Usage
	}
	if f.Note != nil {
		t.Notes = *f.Note
	}
	if f.RunID != nil {
		ex.RunConfig.LastRunID = *f.RunID
	}
	if f.Override != nil {
		ex.RunConfig.Override = *f.Override
	}
	t.Updated = at
	RecomputeTotals(ex)
	ex.Updated = at

	parts := []string{}
	if f.Status != nil {
		parts = append(parts, "→ "+string(*f.Status))
	}
	if t.Test != "" {
		parts = append(parts, "test "+t.Test)
	}
	if t.Review != "" {
		parts = append(parts, "review "+t.Review)
	}
	if t.Commit != "" {
		parts = append(parts, t.Commit)
	}
	if f.Cost != nil {
		parts = append(parts, fmt.Sprintf("$%.2f", *f.Cost))
	}
	if f.TokensOut != nil {
		parts = append(parts, fmt.Sprintf("%d out-tok", *f.TokensOut))
	}
	ex.Log = append(ex.Log, strings.TrimSpace(at+" "+taskID+" "+strings.Join(parts, " ")))
	return nil
}

// LogNote appends a plan-level entry to the execution log and bumps `updated`.
func LogNote(ex *model.ExecState, note, at string) {
	ex.Log = append(ex.Log, strings.TrimSpace(at+" NOTE "+note))
	ex.Updated = at
}
