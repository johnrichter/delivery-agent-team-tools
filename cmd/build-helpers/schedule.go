package main

import (
	"flag"
	"io"
	"os"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
	"github.com/johnrichter/delivery-agent-team-tools/internal/planops"
	"github.com/johnrichter/delivery-agent-team-tools/internal/schedule"
	"github.com/johnrichter/delivery-agent-team-tools/internal/surface"
)

func runNext(rest []string) {
	ex := readExec(arg(rest, 0, "next <execution.json> <plan.json>"))
	plan := readPlan(arg(rest, 1, "next <execution.json> <plan.json>"))
	res := schedule.Next(ex, plan)
	printJSON(res)
	// Structural refusal: a non-nil OrchestratorOnly is a hard error, never a prose caveat a
	// caller could ignore — the orchestrator must run that task inline, not via this dispatch path.
	exitOK(res.OrchestratorOnly == nil)
}

func runBatch(rest []string) {
	pos := positionals(rest, 2, "batch <execution.json> <plan.json> [--max N]")
	fs := flag.NewFlagSet("batch", flag.ContinueOnError)
	max := fs.Int("max", 4, "max tasks to dispatch in one fan-out (default 4; hard cap 8)")
	parse(fs, rest[2:])
	if *max > schedule.MaxBatch {
		*max = schedule.MaxBatch
	}
	ex := readExec(pos[0])
	plan := readPlan(pos[1])
	res := schedule.Batch(ex, plan, *max)
	printJSON(res)
	exitOK(res.OrchestratorOnly == nil)
}

func runVerifySurface(rest []string) {
	pos := positionals(rest, 2, "verify-surface <plan.json> <taskId>[,<taskId>…] [--root DIR]")
	fs := flag.NewFlagSet("verify-surface", flag.ContinueOnError)
	root := fs.String("root", ".", "directory the file_surface paths are relative to")
	parse(fs, rest[2:])
	plan := readPlan(pos[0])
	var perTaskSurfaces [][]model.FileSurfaceEntry
	for _, id := range strings.Split(pos[1], ",") {
		id = strings.TrimSpace(id)
		r, found := planops.TaskByID(plan, id)
		if !found {
			die(2, "verify-surface: task %q not found in %s\n", id, pos[0])
		}
		perTaskSurfaces = append(perTaskSurfaces, r.Task.FileSurface)
	}
	res := surface.VerifyMerged(os.DirFS(*root), perTaskSurfaces)
	printJSON(res)
	exitOK(res.OK)
}

func runCheckChangedSurface(rest []string) {
	pos := positionals(rest, 2, "check-changed-surface <plan.json> <taskId> --changed FILE|-")
	fs := flag.NewFlagSet("check-changed-surface", flag.ContinueOnError)
	changed := fs.String("changed", "", "path to a newline-delimited changed-set, or - for stdin (required)")
	parse(fs, rest[2:])
	if *changed == "" {
		die(2, "check-changed-surface: --changed is required\n")
	}
	plan := readPlan(pos[0])
	r, found := planops.TaskByID(plan, pos[1])
	if !found {
		die(2, "check-changed-surface: task %q not found in %s\n", pos[1], pos[0])
	}
	var raw []byte
	if *changed == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			die(2, "check-changed-surface: cannot read stdin: %v\n", err)
		}
		raw = b
	} else {
		raw = readFile(*changed)
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	res := surface.VerifyChangedSubsetOfSurface(lines, r.Task.FileSurface)
	printJSON(res)
	exitOK(res.OK)
}
