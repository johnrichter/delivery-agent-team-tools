package retrieval

import (
	"fmt"

	"github.com/johnrichter/claude-shared-tooling/go/retrieve"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// milestoneFields / phaseFields expose only a milestone/phase's own scalar fields to
// retrieve.FieldByTag — never its nested Phases/Tasks, which would blow the field level's
// single-value contract.
type milestoneFields struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type phaseFields struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func findMilestone(p model.Plan, id string) (model.Milestone, bool) {
	for _, m := range p.Milestones {
		if m.ID == id {
			return m, true
		}
	}
	return model.Milestone{}, false
}

func findPhase(p model.Plan, id string) (model.Phase, bool) {
	for _, m := range p.Milestones {
		for _, ph := range m.Phases {
			if ph.ID == id {
				return ph, true
			}
		}
	}
	return model.Phase{}, false
}

func findTask(p model.Plan, id string) (model.Task, bool) {
	for _, m := range p.Milestones {
		for _, ph := range m.Phases {
			for _, t := range ph.Tasks {
				if t.ID == id {
					return t, true
				}
			}
		}
	}
	return model.Task{}, false
}

func planOutline(p model.Plan) []OutlineEntry {
	var out []OutlineEntry
	for _, m := range p.Milestones {
		out = append(out, OutlineEntry{ID: m.ID, Name: m.Name})
		for _, ph := range m.Phases {
			out = append(out, OutlineEntry{ID: ph.ID, Name: ph.Name})
			for _, t := range ph.Tasks {
				out = append(out, OutlineEntry{ID: t.ID, Name: t.Name, Deps: retrieve.DeepCopy(t.Deps)})
			}
		}
	}
	return out
}

func planGroup(p model.Plan, id, kind string) (GroupView, error) {
	if id == "" {
		return GroupView{}, fmt.Errorf("retrieve: level %q requires --id", kind)
	}
	if entityKind(id) != kind {
		return GroupView{}, fmt.Errorf("retrieve: %q is not a %s id", id, kind)
	}
	var (
		gv    GroupView
		tasks []model.Task
	)
	switch kind {
	case "milestone":
		m, ok := findMilestone(p, id)
		if !ok {
			return GroupView{}, fmt.Errorf("retrieve: milestone %q not found in plan", id)
		}
		gv = GroupView{ID: m.ID, Name: m.Name}
		for _, ph := range m.Phases {
			tasks = append(tasks, ph.Tasks...)
		}
	default: // "phase"
		ph, ok := findPhase(p, id)
		if !ok {
			return GroupView{}, fmt.Errorf("retrieve: phase %q not found in plan", id)
		}
		gv = GroupView{ID: ph.ID, Name: ph.Name}
		tasks = ph.Tasks
	}
	for _, t := range tasks {
		gv.Tasks = append(gv.Tasks, OutlineEntry{ID: t.ID, Name: t.Name, Deps: retrieve.DeepCopy(t.Deps)})
	}
	return gv, nil
}

func planField(p model.Plan, id, field string) (FieldValue, error) {
	if id == "" || field == "" {
		return FieldValue{}, fmt.Errorf("retrieve: level %q requires --id and --field", LevelField)
	}
	var (
		v  any
		ok bool
	)
	switch entityKind(id) {
	case "task":
		t, found := findTask(p, id)
		if !found {
			return FieldValue{}, fmt.Errorf("retrieve: task %q not found in plan", id)
		}
		v, ok = retrieve.FieldByTag(t, field)
	case "phase":
		ph, found := findPhase(p, id)
		if !found {
			return FieldValue{}, fmt.Errorf("retrieve: phase %q not found in plan", id)
		}
		v, ok = retrieve.FieldByTag(phaseFields{ID: ph.ID, Name: ph.Name}, field)
	case "milestone":
		m, found := findMilestone(p, id)
		if !found {
			return FieldValue{}, fmt.Errorf("retrieve: milestone %q not found in plan", id)
		}
		v, ok = retrieve.FieldByTag(milestoneFields{ID: m.ID, Name: m.Name}, field)
	default:
		return FieldValue{}, fmt.Errorf("retrieve: %q is not a recognized milestone/phase/task id", id)
	}
	if !ok {
		return FieldValue{}, fmt.Errorf("retrieve: %s %q has no field %q", entityKind(id), id, field)
	}
	return FieldValue{ID: id, Field: field, Value: v}, nil
}

// Plan projects p at the requested level.
func Plan(p model.Plan, in Input) (any, error) {
	switch in.Level {
	case LevelOutline:
		if in.ID != "" {
			return nil, fmt.Errorf("retrieve: --id is not used with level %q", LevelOutline)
		}
		return planOutline(p), nil
	case LevelMilestone:
		return planGroup(p, in.ID, "milestone")
	case LevelPhase:
		return planGroup(p, in.ID, "phase")
	case LevelTask:
		if in.ID == "" {
			return nil, fmt.Errorf("retrieve: level %q requires --id", LevelTask)
		}
		t, ok := findTask(p, in.ID)
		if !ok {
			return nil, fmt.Errorf("retrieve: task %q not found in plan", in.ID)
		}
		return retrieve.DeepCopy(t), nil
	case LevelField:
		return planField(p, in.ID, in.Field)
	default:
		return nil, errUnknownLevel(in.Level)
	}
}
