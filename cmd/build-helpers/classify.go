package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	"github.com/johnrichter/delivery-agent-team-tools/internal/classify"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

func runClassify(dir string) classify.Result {
	read := func(name string) (string, bool) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", false
		}
		return string(b), true
	}
	designText, designPresent := read("design.md")
	in := classify.Input{DesignPresent: designPresent, DesignText: designText}
	if planRaw, ok := read("plan.json"); ok {
		var p struct {
			Provenance *model.Provenance `json:"provenance"`
		}
		if json.Unmarshal([]byte(planRaw), &p) == nil {
			in.PlanPresent = true
			if p.Provenance != nil {
				in.PlanProvenanceDesignUpdated = p.Provenance.DesignUpdated
			}
		}
	}
	_, in.ExecutionPresent = read("execution.json")
	_, in.MirrorPresent = read("execution.md")
	return classify.Classify(in)
}

func runEscalate(rest []string) {
	fs := flag.NewFlagSet("escalate", flag.ContinueOnError)
	condition := fs.String("condition", "", "observed condition label (matched exactly against the closed trigger set)")
	touchesSC := fs.Bool("touches-success-criteria", false, "delta touches design success_criteria (forces plan-with-team hand-back)")
	touchesScope := fs.Bool("touches-scope", false, "delta touches design scope (forces plan-with-team hand-back)")
	parse(fs, rest)
	if *condition == "" {
		die(2, "escalate: --condition is required\n")
	}
	printJSON(classify.Escalate(classify.EscalationInput{
		Condition: *condition, TouchesSuccessCriteria: *touchesSC, TouchesScope: *touchesScope,
	}))
}

func runClassifyScope(rest []string) {
	fs := flag.NewFlagSet("classify-scope", flag.ContinueOnError)
	touchesSC := fs.Bool("touches-success-criteria", false, "delta touches design success_criteria")
	touchesScope := fs.Bool("touches-scope", false, "delta touches design scope")
	parse(fs, rest)
	printJSON(classify.Scope(classify.ScopeInput{TouchesSuccessCriteria: *touchesSC, TouchesScope: *touchesScope}))
}
