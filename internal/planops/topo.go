package planops

import (
	"sort"

	"github.com/johnrichter/claude-shared-tooling/go/graph"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// TopoResult is a dependency-respecting task order plus any unschedulable tasks.
type TopoResult struct {
	Order []string
	Cycle []string // tasks that never drained: a dep cycle or a task downstream of one
}

// BuildGraph loads every task into a graph.Graph, wiring an edge for each dep that resolves to
// a real task in the plan. A dep naming a non-existent task, or a task naming itself, is
// dropped as an edge here — ValidatePlan reports both as integrity errors; a single typo must
// never masquerade as a cycle. Exported so the scheduler (next/batch) shares the same
// dependency structure this package's own topological order is computed from.
func BuildGraph(p model.Plan) *graph.Graph[string, struct{}] {
	g := graph.New[string, struct{}](graph.StringIDs())
	for _, r := range WalkTasks(p) {
		_ = g.AddNode(r.Task.ID, struct{}{})
	}
	for _, r := range WalkTasks(p) {
		for _, d := range r.Task.Deps {
			if d != r.Task.ID && g.Has(d) {
				_ = g.AddDep(r.Task.ID, d)
			}
		}
	}
	return g
}

// TopoOrder computes a dependency-respecting task order via Kahn's algorithm over the plan's
// dependency graph, breaking every tie among simultaneously-ready tasks by original plan
// order — the same tie-break `next`/`batch` rely on to pick a deterministic candidate.
func TopoOrder(p model.Plan) TopoResult {
	g := BuildGraph(p)
	ids := g.IDs()
	idx := make(map[string]int, len(ids))
	for i, id := range ids {
		idx[id] = i
	}
	indeg := make(map[string]int, len(ids))
	for _, id := range ids {
		indeg[id] = len(g.Deps(id))
	}
	var ready []string
	for _, id := range ids {
		if indeg[id] == 0 {
			ready = append(ready, id)
		}
	}
	done := make(map[string]bool, len(ids))
	var order []string
	for len(ready) > 0 {
		sort.Slice(ready, func(i, j int) bool { return idx[ready[i]] < idx[ready[j]] })
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		done[id] = true
		for _, dep := range g.Dependents(id) {
			indeg[dep]--
			if indeg[dep] == 0 {
				ready = append(ready, dep)
			}
		}
	}
	var cycle []string
	for _, id := range ids {
		if !done[id] {
			cycle = append(cycle, id)
		}
	}
	return TopoResult{Order: order, Cycle: cycle}
}
