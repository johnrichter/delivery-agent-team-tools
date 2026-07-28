package rendering

import (
	"strings"
	"testing"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

func oneExecPlan() model.Plan {
	return model.Plan{
		Milestones: []model.Milestone{{
			ID: "M1", Name: "m1",
			Phases: []model.Phase{{
				ID: "M1.P1", Name: "p1",
				Tasks: []model.Task{{ID: "M1.P1.T1", Name: "t1"}},
			}},
		}},
	}
}

func baseExecState() model.ExecState {
	return model.ExecState{
		Project: "proj", Name: "Proj", Topic: "tooling", Goal: "ship it",
		Started: "2026-07-03T18:00:00Z", Updated: "2026-07-03T18:10:00Z",
		RunConfig: model.RunConfig{PauseMode: "task", Budget: "unlimited", Rates: "list-price"},
	}
}

// TestRenderExecution_NoGeneratedMarker mirrors TestRenderPlan_NoGeneratedMarker for the
// execution.md mirror.
func TestRenderExecution_NoGeneratedMarker(t *testing.T) {
	out, err := RenderExecution(baseExecState(), oneExecPlan())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "GENERATED FILE") {
		t.Fatalf("render-exec output must not carry a generated-file marker:\n%s", out)
	}
}

// TestRenderExecution_TrueUsageAndCostLines confirms the true-usage/true-cost run-config
// lines carry their full original content (cache bucket, total, model count, and the
// explanatory inline comments) — a rewrite that only prints the input/output pair or drops the
// per-model count silently loses information the mirror exists to surface.
func TestRenderExecution_TrueUsageAndCostLines(t *testing.T) {
	ex := baseExecState()
	ex.RunConfig.TrueUsage = &model.Usage{InputTokens: 10, CacheCreationTokens: 3, CacheReadTokens: 2, OutputTokens: 5, TotalTokens: 20, Turns: 4}
	ex.RunConfig.Accounting = &model.Accounting{
		CostUSD: 1.5,
		Models:  map[string]*model.ModelBuckets{"claude-sonnet-5": {}, "claude-opus-4-8": {}},
	}
	out, err := RenderExecution(ex, oneExecPlan())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"true tokens (transcript): 10 in + 5 out + 5 cache = **20 total** over 4 turns   <!-- whole session incl. subagents; internal transcript format, best-effort -->",
		"true cost (transcript): **$1.5000** across 2 models   <!-- whole session incl. subagents; input+cache+output priced per anthropic-specifications.json -->",
		"spent: $0.00   <!-- cumulative OUTPUT-cost (lower bound; input tokens not priced) -->",
		"last runId: —   <!-- SAME-SESSION fast-resume only; discard on a fresh session -->",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("render-exec output missing %q:\n%s", want, out)
		}
	}
}

// TestRenderExecution_TrailingBlankLine confirms the Log section keeps its trailing blank
// line — a byte-for-byte match against the original mirror's own end-of-file shape.
func TestRenderExecution_TrailingBlankLine(t *testing.T) {
	out, err := RenderExecution(baseExecState(), oneExecPlan())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out, "- (none yet)\n\n") {
		t.Fatalf("render-exec output must end with a blank line after the Log section, got: %q", out[len(out)-30:])
	}
}
