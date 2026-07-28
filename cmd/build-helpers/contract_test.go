package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// contractCommand is the subset of contract-fixtures/surface.json's per-command fields this
// test consumes — a dispatch/argv-shape check, not a re-authoring of the fixture's own
// documentation (invoked_by, notes, exit_codes prose, etc.).
type contractCommand struct {
	Name        string `json:"name"`
	Positionals []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	} `json:"positionals"`
}

type contractSurface struct {
	Commands []contractCommand `json:"commands"`
}

// loadSurface reads the golden contract fixture SC-DAT-FROZEN pins. It lives at the repo root
// (contract-fixtures/), a sibling of cmd/ — a within-module path, stable regardless of where
// the whole repo checkout is rooted on disk.
func loadSurface(t *testing.T) contractSurface {
	t.Helper()
	raw, err := os.ReadFile("../../contract-fixtures/surface.json")
	if err != nil {
		t.Fatalf("read golden contract fixture: %v", err)
	}
	var s contractSurface
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse golden contract fixture: %v", err)
	}
	if len(s.Commands) == 0 {
		t.Fatal("golden contract fixture has no commands — cannot verify dispatch completeness")
	}
	return s
}

var (
	binOnce sync.Once
	binPath string
)

// buildBinary compiles cmd/build-helpers once per test run and reuses it across subtests.
func buildBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "build-helpers-contract-*")
		if err != nil {
			t.Fatalf("tempdir: %v", err)
		}
		binPath = filepath.Join(dir, "build-helpers")
		cmd := exec.Command("go", "build", "-o", binPath, ".")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build ./cmd/build-helpers: %v\n%s", err, out)
		}
	})
	return binPath
}

// TestSurface_DispatchCompleteness proves every subcommand name the golden fixture pins is
// still wired into main's dispatch table. main's ONE unrecognized-command message
// ("unknown command %q") is textually distinct from every recognized command's own usage
// error, so a name silently dropped from the switch is the one failure mode this test cannot
// miss — the forces_pause tripwire for SC-DAT-FROZEN: a missing subcommand breaks every live
// call site the fixture's invoked_by field cites.
func TestSurface_DispatchCompleteness(t *testing.T) {
	bin := buildBinary(t)
	for _, c := range loadSurface(t).Commands {
		if c.Name == "help" {
			continue // "help"/-h/--help/bare invocation is its own documented usage path, not a dispatch case
		}
		top := strings.Fields(c.Name)[0] // "feedback add" -> "feedback" (main dispatches on the top-level verb)
		t.Run(c.Name, func(t *testing.T) {
			cmd := exec.Command(bin, top)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			_ = cmd.Run() // a missing-positional/flag exit is expected here; only dispatch is under test
			if strings.Contains(stderr.String(), "unknown command") {
				t.Fatalf("dispatch table no longer recognizes %q: %s", top, stderr.String())
			}
		})
	}
}

// TestSurface_MissingPositionalExitsUsage cross-checks the golden fixture's own exit-code
// legend (2 == usage/IO error, including a missing positional) against every subcommand the
// fixture declares at least one required positional for: invoking it bare must exit 2, never 0
// or 1 — the latter would mean a positional silently became optional.
func TestSurface_MissingPositionalExitsUsage(t *testing.T) {
	bin := buildBinary(t)
	for _, c := range loadSurface(t).Commands {
		requiresPositional := false
		for _, p := range c.Positionals {
			requiresPositional = requiresPositional || p.Required
		}
		if !requiresPositional {
			continue
		}
		top := strings.Fields(c.Name)[0]
		t.Run(c.Name, func(t *testing.T) {
			err := exec.Command(bin, top).Run()
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("%q with no args: want a non-zero exit, got err=%v", top, err)
			}
			if code := exitErr.ExitCode(); code != 2 {
				t.Fatalf("%q with no args: exit %d, want 2 per the golden exit-code legend", top, code)
			}
		})
	}
}

