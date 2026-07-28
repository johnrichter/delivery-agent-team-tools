// Command build-helpers is the CLI front-end for the build-with-team deterministic engine. It
// owns all filesystem IO and process exit codes; the logic lives in the internal packages
// (pure, testable). Exit codes: 0 ok; 1 a command-specific business check failed (never a usage
// error); 2 usage/IO error; 3 roster-stale (self-check only).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnrichter/delivery-agent-team-tools/internal/docio"
	"github.com/johnrichter/delivery-agent-team-tools/internal/execops"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

func die(code int, format string, a ...any) {
	fmt.Fprintf(os.Stderr, "build-helpers: "+format, a...)
	os.Exit(code)
}

func exitOK(ok bool) {
	if !ok {
		os.Exit(1)
	}
}

// positionals returns the first n args, dying with usage if there are too few.
func positionals(rest []string, n int, usageLine string) []string {
	if len(rest) < n {
		die(2, "usage: %s\n", usageLine)
	}
	return rest[:n]
}

func arg(rest []string, i int, usageLine string) string {
	if i >= len(rest) {
		die(2, "usage: %s\n", usageLine)
	}
	return rest[i]
}

// parse runs fs.Parse and converts a parse error into a clean exit-2.
func parse(fs *flag.FlagSet, args []string) {
	if err := fs.Parse(args); err != nil {
		die(2, "%s: %v\n", fs.Name(), err)
	}
}

func orNow(at string) string { return docio.OrNow(at) }

func absClean(path string) string {
	if a, err := filepath.Abs(path); err == nil {
		return filepath.Clean(a)
	}
	return filepath.Clean(path)
}

func readFile(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		die(2, "cannot read %s: %v\n", path, err)
	}
	return b
}

func readPlan(path string) model.Plan {
	var p model.Plan
	if err := json.Unmarshal(readFile(path), &p); err != nil {
		die(2, "invalid plan JSON in %s: %v\n", path, err)
	}
	return p
}

// readExec loads execution.json and migrates it in place to the current schema version — the
// single, explicit upgrade step every command that reads execution state applies.
func readExec(path string) model.ExecState {
	e := readExecRaw(path)
	if err := execops.MigrateSchema(&e); err != nil {
		die(2, "%v\n", err)
	}
	return e
}

// readExecRaw loads execution.json WITHOUT the on-load schema migration readExec performs.
// Only migrate-project uses it: it must see the true on-disk schema_version.
func readExecRaw(path string) model.ExecState {
	var e model.ExecState
	if err := json.Unmarshal(readFile(path), &e); err != nil {
		die(2, "invalid execution JSON in %s: %v\n", path, err)
	}
	return e
}

// readArchive loads archive.json, tolerating an absent file (a project's first-ever archive
// call) as an empty archive doc rather than an IO error.
func readArchive(path string) model.ArchiveDoc {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.ArchiveDoc{Schema: model.ArchiveSchema}
		}
		die(2, "cannot read %s: %v\n", path, err)
	}
	var a model.ArchiveDoc
	if err := json.Unmarshal(b, &a); err != nil {
		die(2, "invalid archive JSON in %s: %v\n", path, err)
	}
	return a
}

// readFeedback loads feedback.json, tolerating an absent file as an empty, freshly-schema-
// stamped register rather than an IO error.
func readFeedback(path string) model.FeedbackRegister {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.FeedbackRegister{Schema: model.FeedbackSchema}
		}
		die(2, "cannot read %s: %v\n", path, err)
	}
	var r model.FeedbackRegister
	if err := json.Unmarshal(b, &r); err != nil {
		die(2, "invalid feedback JSON in %s: %v\n", path, err)
	}
	return r
}

func printJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		die(2, "cannot encode output: %v\n", err)
	}
	fmt.Println(string(b))
}

// writeJSONFile marshals v and durably writes it to path.
func writeJSONFile(path string, v any) {
	if err := docio.WriteJSON(path, v, docio.DefaultPerm); err != nil {
		die(2, "cannot write %s: %v\n", path, err)
	}
}

// writeTextFile durably writes Markdown content to path.
func writeTextFile(path, content string) {
	if err := docio.WriteText(path, content); err != nil {
		die(2, "cannot write %s: %v\n", path, err)
	}
}

// feedbackMirrorPath derives feedback.md's path from feedback.json's path.
func feedbackMirrorPath(jsonPath string) string {
	return jsonPath[:len(jsonPath)-len(filepath.Ext(jsonPath))] + ".md"
}
