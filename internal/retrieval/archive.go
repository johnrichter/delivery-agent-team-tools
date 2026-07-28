package retrieval

import (
	"fmt"

	"github.com/johnrichter/claude-shared-tooling/go/retrieve"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

func findArchivedMilestone(a model.ArchiveDoc, id string) (model.ArchivedMilestone, bool) {
	for _, m := range a.Milestones {
		if m.ID == id {
			return m, true
		}
	}
	return model.ArchivedMilestone{}, false
}

func findArchivedPhase(a model.ArchiveDoc, id string) (model.ArchivedPhase, bool) {
	for _, m := range a.Milestones {
		for _, ph := range m.Phases {
			if ph.ID == id {
				return ph, true
			}
		}
	}
	return model.ArchivedPhase{}, false
}

func findArchivedTask(a model.ArchiveDoc, id string) (model.ArchivedTask, bool) {
	for _, m := range a.Milestones {
		for _, ph := range m.Phases {
			for _, t := range ph.Tasks {
				if t.ID == id {
					return t, true
				}
			}
		}
	}
	return model.ArchivedTask{}, false
}

func archiveOutline(a model.ArchiveDoc) []OutlineEntry {
	var out []OutlineEntry
	for _, m := range a.Milestones {
		out = append(out, OutlineEntry{ID: m.ID, Name: m.Name})
		for _, ph := range m.Phases {
			out = append(out, OutlineEntry{ID: ph.ID, Name: ph.Name})
			for _, t := range ph.Tasks {
				out = append(out, OutlineEntry{ID: t.ID, Name: t.Summary, Status: string(t.Status), Deps: retrieve.DeepCopy(t.Deps)})
			}
		}
	}
	return out
}

func archiveGroup(a model.ArchiveDoc, id, kind string) (GroupView, error) {
	if id == "" {
		return GroupView{}, fmt.Errorf("retrieve: level %q requires --id", kind)
	}
	if entityKind(id) != kind {
		return GroupView{}, fmt.Errorf("retrieve: %q is not a %s id", id, kind)
	}
	var (
		gv    GroupView
		tasks []model.ArchivedTask
	)
	switch kind {
	case "milestone":
		m, ok := findArchivedMilestone(a, id)
		if !ok {
			return GroupView{}, fmt.Errorf("retrieve: milestone %q not found in archive", id)
		}
		gv = GroupView{ID: m.ID, Name: m.Name}
		for _, ph := range m.Phases {
			tasks = append(tasks, ph.Tasks...)
		}
	default:
		ph, ok := findArchivedPhase(a, id)
		if !ok {
			return GroupView{}, fmt.Errorf("retrieve: phase %q not found in archive", id)
		}
		gv = GroupView{ID: ph.ID, Name: ph.Name}
		tasks = ph.Tasks
	}
	for _, t := range tasks {
		gv.Tasks = append(gv.Tasks, OutlineEntry{ID: t.ID, Name: t.Summary, Status: string(t.Status), Deps: retrieve.DeepCopy(t.Deps)})
	}
	return gv, nil
}

func archiveField(a model.ArchiveDoc, id, field string) (FieldValue, error) {
	if id == "" || field == "" {
		return FieldValue{}, fmt.Errorf("retrieve: level %q requires --id and --field", LevelField)
	}
	var (
		v  any
		ok bool
	)
	switch entityKind(id) {
	case "task":
		t, found := findArchivedTask(a, id)
		if !found {
			return FieldValue{}, fmt.Errorf("retrieve: task %q not found in archive", id)
		}
		v, ok = retrieve.FieldByTag(t, field)
	case "phase":
		ph, found := findArchivedPhase(a, id)
		if !found {
			return FieldValue{}, fmt.Errorf("retrieve: phase %q not found in archive", id)
		}
		v, ok = retrieve.FieldByTag(phaseFields{ID: ph.ID, Name: ph.Name}, field)
	case "milestone":
		m, found := findArchivedMilestone(a, id)
		if !found {
			return FieldValue{}, fmt.Errorf("retrieve: milestone %q not found in archive", id)
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

// Archive projects a at the requested level — archive.json's counterpart to Plan/Exec.
func Archive(a model.ArchiveDoc, in Input) (any, error) {
	switch in.Level {
	case LevelOutline:
		if in.ID != "" {
			return nil, fmt.Errorf("retrieve: --id is not used with level %q", LevelOutline)
		}
		return archiveOutline(a), nil
	case LevelMilestone:
		return archiveGroup(a, in.ID, "milestone")
	case LevelPhase:
		return archiveGroup(a, in.ID, "phase")
	case LevelTask:
		if in.ID == "" {
			return nil, fmt.Errorf("retrieve: level %q requires --id", LevelTask)
		}
		t, ok := findArchivedTask(a, in.ID)
		if !ok {
			return nil, fmt.Errorf("retrieve: task %q not found in archive", in.ID)
		}
		return retrieve.DeepCopy(t), nil
	case LevelField:
		return archiveField(a, in.ID, in.Field)
	default:
		return nil, errUnknownLevel(in.Level)
	}
}
