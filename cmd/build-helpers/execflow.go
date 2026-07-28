package main

import (
	"flag"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/archive"
	"github.com/johnrichter/delivery-agent-team-tools/internal/execops"
	"github.com/johnrichter/delivery-agent-team-tools/internal/migrate"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

func runInitExec(rest []string) {
	pos := positionals(rest, 1, "init-exec <plan.json> --slug S [--name … --topic … --design-updated … --plan-updated … --pause … --budget … --rates … --override … --at …]")
	fs := flag.NewFlagSet("init-exec", flag.ContinueOnError)
	var o execops.InitOptions
	fs.StringVar(&o.Slug, "slug", "", "project slug (required)")
	fs.StringVar(&o.Name, "name", "", "execution doc name")
	fs.StringVar(&o.Topic, "topic", "", "topic tag")
	fs.StringVar(&o.DesignUpdated, "design-updated", "", "source design.md updated timestamp")
	fs.StringVar(&o.PlanUpdated, "plan-updated", "", "source plan.json updated timestamp")
	fs.StringVar(&o.Pause, "pause", "", "pause mode: task|phase|milestone|none")
	fs.StringVar(&o.Budget, "budget", "", "unlimited | $<amount>")
	fs.StringVar(&o.Rates, "rates", "", "list-price | negotiated")
	fs.StringVar(&o.Override, "override", "", "active override directive")
	at := fs.String("at", "", "timestamp override (default: now UTC)")
	parse(fs, rest[1:])
	o.At = orNow(*at)
	plan := readPlan(pos[0])
	ex, err := execops.Init(plan, o)
	if err != nil {
		die(2, "init-exec: %v\n", err)
	}
	printJSON(ex)
}

func runRecord(rest []string) {
	pos := positionals(rest, 2, "record <execution.json> <taskId> [--status … --test … --review … --commit … --cost … --tokens-out … --input-tokens … --cache-write-tokens … --cache-read-tokens … --usage-turns … --note … --run-id … --override … --at …]")
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	status := fs.String("status", "", "not-started|in-progress|blocked|failed|done|superseded")
	test := fs.String("test", "", "PASS|FAIL")
	review := fs.String("review", "", "ACCEPT|FIX-APPLIED|RETURN")
	commit := fs.String("commit", "", "short SHA")
	note := fs.String("note", "", "row note")
	runID := fs.String("run-id", "", "Workflow runId (same-session fast-resume)")
	override := fs.String("override", "", "active override directive")
	cost := fs.Float64("cost", 0, "task cost in USD")
	tokensOut := fs.Int64("tokens-out", 0, "measured output tokens for this task")
	inputTokens := fs.Int64("input-tokens", 0, "this task's measured input tokens")
	cacheWriteTokens := fs.Int64("cache-write-tokens", 0, "this task's measured cache-write tokens")
	cacheReadTokens := fs.Int64("cache-read-tokens", 0, "this task's measured cache-read tokens")
	usageTurns := fs.Int64("usage-turns", 0, "this task's measured turn count")
	at := fs.String("at", "", "timestamp override (default: now UTC)")
	parse(fs, rest[2:])

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	var f execops.RecordFields
	if set["status"] {
		s := model.Status(*status)
		f.Status = &s
	}
	if set["test"] {
		f.Test = test
	}
	if set["review"] {
		f.Review = review
	}
	if set["commit"] {
		f.Commit = commit
	}
	if set["note"] {
		f.Note = note
	}
	if set["run-id"] {
		f.RunID = runID
	}
	if set["override"] {
		f.Override = override
	}
	if set["cost"] {
		f.Cost = cost
	}
	if set["tokens-out"] {
		f.TokensOut = tokensOut
	}
	if set["input-tokens"] || set["cache-write-tokens"] || set["cache-read-tokens"] || set["tokens-out"] || set["usage-turns"] {
		u := &model.Usage{
			InputTokens: *inputTokens, CacheCreationTokens: *cacheWriteTokens, CacheReadTokens: *cacheReadTokens,
			OutputTokens: *tokensOut, Turns: *usageTurns, At: orNow(*at),
		}
		u.TotalTokens = u.InputTokens + u.CacheCreationTokens + u.CacheReadTokens + u.OutputTokens
		f.Usage = u
	}
	ex := readExec(pos[0])
	if err := execops.Record(&ex, pos[1], f, orNow(*at)); err != nil {
		die(2, "%v\n", err)
	}
	printJSON(ex)
}

func runLogNote(rest []string) {
	pos := positionals(rest, 1, "log-note <execution.json> --note \"…\" [--at …]")
	fs := flag.NewFlagSet("log-note", flag.ContinueOnError)
	note := fs.String("note", "", "plan-level log entry (required)")
	at := fs.String("at", "", "timestamp override (default: now UTC)")
	parse(fs, rest[1:])
	if *note == "" {
		die(2, "log-note: --note is required\n")
	}
	ex := readExec(pos[0])
	execops.LogNote(&ex, *note, orNow(*at))
	printJSON(ex)
}

func runReconcile(rest []string) {
	pos := positionals(rest, 3, "reconcile-exec <execution.json> <old-plan.json> <new-plan.json> [--design-updated … --plan-updated … --at …]")
	fs := flag.NewFlagSet("reconcile-exec", flag.ContinueOnError)
	designUpdated := fs.String("design-updated", "", "new design.md updated timestamp")
	planUpdated := fs.String("plan-updated", "", "new plan.json updated timestamp")
	at := fs.String("at", "", "timestamp override (default: now UTC)")
	parse(fs, rest[3:])
	ex := readExec(pos[0])
	oldP := readPlan(pos[1])
	newP := readPlan(pos[2])
	execops.Reconcile(&ex, oldP, newP, *designUpdated, *planUpdated, orNow(*at))
	printJSON(ex)
}

func splitTasks(csv string) []string {
	var out []string
	for _, part := range strings.Split(csv, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runArchive(rest []string) {
	pos := positionals(rest, 3, "archive <plan.json> <execution.json> <archive.json> --milestone ID[,ID…] [--at …]")
	fs := flag.NewFlagSet("archive", flag.ContinueOnError)
	milestones := fs.String("milestone", "", "comma-separated milestone id(s) to archive (required)")
	at := fs.String("at", "", "timestamp override (default: now UTC)")
	parse(fs, rest[3:])
	ids := splitTasks(*milestones)
	if len(ids) == 0 {
		die(2, "archive: --milestone is required (comma-separated milestone id(s))\n")
	}

	plan := readPlan(pos[0])
	ex := readExec(pos[1])
	existing := readArchive(pos[2])

	out, err := archive.Run(plan, ex, existing, archive.Options{MilestoneIDs: ids}, orNow(*at))
	if err != nil {
		die(2, "%v\n", err)
	}

	writeJSONFile(pos[2], out.Archive)
	writeJSONFile(pos[1], out.Exec)
	writeJSONFile(pos[0], out.Plan)

	printJSON(struct {
		Archived []string `json:"archived"`
		Skipped  []string `json:"skipped,omitempty"`
	}{out.Archived, out.Skipped})
}

func runMigrateProject(rest []string) {
	pos := positionals(rest, 2, "migrate-project <plan.json> <execution.json> [--dry-run]")
	fs := flag.NewFlagSet("migrate-project", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "preview the exact changes without writing either file")
	parse(fs, rest[2:])

	plan := readPlan(pos[0])
	ex := readExecRaw(pos[1])

	rep, err := migrate.Run(&plan, &ex, *dryRun)
	if err != nil {
		die(2, "migrate-project: %v\n", err)
	}
	if !rep.AlreadyV2 && !*dryRun {
		writeJSONFile(pos[0], plan)
		writeJSONFile(pos[1], ex)
	}
	printJSON(rep)
}
