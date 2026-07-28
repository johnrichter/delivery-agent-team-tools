package planops

import (
	"encoding/json"
	"testing"
)

// TestValidatePlanBytes_ErrorContentIsFrozen locks in the exact error/warning strings validate
// emits for a known-invalid plan — these strings are part of the CLI's frozen stdout contract
// (SC-DAT-FROZEN), not an implementation detail: a generic validation-library message, or an
// inconsistent "task " prefix across entries, is a content regression even though the JSON
// shape ({ok,errors,warnings}) stays intact.
func TestValidatePlanBytes_ErrorContentIsFrozen(t *testing.T) {
	const invalid = `{
		"goal": "",
		"success_criteria": [],
		"milestones": [{
			"id": "M1", "name": "m1",
			"phases": [{
				"id": "M1.P1", "name": "p1",
				"tasks": [{
					"id": "M1.P1.T1", "name": "t1",
					"summary": "", "deliverable": "",
					"model": "claude-sonnet-5", "effort": "medium",
					"test_strategy": "", "acceptance": []
				}]
			}]
		}]
	}`
	res := ValidatePlanBytes([]byte(invalid))
	if res.OK {
		t.Fatal("expected ok:false for an invalid plan")
	}
	wantErrors := []string{
		"goal: required non-empty string",
		"success_criteria: required array of ≥1 strings",
		"task M1.P1.T1: summary required",
		"task M1.P1.T1: deliverable required",
		"task M1.P1.T1: test_strategy required",
		"task M1.P1.T1: acceptance array of ≥1 required",
	}
	if !equalStrings(res.Errors, wantErrors) {
		t.Fatalf("errors = %q\nwant    %q", res.Errors, wantErrors)
	}
	wantWarnings := []string{
		"task M1.P1.T1: file_surface absent on a code task — parallel overlap checks may not be possible",
	}
	if !equalStrings(res.Warnings, wantWarnings) {
		t.Fatalf("warnings = %q\nwant      %q", res.Warnings, wantWarnings)
	}
}

// TestValidatePlanBytes_UnknownRootKeyIsWarningOnly confirms an unrecognized top-level key
// never flips ok to false — a plan authored against a newer/older schema revision must still
// validate, per the golden fixture's exit-code legend.
func TestValidatePlanBytes_UnknownRootKeyIsWarningOnly(t *testing.T) {
	const raw = `{
		"goal": "g", "success_criteria": ["c"], "milestones": [{
			"id": "M1", "name": "m1", "phases": [{
				"id": "M1.P1", "name": "p1", "tasks": [{
					"id": "M1.P1.T1", "name": "t1", "summary": "s", "deliverable": "d",
					"model": "claude-sonnet-5", "effort": "medium", "test_strategy": "t",
					"acceptance": ["a"], "file_surface": [{"path": "x.go"}]
				}]
			}]
		}],
		"future_field": true
	}`
	res := ValidatePlanBytes([]byte(raw))
	if !res.OK {
		t.Fatalf("expected ok:true, got errors=%q", res.Errors)
	}
	want := "unknown root key 'future_field' (not in plan schema)"
	if !equalStrings(res.Warnings, []string{want}) {
		t.Fatalf("warnings = %q, want [%q]", res.Warnings, want)
	}
}

// TestValidatePlanBytes_EmptyMilestonesShortCircuits confirms an empty milestones array stops
// at that one error — the integrity/tier passes below it never run — matching the frozen
// engine's short-circuit behavior rather than piling on cascading errors from an empty plan.
func TestValidatePlanBytes_EmptyMilestonesShortCircuits(t *testing.T) {
	res := ValidatePlanBytes([]byte(`{"goal":"g","success_criteria":["c"],"milestones":[]}`))
	want := []string{"milestones: required array of ≥1"}
	if !equalStrings(res.Errors, want) {
		t.Fatalf("errors = %q, want %q", res.Errors, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestValidatePlanBytes_MalformedJSON confirms malformed JSON is an ok:false error, not a
// caller-visible panic or a decode-time crash — main.go relies on this to keep validate's
// exit-2 path scoped to a missing/unreadable file, never a malformed one (see surface.json).
func TestValidatePlanBytes_MalformedJSON(t *testing.T) {
	res := ValidatePlanBytes([]byte(`{not json`))
	if res.OK {
		t.Fatal("expected ok:false for malformed JSON")
	}
	if len(res.Errors) != 1 {
		t.Fatalf("errors = %v, want exactly one", res.Errors)
	}
}

// TestValidatePlanBytes_RoundTripsPlanJSON is a light sanity check that decoding still works
// through the model package after the schema-library removal.
func TestValidatePlanBytes_RoundTripsPlanJSON(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"goal": "g", "success_criteria": []string{"c"}, "milestones": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	if ValidatePlanBytes(raw).OK {
		t.Fatal("empty milestones must never validate ok:true")
	}
}
