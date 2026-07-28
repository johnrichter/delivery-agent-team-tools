package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/feedback"
)

// runFeedback dispatches the feedback subcommands: add (writes), list (read-only query), gate
// (read-only threshold partition).
func runFeedback(rest []string) {
	if len(rest) == 0 {
		die(2, "usage: feedback {add|list|gate} <feedback.json> [flags]\n")
	}
	sub, rest := rest[0], rest[1:]
	switch sub {
	case "add":
		runFeedbackAdd(rest)
	case "list":
		runFeedbackList(rest)
	case "gate":
		runFeedbackGate(rest)
	default:
		die(2, "unknown feedback subcommand %q (try: add, list, gate)\n", sub)
	}
}

// runFeedbackAdd validates and appends one entry to feedback.json, then re-renders feedback.md
// from the same updated register in the same call — the two can never diverge.
func runFeedbackAdd(rest []string) {
	pos := positionals(rest, 1, "feedback add <feedback.json> --title … --feedback … --impact N --urgency N [--source-task … --proposed-solution … --why-it-matters … --at …]")
	fs := flag.NewFlagSet("feedback add", flag.ContinueOnError)
	var in feedback.Input
	fs.StringVar(&in.Title, "title", "", "short human name — required")
	fs.StringVar(&in.SourceTaskID, "source-task", "", "originating task id")
	fs.StringVar(&in.Feedback, "feedback", "", "the feedback itself — required")
	fs.StringVar(&in.ProposedSolution, "proposed-solution", "", "proposed fix")
	fs.StringVar(&in.WhyItMatters, "why-it-matters", "", "why this matters / impact rationale")
	fs.IntVar(&in.Impact, "impact", 0, "impact score 1-5 — required")
	fs.IntVar(&in.Urgency, "urgency", 0, "urgency score 1-5 — required")
	at := fs.String("at", "", "timestamp override (default: now UTC)")
	parse(fs, rest[1:])

	path := pos[0]
	reg, err := feedback.Add(readFeedback(path), in, orNow(*at))
	if err != nil {
		die(2, "%v\n", err)
	}
	writeJSONFile(path, reg)
	writeTextFile(feedbackMirrorPath(path), feedback.Render(reg))
	printJSON(reg)
}

// runFeedbackList reads feedback.json (never writes it) and prints entries matching the
// supplied filters, ranked by criticality descending.
func runFeedbackList(rest []string) {
	pos := positionals(rest, 1, "feedback list <feedback.json> [--by-task ID] [--min-impact N] [--min-urgency N]")
	fs := flag.NewFlagSet("feedback list", flag.ContinueOnError)
	var f feedback.Filter
	fs.StringVar(&f.SourceTaskID, "by-task", "", "filter to entries whose source task id exactly matches")
	fs.IntVar(&f.MinImpact, "min-impact", 0, "filter to entries with impact >= N")
	fs.IntVar(&f.MinUrgency, "min-urgency", 0, "filter to entries with urgency >= N")
	parse(fs, rest[1:])

	reg := readFeedback(pos[0])
	for _, e := range feedback.List(reg, f) {
		fmt.Println(e.ID + " — " + e.Title)
	}
}

// runFeedbackGate partitions the ranked register at the configurable criticality threshold.
func runFeedbackGate(rest []string) {
	pos := positionals(rest, 1, "feedback gate <feedback.json> --plan <plan.json> --threshold N")
	fs := flag.NewFlagSet("feedback gate", flag.ContinueOnError)
	planPath := fs.String("plan", "", "plan.json the feedback-review milestone is applied to — required")
	threshold := fs.Int("threshold", 0, "inclusive criticality floor for amend-now — required")
	parse(fs, rest[1:])
	if strings.TrimSpace(*planPath) == "" {
		die(2, "feedback gate: --plan <plan.json> is required\n")
	}
	printJSON(feedback.GatePlan(readPlan(*planPath), readFeedback(pos[0]), *threshold))
}
