// Package feedback is the feedback.json register: add, ranked list, and the criticality-
// threshold gate the magistrate consumes before a local-delta-replan.
package feedback

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/johnrichter/claude-shared-tooling/go/gate"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// ValidScore reports whether a 1-5 impact/urgency score is in range.
func ValidScore(v int) bool { return v >= 1 && v <= 5 }

// Criticality derives the ranking score the threshold gate acts on: impact × urgency — a risk-
// matrix convention that requires both axes high to rank as truly critical.
func Criticality(impact, urgency int) int { return impact * urgency }

// Input is the caller-supplied subset of a FeedbackEntry's fields. ID and Criticality are
// deliberately absent — Add is the only place either is produced.
type Input struct {
	Title            string
	SourceTaskID     string
	Feedback         string
	ProposedSolution string
	WhyItMatters     string
	Impact           int
	Urgency          int
}

// Add validates in and appends one new entry to reg, returning the updated register. The new
// entry's id is FB<n>, n = len(reg.Entries)+1 — stable and monotonic within a project.
func Add(reg model.FeedbackRegister, in Input, at string) (model.FeedbackRegister, error) {
	if strings.TrimSpace(in.Title) == "" {
		return reg, fmt.Errorf("feedback add: title is required")
	}
	if strings.TrimSpace(in.Feedback) == "" {
		return reg, fmt.Errorf("feedback add: feedback is required")
	}
	if !ValidScore(in.Impact) {
		return reg, fmt.Errorf("feedback add: impact %d out of range (must be 1-5)", in.Impact)
	}
	if !ValidScore(in.Urgency) {
		return reg, fmt.Errorf("feedback add: urgency %d out of range (must be 1-5)", in.Urgency)
	}
	if reg.Schema == "" {
		reg.Schema = model.FeedbackSchema
	}
	entry := model.FeedbackEntry{
		ID: fmt.Sprintf("FB%d", len(reg.Entries)+1), Title: in.Title, SourceTaskID: in.SourceTaskID,
		Feedback: in.Feedback, ProposedSolution: in.ProposedSolution, WhyItMatters: in.WhyItMatters,
		Impact: in.Impact, Urgency: in.Urgency, Criticality: Criticality(in.Impact, in.Urgency), Added: at,
	}
	out := reg
	out.Entries = append(append([]model.FeedbackEntry{}, reg.Entries...), entry)
	return out, nil
}

// Filter is the composable predicate `feedback list` applies; every non-zero field narrows the
// result and all supplied fields AND together.
type Filter struct {
	SourceTaskID string
	MinImpact    int
	MinUrgency   int
}

func (f Filter) matches(e model.FeedbackEntry) bool {
	if f.SourceTaskID != "" && e.SourceTaskID != f.SourceTaskID {
		return false
	}
	return e.Impact >= f.MinImpact && e.Urgency >= f.MinUrgency
}

// List returns reg's entries matching f, ranked by criticality descending, ties broken by id
// ascending (ids are monotonic-by-add-order) so ranking is deterministic regardless of storage
// order.
func List(reg model.FeedbackRegister, f Filter) []model.FeedbackEntry {
	var out []model.FeedbackEntry
	for _, e := range reg.Entries {
		if f.matches(e) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Criticality != out[j].Criticality {
			return out[i].Criticality > out[j].Criticality
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Render renders feedback.md, the faithful mirror of feedback.json.
func Render(reg model.FeedbackRegister) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s); b.WriteByte('\n') }
	w("# Feedback register")
	w("")
	if len(reg.Entries) == 0 {
		w("_(no feedback entries yet)_")
		return b.String()
	}
	w("| ID | Title | Source task | Impact | Urgency | Criticality |")
	w("| --- | --- | --- | :--: | :--: | :--: |")
	for _, e := range reg.Entries {
		src := e.SourceTaskID
		if src == "" {
			src = "—"
		}
		w(fmt.Sprintf("| %s | %s | %s | %d | %d | %d |", e.ID, sanitizeCell(e.Title), src, e.Impact, e.Urgency, e.Criticality))
	}
	w("")
	for _, e := range reg.Entries {
		w("## " + e.ID + " — " + sanitizeLine(e.Title))
		w("")
		w("- feedback: " + sanitizeLine(e.Feedback))
		if e.ProposedSolution != "" {
			w("- proposed solution: " + sanitizeLine(e.ProposedSolution))
		}
		if e.WhyItMatters != "" {
			w("- why it matters: " + sanitizeLine(e.WhyItMatters))
		}
		w("")
	}
	return b.String()
}

func sanitizeCell(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "|", `\|`))
}
func sanitizeLine(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "\n", " ")) }

