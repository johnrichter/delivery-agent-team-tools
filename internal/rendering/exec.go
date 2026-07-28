package rendering

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
	"github.com/johnrichter/delivery-agent-team-tools/internal/schedule"
)

// progressTask is one row of the execution.md progress table: the plan's task name merged with
// its live execution.json row.
type progressTask struct {
	ID      string
	Name    string
	Status  string
	Emoji   string
	Kind    string
	Tier    string
	Test    string
	Review  string
	Commit  string
	OutTok  string
	Updated string
	Notes   string
	Struck  bool
}

type progressPhase struct {
	ID    string
	Name  string
	Tasks []progressTask
}

type progressMilestone struct {
	ID     string
	Name   string
	Phases []progressPhase
}

type resumeView struct {
	Done             bool
	OrchestratorOnly bool
	TaskID           string
	TaskName         string
	Reason           string
}

// trueUsageView is the transcript-derived whole-session token total, pre-formatted for the
// mirror (comma-grouped, cache classes combined).
type trueUsageView struct {
	Input  string
	Output string
	Cache  string
	Total  string
	Turns  int64
}

// accountingView is the transcript-derived whole-session true-cost total, pre-formatted.
type accountingView struct {
	CostUSD    string
	ModelCount int
	Unpriced   []string
}

type runConfigView struct {
	PauseMode  string
	Budget     string
	Spent      string
	TokensOut  string
	TrueUsage  *trueUsageView
	Accounting *accountingView
	Rates      string
	Override   string
	LastRunID  string
}

type execDoc struct {
	Ex        model.ExecState
	RunConfig runConfigView
	Progress  []progressMilestone
	Resume    resumeView
}

