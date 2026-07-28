package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/johnrichter/delivery-agent-team-tools/internal/planops"
	"github.com/johnrichter/delivery-agent-team-tools/internal/rendering"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "render":
		runRender(rest)
	case "diff":
		oldP := readPlan(arg(rest, 0, "diff <old-plan.json> <new-plan.json>"))
		newP := readPlan(arg(rest, 1, "diff <old-plan.json> <new-plan.json>"))
		printJSON(planops.Diff(oldP, newP))
	case "check-tiers":
		res := planops.CheckTiers(readPlan(arg(rest, 0, "check-tiers <plan.json>")))
		printJSON(res)
		exitOK(res.OK)
	case "hash":
		printJSON(planops.Hashes(readPlan(arg(rest, 0, "hash <plan.json>"))))
	case "validate":
		res := planops.ValidatePlanBytes(readFile(arg(rest, 0, "validate <plan.json>")))
		printJSON(res)
		exitOK(res.OK)
	case "classify":
		printJSON(runClassify(arg(rest, 0, "classify <project-dir>")))
	case "escalate":
		runEscalate(rest)
	case "classify-scope":
		runClassifyScope(rest)
	case "init-exec":
		runInitExec(rest)
	case "render-exec":
		ex := readExec(arg(rest, 0, "render-exec <execution.json> <plan.json>"))
		plan := readPlan(arg(rest, 1, "render-exec <execution.json> <plan.json>"))
		md, err := rendering.RenderExecution(ex, plan)
		if err != nil {
			die(2, "render-exec: %v\n", err)
		}
		fmt.Print(md)
	case "next":
		runNext(rest)
	case "batch":
		runBatch(rest)
	case "verify-surface":
		runVerifySurface(rest)
	case "check-changed-surface":
		runCheckChangedSurface(rest)
	case "record":
		runRecord(rest)
	case "log-note":
		runLogNote(rest)
	case "reconcile-exec":
		runReconcile(rest)
	case "archive":
		runArchive(rest)
	case "migrate-project":
		runMigrateProject(rest)
	case "usage":
		printJSON(runUsage(arg(rest, 0, "usage <transcript.jsonl>")))
	case "record-usage":
		runRecordUsage(rest)
	case "attribute":
		runAttribute(rest)
	case "retrieve":
		runRetrieve(rest)
	case "self-check":
		runSelfCheck(rest)
	case "resolve-transcript":
		runResolveTranscript(rest)
	case "feedback":
		runFeedback(rest)
	case "-h", "--help", "help":
		usage()
	default:
		die(2, "unknown command %q (try --help)\n", cmd)
	}
}

func runRender(rest []string) {
	pos := positionals(rest, 1, "render <plan.json> [--slug … --topic … --updated …]")
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	var meta rendering.PlanMeta
	fs.StringVar(&meta.Slug, "slug", "", "project slug (emits plan.md frontmatter when set)")
	fs.StringVar(&meta.Topic, "topic", "", "topic tag")
	fs.StringVar(&meta.Updated, "updated", "", "plan.md updated timestamp")
	parse(fs, rest[1:])
	md, err := rendering.RenderPlan(readPlan(pos[0]), meta)
	if err != nil {
		die(2, "render: %v\n", err)
	}
	fmt.Print(md)
}

func usage() {
	fmt.Fprint(os.Stderr, `build-helpers — deterministic mechanics for the build-with-team orchestrator

Usage:
  build-helpers render         <plan.json>                          -> plan.md
  build-helpers diff           <old-plan.json> <new-plan.json>      -> {carried,changed,added,removed}
  build-helpers check-tiers    <plan.json>                          -> {ok,issues}; exit 1 if not ok
  build-helpers hash           <plan.json>                          -> {taskId: contentHash}
  build-helpers validate       <plan.json>                          -> {ok,errors,warnings}; exit 1 if not ok
  build-helpers classify       <project-dir>                        -> {design,plan,execution,route}
  build-helpers escalate       --condition NAME [--touches-success-criteria --touches-scope] -> {route,...}
  build-helpers classify-scope [--touches-success-criteria --touches-scope] -> {route,...}
  build-helpers init-exec      <plan.json> --slug S [flags]         -> execution.json
  build-helpers render-exec    <execution.json> <plan.json>         -> execution.md
  build-helpers next           <execution.json> <plan.json>         -> {task}|{done}|{blocked}
  build-helpers batch          <execution.json> <plan.json> [--max N] -> {tasks}|{done}|{blocked}
  build-helpers verify-surface <plan.json> <taskId>[,<taskId>…] [--root DIR] -> {ok,violations}; exit 1 if not ok
  build-helpers check-changed-surface <plan.json> <taskId> --changed FILE|- -> {ok,off_surface}; exit 1 if not ok
  build-helpers record         <execution.json> <taskId> [flags]    -> execution.json
  build-helpers log-note       <execution.json> --note "…" [--at …] -> execution.json
  build-helpers reconcile-exec <execution.json> <old> <new> [flags] -> execution.json
  build-helpers archive        <plan.json> <execution.json> <archive.json> --milestone ID[,ID…] [--at …] -> {archived,skipped}
  build-helpers migrate-project <plan.json> <execution.json> [--dry-run] -> {already_v2,dry_run,changes,warnings}
  build-helpers usage          <transcript.jsonl>                   -> {input,output,cache,total,turns}
  build-helpers record-usage   <execution.json> --transcript P [--specs P --final --baseline-capture] -> execution.json
  build-helpers attribute      <execution.json> --transcript P [--tasks id,id,… --specs P] -> {tasks,even_split,unmappable}
  build-helpers retrieve       <plan.json|execution.json|archive.json> --level {outline|milestone|phase|task|field} [--id ID --field NAME]
  build-helpers self-check     {--floor-model M --floor-effort E --ceiling-model M --ceiling-effort E | --band NAME} [--transcript P --settings P --session-id ID | --scratchpad-path P]
  build-helpers resolve-transcript --session-id ID | --scratchpad-path P [--cwd DIR --projects-dir DIR]
  build-helpers feedback add   <feedback.json> --title … --feedback … --impact N --urgency N [--source-task … --proposed-solution … --why-it-matters … --at …]
  build-helpers feedback list  <feedback.json> [--by-task ID --min-impact N --min-urgency N]
  build-helpers feedback gate  <feedback.json> --plan <plan.json> --threshold N

Exit codes: 0 ok; 1 a command-specific business check failed; 2 usage/IO error; 3 roster-stale (self-check only).
Positionals precede flags: <positionals…> --key value (Go flag stops at the first positional).
`)
}
