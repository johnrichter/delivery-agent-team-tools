package planops

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

var (
	reMilestone = regexp.MustCompile(`^M[0-9]+$`)
	rePhase     = regexp.MustCompile(`^M[0-9]+\.P[0-9]+$`)
	reTask      = regexp.MustCompile(`^M[0-9]+\.P[0-9]+\.T[0-9]+$`)
)

// rootKeys is the closed set of plan.json top-level keys; anything else is a warning, not a
// failure — a plan authored against a newer/older schema revision should still validate.
var rootKeys = map[string]bool{
	"goal": true, "success_criteria": true, "assumptions": true, "tradeoffs": true,
	"milestones": true, "risks": true, "open_questions": true, "provenance": true,
}

type ValidationResult struct {
	OK       bool     `json:"ok"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// ValidatePlanBytes is the single deterministic plan gate: required fields, id patterns,
// closed enums, and minimum-array-length constraints, plus the cross-referential integrity a
// field-by-field walk alone can express — id uniqueness, id-hierarchy nesting, dep-reference
// integrity, dependency cycles, and tier availability (roster-sourced). An unknown root key is
// a warning, never a failure.
func ValidatePlanBytes(raw []byte) ValidationResult {
	res := ValidationResult{Errors: []string{}, Warnings: []string{}}
	addE := func(f string, a ...any) { res.Errors = append(res.Errors, fmt.Sprintf(f, a...)) }

	var keyProbe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keyProbe); err != nil {
		res.Errors = append(res.Errors, "plan is not a JSON object: "+err.Error())
		return res
	}
	for k := range keyProbe {
		if !rootKeys[k] {
			res.Warnings = append(res.Warnings, fmt.Sprintf("unknown root key '%s' (not in plan schema)", k))
		}
	}
	var p model.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		res.Errors = append(res.Errors, "plan failed to decode: "+err.Error())
		return res
	}

	if strings.TrimSpace(p.Goal) == "" {
		addE("goal: required non-empty string")
	}
	if len(p.SuccessCriteria) < 1 {
		addE("success_criteria: required array of ≥1 strings")
	}
	if len(p.Milestones) < 1 {
		addE("milestones: required array of ≥1")
		res.OK = false
		return res
	}

	taskIDs := map[string]bool{}
	seenM, seenP, seenT := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, m := range p.Milestones {
		if !reMilestone.MatchString(m.ID) {
			addE("milestone id '%s' must match M<n>", m.ID)
		}
		if seenM[m.ID] {
			addE("duplicate milestone id '%s'", m.ID)
		}
		seenM[m.ID] = true
		if strings.TrimSpace(m.Name) == "" {
			addE("milestone %s: name required", m.ID)
		}
		if len(m.Phases) < 1 {
			addE("milestone %s: phases array of ≥1 required", m.ID)
			continue
		}
		for _, ph := range m.Phases {
			if !rePhase.MatchString(ph.ID) {
				addE("phase id '%s' must match M<n>.P<n>", ph.ID)
			} else if !strings.HasPrefix(ph.ID, m.ID+".P") {
				addE("phase %s is nested under %s but its id prefix does not match", ph.ID, m.ID)
			}
			if seenP[ph.ID] {
				addE("duplicate phase id '%s'", ph.ID)
			}
			seenP[ph.ID] = true
			if strings.TrimSpace(ph.Name) == "" {
				addE("phase %s: name required", ph.ID)
			}
			if len(ph.Tasks) < 1 {
				addE("phase %s: tasks array of ≥1 required", ph.ID)
				continue
			}
			for _, t := range ph.Tasks {
				if !reTask.MatchString(t.ID) {
					addE("task id '%s' must match M<n>.P<n>.T<n>", t.ID)
					continue
				}
				if !strings.HasPrefix(t.ID, ph.ID+".T") {
					addE("task %s is nested under %s but its id prefix does not match", t.ID, ph.ID)
				}
				if seenT[t.ID] {
					addE("duplicate task id '%s'", t.ID)
				}
				seenT[t.ID] = true
				taskIDs[t.ID] = true
				if strings.TrimSpace(t.Name) == "" {
					addE("task %s: name required", t.ID)
				}
				if strings.TrimSpace(t.Summary) == "" {
					addE("task %s: summary required", t.ID)
				}
				if strings.TrimSpace(t.Deliverable) == "" {
					addE("task %s: deliverable required", t.ID)
				}
				if t.Kind != "" && !t.Kind.Known() {
					addE("task %s: deliverable_kind '%s' must be 'code' or 'docs' (omit for code)", t.ID, t.Kind)
				}
				if strings.TrimSpace(t.TestStrategy) == "" {
					addE("task %s: test_strategy required", t.ID)
				}
				if len(t.Acceptance) < 1 {
					addE("task %s: acceptance array of ≥1 required", t.ID)
				}
				if len(t.FileSurface) == 0 && t.Kind.Resolve() == model.KindCode {
					res.Warnings = append(res.Warnings, fmt.Sprintf("task %s: file_surface absent on a code task — parallel overlap checks may not be possible", t.ID))
				}
				for _, fs := range t.FileSurface {
					if strings.TrimSpace(fs.Path) == "" {
						addE("task %s: file_surface entry has an empty path", t.ID)
					}
					if fs.Kind != "" && !fs.Kind.Known() {
						addE("task %s: file_surface entry '%s' has kind '%s' — must be 'file', 'glob', or 'dir' (omit for file)", t.ID, fs.Path, fs.Kind)
					}
				}
				for _, d := range t.Deps {
					if !reTask.MatchString(d) {
						addE("task %s: dep '%s' is not a task id", t.ID, d)
					}
				}
			}
		}
	}
	// dep-reference integrity + self-dep (cycles handled below)
	for _, r := range WalkTasks(p) {
		for _, d := range r.Task.Deps {
			if d == r.Task.ID {
				addE("task %s: depends on itself", r.Task.ID)
			} else if reTask.MatchString(d) && !taskIDs[d] {
				addE("task %s: dep '%s' references a non-existent task", r.Task.ID, d)
			}
		}
	}
	if cyc := TopoOrder(p).Cycle; len(cyc) > 0 {
		addE("dependency cycle / unschedulable tasks: %s", strings.Join(cyc, ", "))
	}
	for _, i := range CheckTiers(p).Issues {
		addE("tier: %s: %s", i.ID, i.Issue)
	}

	res.OK = len(res.Errors) == 0
	return res
}