// TestSurface_JSONShapes spot-checks the top-level key set of a representative sample of the
// fixture's JSON-emitting subcommands against a minimal valid plan — the "schema unchanged"
// half of SC-DAT-FROZEN for the commands most load-bearing to plan-with-team/build-with-team.
func TestSurface_JSONShapes(t *testing.T) {
	bin := buildBinary(t)
	plan := "testdata/plan-minimal.json"

	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"validate", []string{"validate", plan}, []string{"ok", "errors", "warnings"}},
		{"check-tiers", []string{"check-tiers", plan}, []string{"ok", "issues"}},
		{"hash", []string{"hash", plan}, nil}, // {taskId: hash, ...} — no fixed key set, checked separately below
		{"classify", []string{"classify", "testdata/does-not-exist"}, []string{"design", "plan", "execution", "route"}},
		{"retrieve-outline", []string{"retrieve", plan, "--level", "outline"}, nil}, // array, checked separately below
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := exec.Command(bin, tc.args...).Output()
			if tc.want == nil {
				return
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(out, &m); err != nil {
				t.Fatalf("%s: stdout not a JSON object: %v\nstdout: %s", tc.name, err, out)
			}
			for _, k := range tc.want {
				if _, ok := m[k]; !ok {
					t.Fatalf("%s: stdout missing key %q\nstdout: %s", tc.name, k, out)
				}
			}
		})
	}

	// hash: {taskId: contentHash, ...} — every value a hex string.
	hashOut, err := exec.Command(bin, "hash", plan).Output()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	var hashes map[string]string
	if err := json.Unmarshal(hashOut, &hashes); err != nil {
		t.Fatalf("hash: stdout not {taskId: hash}: %v\nstdout: %s", err, hashOut)
	}
	if hashes["M1.P1.T1"] == "" {
		t.Fatalf("hash: missing or empty hash for M1.P1.T1: %v", hashes)
	}

	// retrieve --level outline: a JSON array.
	outlineOut, err := exec.Command(bin, "retrieve", plan, "--level", "outline").Output()
	if err != nil {
		t.Fatalf("retrieve outline: %v", err)
	}
	var outline []json.RawMessage
	if err := json.Unmarshal(outlineOut, &outline); err != nil {
		t.Fatalf("retrieve outline: stdout not a JSON array: %v\nstdout: %s", err, outlineOut)
	}
}

