package rendering

import (
	"strings"
	"testing"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

func onePlan() model.Plan {
	return model.Plan{
		Goal:            "ship it",
		SuccessCriteria: []string{"it ships"},
		Milestones: []model.Milestone{{
			ID: "M1", Name: "m1",
			Phases: []model.Phase{{
				ID: "M1.P1", Name: "p1",
				Tasks: []model.Task{{
					ID: "M1.P1.T1", Name: "t1", Summary: "do it", Deliverable: "the thing",
					Model: "claude-sonnet-5", Effort: "medium", TestStrategy: "run it",
					Acceptance:  []string{"it works"},
					FileSurface: []model.FileSurfaceEntry{{Path: "main.go", Required: true}},
				}},
			}},
		}},
	}
}

// TestRenderPlan_NoGeneratedMarker confirms the frozen `render` stdout contract (raw Markdown
// only) never gains an unrequested generated-file banner — the CLI's caller pipes this output
// straight to plan.md.
func TestRenderPlan_NoGeneratedMarker(t *testing.T) {
	out, err := RenderPlan(onePlan(), PlanMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "GENERATED FILE") {
		t.Fatalf("render output must not carry a generated-file marker:\n%s", out)
	}
}

// TestRenderPlan_FileSurfaceLine confirms a task's declared file_surface renders — dropping it
// silently discards load-bearing plan content a human/plugin reads from plan.md.
func TestRenderPlan_FileSurfaceLine(t *testing.T) {
	out, err := RenderPlan(onePlan(), PlanMeta{})
	if err != nil {
		t.Fatal(err)
	}
	want := "  - file surface: main.go (file, required)"
	if !strings.Contains(out, want) {
		t.Fatalf("render output missing file-surface line %q:\n%s", want, out)
	}
}

// TestRenderPlan_NoFrontmatterWithoutSlug confirms the frontmatter block stays opt-in (empty
// Slug omits it), matching the original ad-hoc-viewing behavior.
func TestRenderPlan_NoFrontmatterWithoutSlug(t *testing.T) {
	out, err := RenderPlan(onePlan(), PlanMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(out, "---") {
		t.Fatalf("render output must omit frontmatter when Slug is empty:\n%s", out)
	}
}