// commaInt formats an integer with thousands separators for the human mirror (e.g. 1234567 ->
// "1,234,567"). Token totals are large; grouping keeps the run-config and table readable.
func commaInt(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	out := b.String()
	if neg {
		return "-" + out
	}
	return out
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// buildRunConfig turns the persisted run config into the mirror's pre-formatted view: dollar
// amounts, comma-grouped token counts, and the fallback strings ("none"/"—") the template
// itself never has to compute.
func buildRunConfig(rc model.RunConfig) runConfigView {
	v := runConfigView{
		PauseMode: rc.PauseMode,
		Budget:    rc.Budget,
		Spent:     fmt.Sprintf("$%.2f", rc.SpentUSD),
		TokensOut: commaInt(rc.TokensOut),
		Rates:     rc.Rates,
		Override:  rc.Override,
		LastRunID: orDash(rc.LastRunID),
	}
	if v.Override == "" {
		v.Override = "none"
	}
	if u := rc.TrueUsage; u != nil {
		v.TrueUsage = &trueUsageView{
			Input:  commaInt(u.InputTokens),
			Output: commaInt(u.OutputTokens),
			Cache:  commaInt(u.CacheCreationTokens + u.CacheReadTokens),
			Total:  commaInt(u.TotalTokens),
			Turns:  u.Turns,
		}
	}
	if a := rc.Accounting; a != nil {
		v.Accounting = &accountingView{
			CostUSD:    fmt.Sprintf("$%.4f", a.CostUSD),
			ModelCount: len(a.Models),
			Unpriced:   a.Unpriced,
		}
	}
	return v
}

func buildProgress(ex model.ExecState, p model.Plan) []progressMilestone {
	rowByID := map[string]model.ExecTask{}
	for _, t := range ex.Tasks {
		rowByID[t.ID] = t
	}
	var out []progressMilestone
	for _, m := range p.Milestones {
		pm := progressMilestone{ID: m.ID, Name: m.Name}
		for _, ph := range m.Phases {
			pp := progressPhase{ID: ph.ID, Name: ph.Name}
			for _, t := range ph.Tasks {
				row, ok := rowByID[t.ID]
				if !ok {
					row = model.ExecTask{ID: t.ID, Status: model.StatusNotStarted}
				}
				outTok := "—"
				if row.TokensOut > 0 {
					outTok = commaInt(row.TokensOut)
				}
				pp.Tasks = append(pp.Tasks, progressTask{
					ID: row.ID, Name: t.Name, Status: string(row.Status), Emoji: row.Status.Emoji(),
					Kind: string(row.Kind.Resolve()), Tier: tier(string(row.Model), string(row.Effort)),
					Test: orDash(row.Test), Review: orDash(row.Review), Commit: orDash(row.Commit),
					OutTok: outTok, Updated: orDash(row.Updated), Notes: orDash(row.Notes),
					Struck: row.Status == model.StatusSuperseded,
				})
			}
			pm.Phases = append(pm.Phases, pp)
		}
		out = append(out, pm)
	}
	return out
}

func buildResume(ex model.ExecState, p model.Plan) resumeView {
	switch np := schedule.Next(ex, p); {
	case np.Done:
		return resumeView{Done: true}
	case np.Task != nil:
		return resumeView{TaskID: np.Task.ID, TaskName: np.Task.Name}
	case np.OrchestratorOnly != nil:
		return resumeView{OrchestratorOnly: true, TaskID: np.OrchestratorOnly.ID, TaskName: np.OrchestratorOnly.Name}
	default:
		reason := np.Reason
		if reason == "" {
			reason = "no eligible task"
		}
		return resumeView{Reason: reason}
	}
}

var execTemplate = mustParse("execution.md", `---
name: {{.Ex.Name}}
description: {{printf "Live execution-state mirror for the %s project — generated from execution.json (canonical); per-task status, verdicts, commit SHAs, cost, and resume pointer. Do not hand-edit." .Ex.Project | printf "%q"}}
id: project:{{.Ex.Project}}:execution
tags:
  - type:project
  - topic:{{dflt "tooling" .Ex.Topic}}
  - status:complete
links: []
updated: {{.Ex.Updated}}
---

# {{.Ex.Name}}

> Generated mirror of `+"`execution.json`"+` (canonical). Do not hand-edit — re-render with `+"`build-helpers render-exec`"+`.

- **Design:** `+"`design.md`"+` (proposal/context; source of truth)
- **Plan:** `+"`plan.json`"+` (immutable build spec) · readable mirror `+"`plan.md`"+`
- **Derived from:** plan.json @ {{dflt "—" .Ex.Provenance.PlanUpdated}} · design.md @ {{dflt "—" .Ex.Provenance.DesignUpdated}}
- **Goal:** {{.Ex.Goal | cell}}
- **Started:** {{.Ex.Started}} · **Updated:** {{.Ex.Updated}}

## Run config
- pause mode: `+"`{{.RunConfig.PauseMode}}`"+`
- budget: {{.RunConfig.Budget}}
- spent: {{.RunConfig.Spent}}   <!-- cumulative OUTPUT-cost (lower bound; input tokens not priced) -->
- output tokens (measured): {{.RunConfig.TokensOut}}   <!-- engine-measured per-task output; same basis as spent -->
{{with .RunConfig.TrueUsage}}- true tokens (transcript): {{.Input}} in + {{.Output}} out + {{.Cache}} cache = **{{.Total}} total** over {{.Turns}} turns   <!-- whole session incl. subagents; internal transcript format, best-effort -->
{{end -}}
{{with .RunConfig.Accounting}}- true cost (transcript): **{{.CostUSD}}** across {{.ModelCount}} models   <!-- whole session incl. subagents; input+cache+output priced per anthropic-specifications.json -->
{{with .Unpriced}}- ⚠️ unpriced models (no rate matched, excluded from cost): {{join ", " .}}
{{end}}{{end -}}
- rates: {{.RunConfig.Rates}}
- override: {{.RunConfig.Override}}
- last runId: {{.RunConfig.LastRunID}}   <!-- SAME-SESSION fast-resume only; discard on a fresh session -->

## Resume pointer
{{if .Resume.Done}}**All tasks ✅ — build complete.**
{{else if .Resume.OrchestratorOnly}}**Orchestrator-only, run inline (refused for dispatch) →** {{.Resume.TaskID}} — {{.Resume.TaskName | cell}}
{{else if .Resume.TaskID}}**Resume here →** {{.Resume.TaskID}} — {{.Resume.TaskName | cell}}
{{else}}**Blocked →** {{.Resume.Reason | cell}}
{{end}}
## Progress
{{range .Progress}}
### {{.ID}} — {{.Name | cell}}
{{range .Phases}}
#### {{.ID}} — {{.Name | cell}}
| Task | Status | Kind | Model/Effort | Test | Review | Commit | Out-tok | Updated | Notes |
| --- | :--: | :--: | --- | :--: | :--: | --- | --: | --- | --- |
{{range .Tasks}}| {{if .Struck}}~~{{.ID}}~~{{else}}{{.ID}}{{end}} — {{.Name | cell}} | {{.Emoji}} | {{.Kind}} | {{.Tier}} | {{.Test}} | {{.Review}} | {{.Commit}} | {{.OutTok}} | {{.Updated}} | {{.Notes | cell}} |
{{end}}{{end}}{{end}}

## Log
{{if not .Ex.Log}}- (none yet)
{{end}}{{range .Ex.Log}}- {{.}}
{{end}}
`)

// RenderExecution renders the human-readable execution.md mirror from canonical state + the
// plan (for milestone/phase names, order, and the resume pointer).
func RenderExecution(ex model.ExecState, p model.Plan) (string, error) {
	doc := execDoc{Ex: ex, RunConfig: buildRunConfig(ex.RunConfig), Progress: buildProgress(ex, p), Resume: buildResume(ex, p)}
	return renderMirror(execTemplate, doc)
}
