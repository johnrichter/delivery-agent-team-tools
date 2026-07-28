// Package classify is the front-door router (design/plan/execution doc-state -> build route)
// plus the escalation/scope/feedback-criticality classifiers: deterministic detectors that
// decide when a specialist (the magistrate) or a design-level hand-back is warranted.
package classify

import (
	"regexp"
	"strings"
	"time"
)

var (
	reFrontmatter = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---`)
	reStatusTag   = regexp.MustCompile(`status:(stub|complete)`)
	reUpdated     = regexp.MustCompile(`(?m)^\s*updated:\s*(.+?)\s*$`)
)

// ParseDesignFrontmatter extracts the design's status tag (stub|complete) and updated
// timestamp from its YAML frontmatter.
func ParseDesignFrontmatter(text string) (status, updated string) {
	m := reFrontmatter.FindStringSubmatch(text)
	if m == nil {
		return "", ""
	}
	fm := m[1]
	if s := reStatusTag.FindStringSubmatch(fm); s != nil {
		status = s[1]
	}
	if u := reUpdated.FindStringSubmatch(fm); u != nil {
		updated = strings.Trim(u[1], `"'`)
	}
	return status, updated
}

// Input is the observed on-disk state of a project's docs.
type Input struct {
	DesignPresent               bool
	DesignText                  string
	PlanPresent                 bool
	PlanProvenanceDesignUpdated string
	ExecutionPresent            bool
	MirrorPresent               bool
}

type designState struct {
	Present bool   `json:"present"`
	Status  string `json:"status,omitempty"`
	Updated string `json:"updated,omitempty"`
}
type planState struct {
	Present                 bool   `json:"present"`
	ProvenanceDesignUpdated string `json:"provenance_design_updated,omitempty"`
}
type execStateInfo struct {
	Present       bool `json:"present"`
	MirrorPresent bool `json:"mirror_present"`
}

// Result is the deterministic front-door route plus the observed state it derives from.
type Result struct {
	Design    designState   `json:"design"`
	Plan      planState     `json:"plan"`
	Execution execStateInfo `json:"execution"`
	Route     string        `json:"route"`
}

// Front-door routes.
const (
	RouteInteractiveBuild = "interactive-build"
	RouteResumeDraft      = "resume-draft"
	RouteDerive           = "derive"
	RouteReconcile        = "reconcile"
	RouteReady            = "ready"
)

// Classify decides the route from observed doc state.
func Classify(in Input) Result {
	r := Result{}
	r.Design.Present = in.DesignPresent
	if in.DesignPresent {
		r.Design.Status, r.Design.Updated = ParseDesignFrontmatter(in.DesignText)
	}
	r.Plan.Present = in.PlanPresent
	r.Plan.ProvenanceDesignUpdated = in.PlanProvenanceDesignUpdated
	r.Execution.Present = in.ExecutionPresent
	r.Execution.MirrorPresent = in.MirrorPresent

	switch {
	case !r.Design.Present:
		r.Route = RouteInteractiveBuild
	case r.Design.Status == "stub":
		r.Route = RouteResumeDraft
	case !r.Plan.Present:
		r.Route = RouteDerive
	default:
		switch {
		case planIsStale(r.Design.Updated, r.Plan.ProvenanceDesignUpdated):
			r.Route = RouteReconcile
		case !r.Execution.Present:
			r.Route = RouteDerive
		default:
			r.Route = RouteReady
		}
	}
	return r
}

func planIsStale(designUpdated, planProvenance string) bool {
	if planProvenance == "" {
		return false
	}
	dt, derr := time.Parse(time.RFC3339, designUpdated)
	pt, perr := time.Parse(time.RFC3339, planProvenance)
	if derr == nil && perr == nil {
		return pt.Before(dt)
	}
	return designUpdated != planProvenance
}
