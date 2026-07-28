// Package planops holds plan.json's read-only computations: dependency ordering, content
// hashing, reconciliation diffing, tier checking, and schema+integrity validation. Every
// function here is pure — no filesystem IO, no process exit.
package planops

import "github.com/johnrichter/delivery-agent-team-tools/internal/model"

// TaskRef is a task paired with its milestone/phase, yielded in plan order.
type TaskRef struct {
	Task      model.Task
	Milestone model.Milestone
	Phase     model.Phase
}

// WalkTasks returns every task in plan order with its milestone/phase context.
func WalkTasks(p model.Plan) []TaskRef {
	var refs []TaskRef
	for _, m := range p.Milestones {
		for _, ph := range m.Phases {
			for _, t := range ph.Tasks {
				refs = append(refs, TaskRef{Task: t, Milestone: m, Phase: ph})
			}
		}
	}
	return refs
}

// TaskByID looks up one task by id via WalkTasks.
func TaskByID(p model.Plan, id string) (TaskRef, bool) {
	for _, r := range WalkTasks(p) {
		if r.Task.ID == id {
			return r, true
		}
	}
	return TaskRef{}, false
}
