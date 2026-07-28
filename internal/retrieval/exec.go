package retrieval

import (
	"fmt"
	"strings"

	"github.com/johnrichter/claude-shared-tooling/go/retrieve"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// execution.json has no independent milestone/phase objects — only its flat task list.
// Milestone/phase views are synthesized by grouping on the hierarchical task-id prefix
// plan.json's ids establish.

func execOutline(ex model.ExecState) []OutlineEntry {
	out := make([]OutlineEntry, 0, len(ex.Tasks))
	for _, t := range ex.Tasks {
		out = append(out, OutlineEntry{ID: t.ID, Name: t.Summary, Status: string(t.Status)})
	}
	return out
}

func execGroup(ex model.ExecState, id, kind string) (GroupView, error) {
	if id == "" {
		return GroupView{}, fmt.Errorf("retrieve: level %q requires --id", kind)
	}
	if entityKind(id) != kind {
		return GroupView{}, fmt.Errorf("retrieve: %q is not a %s id", id, kind)
	}
	prefix := id + "."
	gv := GroupView{ID: id}
	for _, t := range ex.Tasks {
		if strings.HasPrefix(t.ID, prefix) {
			gv.Tasks = append(gv.Tasks, OutlineEntry{ID: t.ID, Name: t.Summary, Status: string(t.Status)})
		}
	}
	if len(gv.Tasks) == 0 {
		return GroupView{}, fmt.Errorf("retrieve: no tasks found under %s %q in execution state", kind, id)
	}
	return gv, nil
}

func execField(ex model.ExecState, id, field string) (FieldValue, error) {
	if id == "" || field == "" {
		return FieldValue{}, fmt.Errorf("retrieve: level %q requires --id and --field", LevelField)
	}
	if entityKind(id) != "task" {
		return FieldValue{}, fmt.Errorf("retrieve: execution.json has no milestone/phase-scoped fields (id %q) — query plan.json for milestone/phase fields", id)
	}
	for _, t := range ex.Tasks {
		if t.ID == id {
			v, ok := retrieve.FieldByTag(t, field)
			if !ok {
				return FieldValue{}, fmt.Errorf("retrieve: task %q has no field %q", id, field)
			}
			return FieldValue{ID: id, Field: field, Value: v}, nil
		}
	}
	return FieldValue{}, fmt.Errorf("retrieve: task %q not found in execution state", id)
}

// Exec projects ex at the requested level — execution.json's counterpart to Plan.
func Exec(ex model.ExecState, in Input) (any, error) {
	switch in.Level {
	case LevelOutline:
		if in.ID != "" {
			return nil, fmt.Errorf("retrieve: --id is not used with level %q", LevelOutline)
		}
		return execOutline(ex), nil
	case LevelMilestone:
		return execGroup(ex, in.ID, "milestone")
	case LevelPhase:
		return execGroup(ex, in.ID, "phase")
	case LevelTask:
		if in.ID == "" {
			return nil, fmt.Errorf("retrieve: level %q requires --id", LevelTask)
		}
		for _, t := range ex.Tasks {
			if t.ID == in.ID {
				return retrieve.DeepCopy(t), nil
			}
		}
		return nil, fmt.Errorf("retrieve: task %q not found in execution state", in.ID)
	case LevelField:
		return execField(ex, in.ID, in.Field)
	default:
		return nil, errUnknownLevel(in.Level)
	}
}
