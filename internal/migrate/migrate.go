// Package migrate upgrades an in-flight v1 project's plan.json + execution.json to the current
// harness shapes, in place, with an exact change report. Lossless: only additive fields
// (entity names, the schema_version stamp) are ever written; task status/commit/cost/verdicts
// and the log are never touched.
package migrate

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/execops"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// nameMaxLen bounds a synthesized task name so it stays a short label, not a second summary.
const nameMaxLen = 72

var taskIDPattern = regexp.MustCompile(`M[0-9]+\.P[0-9]+\.T[0-9]+`)
var doNotDispatchPattern = regexp.MustCompile(`(?i)do[\s-]+not[\s-]+dispatch`)

// legacyModelRenames maps a retired model id to its current replacement; empty until a v2-style
// rename actually lands (every model-enum change so far has been additive).
var legacyModelRenames = map[model.Model]model.Model{}

// Change is one field-level upgrade the migration applied (or, in dry-run, would apply).
type Change struct {
	Target string `json:"target"`
	ID     string `json:"id,omitempty"`
	Field  string `json:"field"`
	From   string `json:"from"`
	To     string `json:"to"`
	Note   string `json:"note,omitempty"`
}

// Report is the full result of a migrate-project run.
type Report struct {
	AlreadyV2 bool     `json:"already_v2"`
	DryRun    bool     `json:"dry_run"`
	Changes   []Change `json:"changes"`
	Warnings  []string `json:"warnings"`
}

// Run upgrades a parsed plan + execution pair to the current harness shapes in place and
// returns the exact change report. dryRun only records intent on the report; the in-memory
// values are upgraded either way, so a real run and its preview produce an identical change list.
func Run(p *model.Plan, ex *model.ExecState, dryRun bool) (Report, error) {
	rep := Report{DryRun: dryRun, Changes: []Change{}, Warnings: []string{}}
	migratePlan(p, &rep)
	if err := migrateExec(ex, &rep); err != nil {
		return rep, err
	}
	rep.AlreadyV2 = len(rep.Changes) == 0
	return rep, nil
}

func migratePlan(p *model.Plan, rep *Report) {
	for mi := range p.Milestones {
		m := &p.Milestones[mi]
		if strings.TrimSpace(m.Name) == "" {
			n := "Milestone " + m.ID
			rep.Changes = append(rep.Changes, Change{Target: "plan", ID: m.ID, Field: "name", From: "", To: n, Note: "milestone name backfilled from id"})
			m.Name = n
		}
		for pi := range m.Phases {
			ph := &m.Phases[pi]
			if strings.TrimSpace(ph.Name) == "" {
				n := "Phase " + ph.ID
				rep.Changes = append(rep.Changes, Change{Target: "plan", ID: ph.ID, Field: "name", From: "", To: n, Note: "phase name backfilled from id"})
				ph.Name = n
			}
			for ti := range ph.Tasks {
				t := &ph.Tasks[ti]
				if strings.TrimSpace(t.Name) == "" {
					n := nameFromSummary(t.Summary, t.ID)
					rep.Changes = append(rep.Changes, Change{Target: "plan", ID: t.ID, Field: "name", From: "", To: n, Note: "task name backfilled from summary"})
					t.Name = n
				}
				migrateModel("plan", t.ID, &t.Model, rep)
			}
		}
	}
	migrateOrchestratorOnly(p, rep)
}

// migrateOrchestratorOnly promotes the legacy "do not dispatch" hand-note convention (a
// forces_pause risk naming a task by id) into the structural orchestrator_only field.
func migrateOrchestratorOnly(p *model.Plan, rep *Report) {
	named := map[string]bool{}
	for _, r := range p.Risks {
		if !r.ForcesPause {
			continue
		}
		text := r.Risk + " " + r.Mitigation
		if !doNotDispatchPattern.MatchString(text) {
			continue
		}
		for _, id := range taskIDPattern.FindAllString(text, -1) {
			named[id] = true
		}
	}
	if len(named) == 0 {
		return
	}
	for mi := range p.Milestones {
		for pi := range p.Milestones[mi].Phases {
			for ti := range p.Milestones[mi].Phases[pi].Tasks {
				t := &p.Milestones[mi].Phases[pi].Tasks[ti]
				if named[t.ID] && !t.OrchestratorOnly {
					rep.Changes = append(rep.Changes, Change{Target: "plan", ID: t.ID, Field: "orchestrator_only", From: "false", To: "true", Note: "promoted from a forces_pause hand-note naming this task by id (design-declared do-not-dispatch)"})
					t.OrchestratorOnly = true
				}
			}
		}
	}
}

func migrateExec(ex *model.ExecState, rep *Report) error {
	before := ex.SchemaVersion
	if err := execops.MigrateSchema(ex); err != nil {
		return err
	}
	if before != ex.SchemaVersion {
		from := "absent"
		if before != 0 {
			from = fmt.Sprintf("%d", before)
		}
		rep.Changes = append(rep.Changes, Change{Target: "execution", Field: "schema_version", From: from, To: fmt.Sprintf("%d", ex.SchemaVersion), Note: "stamped current execution schema version"})
	}
	if strings.TrimSpace(ex.Name) == "" {
		n := ex.Project + " — Execution"
		rep.Changes = append(rep.Changes, Change{Target: "execution", Field: "name", From: "", To: n, Note: "execution name backfilled"})
		ex.Name = n
	}
	for i := range ex.Tasks {
		migrateModel("execution", ex.Tasks[i].ID, &ex.Tasks[i].Model, rep)
	}
	return nil
}

func migrateModel(target, id string, m *model.Model, rep *Report) {
	if renamed, ok := legacyModelRenames[*m]; ok {
		rep.Changes = append(rep.Changes, Change{Target: target, ID: id, Field: "model", From: string(*m), To: string(renamed), Note: "retired model id renamed to the current replacement"})
		*m = renamed
	}
}

// nameFromSummary derives a short task name from the summary: the first sentence, collapsed to
// a single line and capped at nameMaxLen runes on a word boundary.
func nameFromSummary(summary, fallback string) string {
	s := strings.Join(strings.Fields(summary), " ")
	if s == "" {
		return fallback
	}
	if i := strings.Index(s, ". "); i >= 0 {
		s = s[:i]
	} else {
		s = strings.TrimSuffix(s, ".")
	}
	r := []rune(s)
	if len(r) > nameMaxLen {
		cut := nameMaxLen
		for cut > 0 && r[cut] != ' ' {
			cut--
		}
		if cut == 0 {
			cut = nameMaxLen
		}
		s = strings.TrimRight(string(r[:cut]), " ") + "…"
	}
	return s
}
