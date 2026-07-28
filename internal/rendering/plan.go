package rendering

import "github.com/johnrichter/delivery-agent-team-tools/internal/model"

// PlanMeta supplies the frontmatter the workspace requires on the plan.md mirror. When Slug is
// empty, the frontmatter block is omitted (handy for ad-hoc viewing).
type PlanMeta struct {
	Slug    string
	Topic   string
	Updated string
}

type planDoc struct {
	Plan model.Plan
	Meta PlanMeta
}

var planTemplate = mustParse("plan.md", `
{{- with .Meta}}{{if .Slug}}---
name: {{.Slug}} — Plan
description: {{printf "Build plan mirror for the %s project — generated from plan.json (canonical, immutable spec); milestones → phases → tasks with per-task tier, deps, acceptance, and test strategy. Do not hand-edit." .Slug | printf "%q"}}
id: project:{{.Slug}}:plan
tags:
  - type:project
  - topic:{{dflt "tooling" .Topic}}
  - status:complete
links: []
updated: {{dflt "(unset)" .Updated}}
---

{{end}}{{end -}}
# Build plan — {{dflt "(no goal)" .Plan.Goal | line}}

{{with .Plan.SuccessCriteria}}## Success criteria
{{range .}}- {{. | line}}
{{end}}
{{end -}}
{{with .Plan.Assumptions}}## Assumptions
{{range .}}- {{. | line}}
{{end}}
{{end -}}
{{with .Plan.Tradeoffs}}## Key tradeoffs
{{range .}}- **{{.Decision | line}}**{{if .Options}} (options: {{range $i, $o := .Options}}{{if $i}} / {{end}}{{$o | line}}{{end}}){{end}} → **{{.Recommendation | line}}** — {{.Why | line}}
{{end}}
{{end -}}
{{range .Plan.Milestones}}## {{.ID}} — {{.Name | line}}

{{range .Phases}}### {{.ID}} — {{.Name | line}}

| Task | Summary | Kind | Model/Effort | Deps | Test strategy |
| --- | --- | :--: | --- | --- | --- |
{{range .Tasks}}| {{.ID}} — {{.Name | cell}} | {{.Summary | cell}} | {{dflt "code" .Kind}} | {{tier .Model .Effort}} | {{if .Deps}}{{join ", " .Deps}}{{else}}—{{end}} | {{.TestStrategy | cell}} |
{{end}}
{{range .Tasks}}- **{{.ID}} — {{.Name | line}}** — {{.Deliverable | line}}
{{if .OrchestratorOnly}}  - **orchestrator-only** — runs inline in the orchestrator; refused for subagent dispatch
{{end}}{{if .Thinking}}  - thinking: {{.Thinking | line}}
{{end}}{{if .FileSurface}}  - file surface: {{fileSurfaces .FileSurface}}
{{end}}{{range .Acceptance}}  - acceptance: {{. | line}}
{{end}}{{end}}
{{end}}{{end -}}
{{with .Plan.Risks}}## Risks
{{range .}}- {{.Risk | line}}{{if .Mitigation}} — mitigation: {{.Mitigation | line}}{{end}}{{if .ForcesPause}} **[forces pause]**{{end}}
{{end}}
{{end -}}
{{with .Plan.OpenQuestions}}## Open questions
{{range .}}- {{. | line}}
{{end}}
{{end -}}
`)

// RenderPlan renders the human-readable plan.md mirror from a Plan.
func RenderPlan(p model.Plan, meta PlanMeta) (string, error) {
	return renderMirror(planTemplate, planDoc{Plan: p, Meta: meta})
}