// Gate is the deterministic threshold partition of the ranked register: AmendNow (criticality
// >= threshold) is ranked reconcile-exec amendment input; Deferred (< threshold) is realized as
// feedback-review tasks. Total, lossless, exactly-once: every register entry lands in precisely
// one bucket.
type Gate struct {
	Threshold int                   `json:"threshold"`
	AmendNow  []model.FeedbackEntry `json:"amend_now"`
	Deferred  []model.FeedbackEntry `json:"deferred"`
}

// PartitionByThreshold ranks the full register and splits it at threshold via gate.Partition —
// the shared generic threshold-partition primitive, applied here to feedback entries.
func PartitionByThreshold(reg model.FeedbackRegister, threshold int) Gate {
	ranked := List(reg, Filter{})
	deferred, amendNow := gate.Partition(ranked, threshold, func(e model.FeedbackEntry) int { return e.Criticality })
	return Gate{Threshold: threshold, AmendNow: nonNil(amendNow), Deferred: nonNil(deferred)}
}

func nonNil(s []model.FeedbackEntry) []model.FeedbackEntry {
	if s == nil {
		return []model.FeedbackEntry{}
	}
	return s
}

// ---- standing feedback-review milestone ----

const (
	ReviewMilestoneID   = "M999"
	ReviewMilestoneName = "Feedback review"
	ReviewPhaseID       = "M999.P1"
	ReviewPhaseName     = "Deferred feedback"
)

func reviewTaskNum(taskID string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(taskID, ReviewPhaseID+".T"))
	return n
}

func reviewTaskID(entryID string) string {
	n := strings.TrimPrefix(entryID, "FB")
	if n == "" || n == entryID {
		n = "0"
	}
	return ReviewPhaseID + ".T" + n
}

// ReviewTask converts one sub-threshold entry into a schema-valid Task the operator or
// magistrate refines when the deferred item is picked up.
func ReviewTask(e model.FeedbackEntry) model.Task {
	deliverable := strings.TrimSpace(e.ProposedSolution)
	if deliverable == "" {
		deliverable = "Triage and address the deferred feedback: " + e.Feedback
	}
	accept := strings.TrimSpace(e.WhyItMatters)
	if accept == "" {
		accept = "The deferred feedback is resolved or explicitly re-deferred with a recorded rationale."
	}
	return model.Task{
		ID: reviewTaskID(e.ID), Name: e.Title, Summary: "Deferred feedback (" + e.ID + "): " + e.Feedback,
		Deliverable: deliverable, Model: model.ModelInherit, Effort: model.EffortMedium,
		TestStrategy: "Reviewer confirms the deferred feedback is addressed or explicitly re-deferred with a recorded rationale.",
		Acceptance:   []string{accept},
	}
}

// BuildReviewMilestone assembles the standing feedback-review milestone from the deferred
// entries, tasks ordered by id ascending. Returns false when deferred is empty — an empty
// milestone/phase is schema-invalid, so callers must omit it entirely.
func BuildReviewMilestone(deferred []model.FeedbackEntry) (model.Milestone, bool) {
	if len(deferred) == 0 {
		return model.Milestone{}, false
	}
	tasks := make([]model.Task, 0, len(deferred))
	for _, e := range deferred {
		tasks = append(tasks, ReviewTask(e))
	}
	sort.SliceStable(tasks, func(i, j int) bool { return reviewTaskNum(tasks[i].ID) < reviewTaskNum(tasks[j].ID) })
	return model.Milestone{ID: ReviewMilestoneID, Name: ReviewMilestoneName, Phases: []model.Phase{{ID: ReviewPhaseID, Name: ReviewPhaseName, Tasks: tasks}}}, true
}

// ApplyReview returns p with its feedback-review milestone regenerated wholesale from the
// deferred entries — the new-plan input to reconcile-exec for the sub-threshold side.
func ApplyReview(p model.Plan, deferred []model.FeedbackEntry) model.Plan {
	out := p
	kept := make([]model.Milestone, 0, len(p.Milestones)+1)
	for _, m := range p.Milestones {
		if m.ID != ReviewMilestoneID {
			kept = append(kept, m)
		}
	}
	if m, ok := BuildReviewMilestone(deferred); ok {
		kept = append(kept, m)
	}
	out.Milestones = kept
	return out
}

// GateResult is the full gate output the CLI emits and the magistrate consumes.
type GateResult struct {
	Threshold int                   `json:"threshold"`
	AmendNow  []model.FeedbackEntry `json:"amend_now"`
	Deferred  []model.FeedbackEntry `json:"deferred"`
	Plan      model.Plan            `json:"plan"`
}

// GatePlan is the single entry point: partition reg by threshold, emit the ranked amend-now
// set, and apply the deferred set to p as the feedback-review milestone.
func GatePlan(p model.Plan, reg model.FeedbackRegister, threshold int) GateResult {
	g := PartitionByThreshold(reg, threshold)
	return GateResult{Threshold: threshold, AmendNow: g.AmendNow, Deferred: g.Deferred, Plan: ApplyReview(p, g.Deferred)}
}