// TestSurface_SelfCheckSuccessPath exercises self-check's success and abort paths — the half of
// SC-DAT-FROZEN the dispatch/missing-positional checks never reach (they only prove bare
// invocation errors out). The fixture transcript deliberately OMITS the isSidechain marker:
// model detection must key off the last line NAMING a model, not off orchestrator-authorship
// resolution — a stricter authorship gate silently fails to detect a model on any transcript
// that lacks the marker, aborting self-check (exit 2) where the frozen surface emits a verdict.
func TestSurface_SelfCheckSuccessPath(t *testing.T) {
	bin := buildBinary(t)
	tx := "testdata/selfcheck/transcript.jsonl"
	settings := "testdata/selfcheck/settings.json"

	// In-band success: exit 0, full verdict JSON, model resolved to the LAST line's model.
	out, err := exec.Command(bin, "self-check",
		"--floor-model", "claude-sonnet-4-5", "--floor-effort", "low",
		"--ceiling-model", "claude-sonnet-5", "--ceiling-effort", "high",
		"--transcript", tx, "--settings", settings,
	).Output()
	if err != nil {
		t.Fatalf("self-check in-band: want exit 0, got err=%v\nstdout: %s", err, out)
	}
	var res struct {
		Model          string `json:"model"`
		Effort         string `json:"effort"`
		EffortDetected bool   `json:"effort_detected"`
		Abort          bool   `json:"abort"`
		Reason         string `json:"reason"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("self-check: stdout not the frozen verdict shape: %v\nstdout: %s", err, out)
	}
	if res.Model != "claude-sonnet-5" {
		t.Fatalf("self-check: model = %q, want claude-sonnet-5 (last transcript line, isSidechain absent)", res.Model)
	}
	if res.Abort {
		t.Fatalf("self-check in-band: abort=true, want false\nstdout: %s", out)
	}

	// Identity-guard mismatch: exit 1, and the reason quotes both session ids (%q) — the frozen
	// message format live callers surface verbatim.
	mm := exec.Command(bin, "self-check",
		"--band", "build", "--transcript", tx, "--settings", settings,
		"--session-id", "99999999-2222-3333-4444-555555555555",
	)
	mmOut, mmErr := mm.Output()
	exitErr, ok := mmErr.(*exec.ExitError)
	if !ok {
		t.Fatalf("self-check identity mismatch: want exit 1, got err=%v\nstdout: %s", mmErr, mmOut)
	}
	if code := exitErr.ExitCode(); code != 1 {
		t.Fatalf("self-check identity mismatch: exit %d, want 1 (abort)", code)
	}
	var mmRes struct {
		SessionIDChecked bool   `json:"session_id_checked"`
		SessionIDMatch   bool   `json:"session_id_match"`
		Reason           string `json:"reason"`
	}
	if err := json.Unmarshal(mmOut, &mmRes); err != nil {
		t.Fatalf("self-check mismatch: stdout not JSON: %v\nstdout: %s", err, mmOut)
	}
	if !mmRes.SessionIDChecked || mmRes.SessionIDMatch {
		t.Fatalf("self-check mismatch: session_id_checked/match = %v/%v, want true/false", mmRes.SessionIDChecked, mmRes.SessionIDMatch)
	}
	if !strings.Contains(mmRes.Reason, `"11111111-2222-3333-4444-555555555555"`) {
		t.Fatalf("self-check mismatch: reason must quote the transcript session id (%%q format), got: %s", mmRes.Reason)
	}
}

// accountingCostTolerance is the absolute-or-relative dollar tolerance golden accounting
// numbers are checked against, mirroring the identity check's own convention
// (internal/accounting): float64 summation noise sits many orders of magnitude below this, so
// a real pricing regression (wrong rate, dropped bucket, wrong model split) cannot hide inside
// it, while a rebuild's incidental float rounding never trips it.
const accountingCostTolerance = 1e-6

func withinTolerance(got, want float64) bool {
	return math.Abs(got-want) <= math.Max(accountingCostTolerance, accountingCostTolerance*math.Abs(want))
}

// TestAccounting_UsageAndCostWithinTolerance runs `usage` and `record-usage` against a fixed
// transcript corpus (whole-session grand total: orchestrator + 4 subagent transcripts,
// testdata/accounting/) and checks the results against hand-verified totals — SC-DAT-FROZEN's
// numeric-tolerance half. Token counts are exact-integer checks (no tolerance needed); dollar
// costs — derived from the shared model roster's rate table — are checked within
// accountingCostTolerance rather than byte-for-byte, since the roster is free to gain a new
// rate-table format revision without this test caring, as long as the priced total does not
// move by more than a rounding-noise amount.
func TestAccounting_UsageAndCostWithinTolerance(t *testing.T) {
	bin := buildBinary(t)

	usageOut, err := exec.Command(bin, "usage", "testdata/accounting/orchestrator.jsonl").Output()
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	var usage struct {
		InputTokens  int64 `json:"input_tokens"`
		CacheCreate  int64 `json:"cache_creation_input_tokens"`
		CacheRead    int64 `json:"cache_read_input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
		TotalTokens  int64 `json:"total_tokens"`
		Turns        int64 `json:"turns"`
	}
	if err := json.Unmarshal(usageOut, &usage); err != nil {
		t.Fatalf("usage: parse stdout: %v\nstdout: %s", err, usageOut)
	}
	// Grand total (orchestrator.jsonl + its 4 sibling subagent transcripts): input 38000,
	// cache_creation 14000 (c5m+c1h summed across both models), cache_read 9700, output 5600,
	// total 67300, 11 usage-bearing assistant turns. Independently hand-verified; see the
	// per-model breakdown this package's rate math replays below.
	wantUsage := struct {
		InputTokens  int64 `json:"input_tokens"`
		CacheCreate  int64 `json:"cache_creation_input_tokens"`
		CacheRead    int64 `json:"cache_read_input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
		TotalTokens  int64 `json:"total_tokens"`
		Turns        int64 `json:"turns"`
	}{38000, 14000, 9700, 5600, 67300, 11}
	if usage != wantUsage {
		t.Fatalf("usage: got %+v, want %+v", usage, wantUsage)
	}

	execFixture, err := os.ReadFile("testdata/exec-minimal.json")
	if err != nil {
		t.Fatalf("read exec fixture: %v", err)
	}
	execPath := filepath.Join(t.TempDir(), "execution.json")
	if err := os.WriteFile(execPath, execFixture, 0o644); err != nil {
		t.Fatalf("write exec fixture: %v", err)
	}

	ruOut, err := exec.Command(bin, "record-usage", execPath,
		"--transcript", "testdata/accounting/orchestrator.jsonl",
		"--specs", "testdata/accounting/specs.json",
		"--at", "2026-07-03T18:10:00Z",
	).Output()
	if err != nil {
		t.Fatalf("record-usage: %v", err)
	}
	var ex struct {
		RunConfig struct {
			Accounting struct {
				CostUSD     float64            `json:"cost_usd"`
				CostByModel map[string]float64 `json:"cost_by_model"`
			} `json:"accounting"`
		} `json:"run_config"`
	}
	if err := json.Unmarshal(ruOut, &ex); err != nil {
		t.Fatalf("record-usage: parse stdout: %v\nstdout: %s", err, ruOut)
	}

	// Grand-total per-model dollar cost, hand-verified against the model roster's list rates
	// (sonnet-5 3.00/3.75/6.00/0.30/15.00, opus-4-8 5.00/6.25/10.00/0.50/25.00 per MTok):
	// sonnet-5 (23000,5800,3900,3800,2400) -> $0.15129; opus-4-8 (15000,3100,1200,5900,3200)
	// -> $0.189325; total $0.340615.
	wantSonnet, wantOpus, wantTotal := 0.15129, 0.189325, 0.340615
	if got := ex.RunConfig.Accounting.CostByModel["claude-sonnet-5"]; !withinTolerance(got, wantSonnet) {
		t.Errorf("cost_by_model[claude-sonnet-5] = %v, want %v ± %v", got, wantSonnet, accountingCostTolerance)
	}
	if got := ex.RunConfig.Accounting.CostByModel["claude-opus-4-8"]; !withinTolerance(got, wantOpus) {
		t.Errorf("cost_by_model[claude-opus-4-8] = %v, want %v ± %v", got, wantOpus, accountingCostTolerance)
	}
	if got := ex.RunConfig.Accounting.CostUSD; !withinTolerance(got, wantTotal) {
		t.Errorf("cost_usd = %v, want %v ± %v", got, wantTotal, accountingCostTolerance)
	}
}
