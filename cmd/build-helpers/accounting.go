package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/johnrichter/delivery-agent-team-tools/internal/accounting"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// defaultSpecsPath locates anthropic-specifications.json relative to the executable, falling
// back to a cwd-relative path. Override with --specs.
func defaultSpecsPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "..", "..", "anthropic-specifications.json")
	}
	return "anthropic-specifications.json"
}

// pricingAvailable reports whether specsPath names a readable, valid specifications document —
// the on/off gate for cost math (buckets are always counted for Turns regardless) — plus its
// pinned `_as_of` provenance date.
func pricingAvailable(specsPath string) (priced bool, specsAsOf string) {
	b, err := os.ReadFile(specsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build-helpers: specs %s unreadable, recording token buckets without cost: %v\n", specsPath, err)
		return false, ""
	}
	var doc struct {
		Pricing struct {
			List map[string]json.RawMessage `json:"list"`
		} `json:"pricing"`
	}
	if err := json.Unmarshal(b, &doc); err != nil || len(doc.Pricing.List) == 0 {
		fmt.Fprintf(os.Stderr, "build-helpers: specs %s invalid or has no priced models, recording token buckets without cost\n", specsPath)
		return false, accounting.SpecsAsOf(b)
	}
	return true, accounting.SpecsAsOf(b)
}

// engineSHA returns the sha256 of the currently-running executable (hex-encoded), pinning
// which binary produced a given accounting snapshot. Best-effort: an unresolvable/unreadable
// executable path yields "" rather than an error.
func engineSHA() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

func runUsage(path string) model.Usage {
	paths, resolved := accounting.DiscoverSession(path)
	if !resolved {
		die(2, "cannot read transcript %s\n", path)
	}
	sources := accounting.OpenSources(paths, nil)
	defer accounting.CloseAll(sources)
	acct, err := accounting.Account(nil, sources, false, "")
	if err != nil {
		die(2, "parse transcript %s: %v\n", path, err)
	}
	return accounting.Flatten(acct.Models)
}

func runRecordUsage(rest []string) {
	pos := positionals(rest, 1, "record-usage <execution.json> --transcript <path> [--specs P --final --baseline-capture --at …]")
	fs := flag.NewFlagSet("record-usage", flag.ContinueOnError)
	transcriptPath := fs.String("transcript", "", "main session transcript JSONL path (required)")
	specs := fs.String("specs", "", "anthropic-specifications.json path")
	final := fs.Bool("final", false, "finish-time authoritative snapshot: re-parse every transcript in full")
	baselineCapture := fs.Bool("baseline-capture", false, "this run IS a baseline-capture measurement")
	at := fs.String("at", "", "timestamp override (default: now UTC)")
	parse(fs, rest[1:])
	if *transcriptPath == "" {
		die(2, "record-usage: --transcript is required\n")
	}
	ex := readExec(pos[0])
	when := orNow(*at)

	paths, resolved := accounting.DiscoverSession(*transcriptPath)
	if !resolved {
		if *baselineCapture {
			die(1, "record-usage: baseline-capture run failed — main transcript %s unresolved\n", *transcriptPath)
		}
		accounting.SetUnresolved(&ex, *transcriptPath, when)
		printJSON(ex)
		return
	}

	var priorWatermarks map[string]int64
	if !*final && ex.RunConfig.Accounting != nil {
		priorWatermarks = ex.RunConfig.Accounting.Watermarks
	}
	var prior *model.Accounting
	if !*final {
		prior = ex.RunConfig.Accounting
	}

	specsPath := *specs
	if specsPath == "" {
		specsPath = defaultSpecsPath()
	}
	priced, specsAsOf := pricingAvailable(specsPath)

	mainAbs := absClean(*transcriptPath)
	sources := accounting.OpenSources(paths, priorWatermarks)
	defer accounting.CloseAll(sources)
	acct, err := accounting.Account(prior, sources, priced, when)
	if err != nil {
		die(2, "record-usage: parse transcript %s: %v\n", *transcriptPath, err)
	}
	accounting.SetAccounting(&ex, acct, mainAbs, priced, *final, when, specsAsOf, engineSHA())
	printJSON(ex)
}

func runAttribute(rest []string) {
	pos := positionals(rest, 1, "attribute <execution.json> --transcript <path> [--tasks id,id,… --specs P --at …]")
	fs := flag.NewFlagSet("attribute", flag.ContinueOnError)
	transcriptPath := fs.String("transcript", "", "main session transcript JSONL path (required)")
	tasks := fs.String("tasks", "", "comma-separated known task IDs to match against")
	specs := fs.String("specs", "", "anthropic-specifications.json path")
	at := fs.String("at", "", "timestamp override (default: now UTC)")
	parse(fs, rest[1:])
	if *transcriptPath == "" {
		die(2, "attribute: --transcript is required\n")
	}
	ex := readExec(pos[0])

	known := splitTasks(*tasks)
	if len(known) == 0 {
		for _, t := range ex.Tasks {
			known = append(known, t.ID)
		}
	}
	specsPath := *specs
	if specsPath == "" {
		specsPath = defaultSpecsPath()
	}
	priced, _ := pricingAvailable(specsPath)

	paths := accounting.DiscoverSubagents(*transcriptPath)
	var sources []accounting.AttribSource
	var handles []*os.File
	for _, p := range paths {
		fh, err := os.Open(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build-helpers: skipping unreadable transcript %s: %v\n", p, err)
			continue
		}
		handles = append(handles, fh)
		sources = append(sources, accounting.AttribSource{FileID: p, Reader: fh})
	}
	defer func() {
		for _, h := range handles {
			_ = h.Close()
		}
	}()

	attr, err := accounting.Attribute(sources, known, priced, orNow(*at))
	if err != nil {
		die(2, "attribute: parse transcripts under %s: %v\n", *transcriptPath, err)
	}
	printJSON(attr)
}
