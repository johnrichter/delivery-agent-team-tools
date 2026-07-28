package schedule

import (
	"strings"

	"github.com/johnrichter/claude-shared-tooling/go/graph"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
	"github.com/johnrichter/delivery-agent-team-tools/internal/planops"
)

// MaxBatch is the hard ceiling on how many tasks a single fan-out may dispatch at once,
// regardless of the requested --max.
const MaxBatch = 8

// pathProver decides file_surface disjointness over a single "path" resource domain: file/dir/
// glob claims compared with doublestar-dialect glob semantics. A dir claim covering another
// entry's path also catches the same-package-symbol risk a literal path/glob comparison alone
// cannot see.
var pathProver = mustProver()

func mustProver() *graph.Prover {
	p, err := graph.NewProver(graph.PathDomain("path", graph.WithPathMatcher(matchGlob)))
	if err != nil {
		panic(err)
	}
	return p
}

// surfaceOf renders a task's declared file_surface as a graph.Surface. A task with no declared
// surface returns nil — an UNDECLARED domain, which the prover always treats as unsafe to
// batch beside, matching "an unknown surface overlaps everything".
func surfaceOf(entries []model.FileSurfaceEntry) graph.Surface {
	if len(entries) == 0 {
		return nil
	}
	claims := make([]graph.Claim, len(entries))
	for i, e := range entries {
		claims[i] = graph.Claim{Kind: string(e.Kind.Resolve()), Value: e.Path}
	}
	return graph.Surface{"path": claims}
}

// Task is one task admitted into a parallel batch.
type Task struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Summary string       `json:"summary"`
	Model   model.Model  `json:"model"`
	Effort  model.Effort `json:"effort"`
}

// Result is exactly one of: a non-empty batch of independent, file_surface-disjoint tasks, an
// orchestrator-only refusal, done, or blocked — mirroring NextResult's four outcomes.
type Result struct {
	Tasks            []Task   `json:"tasks,omitempty"`
	OrchestratorOnly *Task    `json:"orchestrator_only,omitempty"`
	Done             bool     `json:"done,omitempty"`
	Blocked          []string `json:"blocked,omitempty"`
	Reason           string   `json:"reason,omitempty"`
}

// Batch selects up to clamped-max tasks to run in parallel. The candidate pool is scoped by
// run_config.pause_mode (task/phase/milestone/none, relative to the anchor — the same
// next-eligible task Next would pick); from that pool it walks in dependency order and greedily
// admits a task only when the dependency graph proves it independent of, and the resource
// prover proves it file_surface-disjoint from, every already-admitted task.
func Batch(ex model.ExecState, p model.Plan, max int) Result {
	s := newScheduleState(ex, p)
	if !s.anyUnfinished() {
		if len(s.topo.Cycle) > 0 {
			return Result{Blocked: s.topo.Cycle, Reason: "unschedulable (cycle/dangling deps): " + strings.Join(s.topo.Cycle, ", ")}
		}
		return Result{Done: true}
	}

	anchor := ""
	for _, id := range s.topo.Order {
		if s.eligible(id) {
			anchor = id
			break
		}
	}
	if anchor == "" {
		return Result{Blocked: s.unfinishedIDs(), Reason: s.blockedReason()}
	}
	anchorRef := s.refByID[anchor]
	if anchorRef.Task.OrchestratorOnly {
		row := s.rowByID[anchor]
		return Result{OrchestratorOnly: &Task{ID: anchor, Name: anchorRef.Task.Name, Summary: row.Summary, Model: row.Model, Effort: row.Effort}}
	}

	mode := strings.TrimSpace(ex.RunConfig.PauseMode)
	inBoundary := func(id string) bool {
		switch mode {
		case "task":
			return id == anchor
		case "phase":
			return s.refByID[id].Phase.ID == anchorRef.Phase.ID
		case "milestone":
			return s.refByID[id].Milestone.ID == anchorRef.Milestone.ID
		default:
			return true
		}
	}

	cap := MaxBatch
	if max > 0 && max < cap {
		cap = max
	}

	g := planops.BuildGraph(p)
	var admitted []string
	for _, id := range s.topo.Order {
		if len(admitted) >= cap {
			break
		}
		if !inBoundary(id) || !s.eligible(id) || s.refByID[id].Task.OrchestratorOnly {
			continue
		}
		ok := true
		for _, a := range admitted {
			surfaces := func(x string) graph.Surface { return surfaceOf(s.refByID[x].Task.FileSurface) }
			if can, _ := g.CanCoBatch(id, a, surfaces, pathProver); !can {
				ok = false
				break
			}
		}
		if ok {
			admitted = append(admitted, id)
		}
	}

	tasks := make([]Task, 0, len(admitted))
	for _, id := range admitted {
		ref, row := s.refByID[id], s.rowByID[id]
		tasks = append(tasks, Task{ID: id, Name: ref.Task.Name, Summary: row.Summary, Model: row.Model, Effort: row.Effort})
	}
	return Result{Tasks: tasks}
}
