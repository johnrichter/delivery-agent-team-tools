package main

import (
	"encoding/json"
	"flag"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
	"github.com/johnrichter/delivery-agent-team-tools/internal/retrieval"
)

// isArchiveDoc sniffs a doc's shape ahead of isExecutionDoc's plan/exec fork: archive.json's
// own "schema" key is the one unambiguous discriminator, so this check must run first.
func isArchiveDoc(raw []byte) bool {
	var probe struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.Schema == model.ArchiveSchema
}

// isExecutionDoc sniffs a doc's shape (never its filename): only Plan declares a top-level
// "milestones" array, so its absence means the doc is execution.json.
func isExecutionDoc(raw []byte) bool {
	var probe struct {
		Milestones json.RawMessage `json:"milestones"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return true
	}
	return probe.Milestones == nil
}

func runRetrieve(rest []string) {
	pos := positionals(rest, 1, "retrieve <plan.json|execution.json|archive.json> --level {outline|milestone|phase|task|field} [--id ID] [--field NAME]")
	fs := flag.NewFlagSet("retrieve", flag.ContinueOnError)
	level := fs.String("level", "", "outline|milestone|phase|task|field (required)")
	id := fs.String("id", "", "entity id (required for milestone/phase/task/field)")
	field := fs.String("field", "", "field name (required for level field)")
	parse(fs, rest[1:])

	in := retrieval.Input{Level: retrieval.Level(*level), ID: *id, Field: *field}
	raw := readFile(pos[0])
	var (
		result any
		err    error
	)
	switch {
	case isArchiveDoc(raw):
		result, err = retrieval.Archive(readArchive(pos[0]), in)
	case isExecutionDoc(raw):
		result, err = retrieval.Exec(readExec(pos[0]), in)
	default:
		result, err = retrieval.Plan(readPlan(pos[0]), in)
	}
	if err != nil {
		die(2, "%v\n", err)
	}
	printJSON(result)
}
