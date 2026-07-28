package classify

// Trigger is a member of the closed named set the magistrate judges. Membership is by exact
// string; the set is exactly the four consts below and nothing else.
type Trigger string

const (
	TriggerSurpriseOverlap     Trigger = "surprise-overlap"
	TriggerLocalDeltaReplan    Trigger = "local-delta-replan"
	TriggerFailedTaskTriage    Trigger = "failed-task-triage"
	TriggerPhaseGateRegression Trigger = "phase-gate-regression"
)

// tiers is the CLOSED trigger set AND each trigger's classifier-selected magistrate effort
// tier — the SOLE gate to the magistrate: exact-string lookup, no default branch.
var tiers = map[Trigger]string{
	TriggerSurpriseOverlap:     "xhigh",
	TriggerFailedTaskTriage:    "xhigh",
	TriggerLocalDeltaReplan:    "high",
	TriggerPhaseGateRegression: "high",
}

// Triggers returns the closed named set in a stable order (descending tier weight, then name).
func Triggers() []Trigger {
	return []Trigger{TriggerFailedTaskTriage, TriggerSurpriseOverlap, TriggerLocalDeltaReplan, TriggerPhaseGateRegression}
}

// Routes. Distinct from the front-door Route* constants.
const (
	RouteMagistrate   = "magistrate"
	RoutePlanWithTeam = "plan-with-team"
	RouteNoEscalation = "no-escalation"
	RouteInScope      = "in-scope"
)

// ScopeInput is the orchestrator's observation of whether a proposed mid-build delta touches
// the design's success_criteria or scope.
type ScopeInput struct {
	TouchesSuccessCriteria bool
	TouchesScope           bool
}

// ScopeResult is the deterministic scope route.
type ScopeResult struct {
	TouchesSuccessCriteria bool   `json:"touches_success_criteria"`
	TouchesScope           bool   `json:"touches_scope"`
	DesignLevel            bool   `json:"design_level"`
	Route                  string `json:"route"`
	Reason                 string `json:"reason"`
}

// Scope routes a mid-build delta by whether it touches success_criteria/scope. Either -> pause
// + hand back to plan-with-team; neither -> in-scope.
func Scope(in ScopeInput) ScopeResult {
	r := ScopeResult{TouchesSuccessCriteria: in.TouchesSuccessCriteria, TouchesScope: in.TouchesScope}
	if in.TouchesSuccessCriteria || in.TouchesScope {
		r.DesignLevel = true
		r.Route = RoutePlanWithTeam
		r.Reason = "design-level delta (touches success_criteria/scope) -> pause + hand back to plan-with-team"
		return r
	}
	r.Route = RouteInScope
	r.Reason = "delta does not touch design success_criteria/scope -> in-scope"
	return r
}

// EscalationInput is one observed condition to classify.
type EscalationInput struct {
	Condition              string
	TouchesSuccessCriteria bool
	TouchesScope           bool
}

// EscalationResult is the deterministic escalation route. Trigger/Tier are set only for the
// magistrate route.
type EscalationResult struct {
	Condition   string  `json:"condition"`
	DesignLevel bool    `json:"design_level"`
	Route       string  `json:"route"`
	Trigger     Trigger `json:"trigger,omitempty"`
	Tier        string  `json:"tier,omitempty"`
	Reason      string  `json:"reason"`
}

// Escalate routes an observed condition to exactly one of: magistrate, plan-with-team, or
// no-escalation. The design-level override runs first and is absolute — it can never be
// overridden by a condition string that also names a trigger.
func Escalate(in EscalationInput) EscalationResult {
	r := EscalationResult{Condition: in.Condition}
	if scope := Scope(ScopeInput{TouchesSuccessCriteria: in.TouchesSuccessCriteria, TouchesScope: in.TouchesScope}); scope.DesignLevel {
		r.DesignLevel = true
		r.Route = RoutePlanWithTeam
		r.Reason = scope.Reason
		return r
	}
	if tier, ok := tiers[Trigger(in.Condition)]; ok {
		r.Route = RouteMagistrate
		r.Trigger = Trigger(in.Condition)
		r.Tier = tier
		r.Reason = "named escalation trigger -> fire magistrate"
		return r
	}
	r.Route = RouteNoEscalation
	r.Reason = "out-of-set condition -> orchestrator handles locally; no magistrate"
	return r
}

// Feedback criticality routes.
const (
	RouteFeedbackAmend  = "feedback-amend"
	RouteFeedbackReview = "feedback-review"
)

// FeedbackCriticality routes one entry's criticality against the amend-now threshold
// (inclusive floor): criticality >= threshold amends now, strictly below defers.
func FeedbackCriticality(criticality, threshold int) string {
	if criticality >= threshold {
		return RouteFeedbackAmend
	}
	return RouteFeedbackReview
}
