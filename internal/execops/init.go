// Package execops mutates execution.json: init-exec, record, log-note, reconcile-exec, and the
// schema-version upgrade every load path applies. Pure — no filesystem IO, no process exit.
package execops

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
	"github.com/johnrichter/delivery-agent-team-tools/internal/planops"
)

// InitOptions are the run-config inputs gathered up front by the caller.
type InitOptions struct {
	Slug          string
	Name          string
	Topic         string
	DesignUpdated string
	PlanUpdated   string
	Pause         string
	Budget        string
	Rates         string
	Override      string
	At            string
}

// ParseBudget normalizes a budget string into a display label and an optional dollar ceiling.
func ParseBudget(b string) (label string, ceiling *float64, err error) {
	b = strings.TrimSpace(b)
	if b == "" || b == "unlimited" {
		return "unlimited", nil, nil
	}
	n, e := strconv.ParseFloat(strings.TrimPrefix(b, "$"), 64)
	if e != nil {
		return "", nil, fmt.Errorf("budget must be 'unlimited' or a dollar amount, got %q", b)
	}
	v := n
	return fmt.Sprintf("$%.2f", n), &v, nil
}

// Init builds the canonical execution.json for a fresh plan: one not-started row per task, run
// config seeded from opt, provenance filled.
func Init(p model.Plan, opt InitOptions) (model.ExecState, error) {
	if strings.TrimSpace(opt.Slug) == "" {
		return model.ExecState{}, fmt.Errorf("init-exec requires --slug")
	}
	label, ceiling, err := ParseBudget(opt.Budget)
	if err != nil {
		return model.ExecState{}, err
	}
	rows := []model.ExecTask{}
	for _, r := range planops.WalkTasks(p) {
		rows = append(rows, model.ExecTask{
			ID: r.Task.ID, Summary: r.Task.Summary, Kind: r.Task.Kind.Resolve(),
			Model: r.Task.Model, Effort: r.Task.Effort, Status: model.StatusNotStarted, Updated: opt.At,
		})
	}
	return model.ExecState{
		Schema: model.ExecSchema, SchemaVersion: model.CurrentExecSchemaVersion,
		Project: opt.Slug, Name: cmp.Or(opt.Name, opt.Slug+" — Execution"), Topic: cmp.Or(opt.Topic, "tooling"), Goal: p.Goal,
		Provenance: model.Provenance{DesignUpdated: opt.DesignUpdated, PlanUpdated: opt.PlanUpdated, DerivedAt: opt.At},
		Started:    opt.At, Updated: opt.At,
		RunConfig: model.RunConfig{
			PauseMode: cmp.Or(opt.Pause, "phase"), Budget: label, BudgetCeilingUSD: ceiling,
			Rates: cmp.Or(opt.Rates, "list-price"), Override: opt.Override,
		},
		Tasks: rows, Log: []string{},
	}, nil
}

// MigrateSchema upgrades ex in place to the current schema version. A version newer than this
// build supports is refused rather than risk silently dropping unrecognized fields.
func MigrateSchema(ex *model.ExecState) error {
	v := ex.SchemaVersion
	if v == 0 {
		v = model.LegacyExecSchemaVersion
	}
	if v > model.CurrentExecSchemaVersion {
		return fmt.Errorf("execution.json schema_version %d is newer than this build supports (max %d) — upgrade before resuming this execution", v, model.CurrentExecSchemaVersion)
	}
	ex.SchemaVersion = model.CurrentExecSchemaVersion
	return nil
}
