// Package selfcheck is the session-tier self-check (live model/effort against a floor/ceiling
// band) plus the deterministic transcript resolver and its session-id identity guard.
package selfcheck

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/johnrichter/claude-shared-tooling/go/bandcheck"
	"github.com/johnrichter/claude-shared-tooling/go/gate"
	"github.com/johnrichter/claude-shared-tooling/go/roster"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// TierBand is a caller-supplied floor/ceiling pair. Both ends are inclusive: at-floor and
// at-ceiling are within band.
type TierBand struct {
	FloorModel    model.Model
	FloorEffort   model.Effort
	CeilingModel  model.Model
	CeilingEffort model.Effort
}

// namedBands are the roster-resolved bands --band may select in place of the four literal
// --floor-*/--ceiling-* flags.
var namedBands = map[string]TierBand{
	"plan":  {FloorModel: "claude-opus-4-8", FloorEffort: model.EffortHigh, CeilingModel: "claude-opus-4-8", CeilingEffort: model.EffortMax},
	"build": {FloorModel: "claude-sonnet-5", FloorEffort: model.EffortMedium, CeilingModel: "claude-sonnet-5", CeilingEffort: model.EffortHigh},
}

// ResolveBand looks up a named band.
func ResolveBand(name string) (TierBand, bool) {
	b, ok := namedBands[name]
	return b, ok
}

// Result is the self-check verdict.
type Result struct {
	Model            model.Model  `json:"model"`
	Effort           model.Effort `json:"effort,omitempty"`
	EffortDetected   bool         `json:"effort_detected"`
	Abort            bool         `json:"abort"`
	RosterStale      bool         `json:"roster_stale,omitempty"`
	Warnings         []string     `json:"warnings,omitempty"`
	Reason           string       `json:"reason"`
	SessionIDChecked bool         `json:"session_id_checked,omitempty"`
	SessionIDMatch   bool         `json:"session_id_match,omitempty"`
}

// Check enforces band on the observed (model, effort) session tier, ordering solely through
// bandcheck.SelfCheck (roster.Compare underneath — no hardcoded rank table). When effort is
// undetected, only the model dimension is enforced and a warning always notes the skip.
func Check(observedModel model.Model, effort model.Effort, effortDetected bool, band TierBand) Result {
	bcBand := bandcheck.TierBand{
		FloorModel: string(band.FloorModel), FloorEffort: roster.Effort(band.FloorEffort),
		CeilingModel: string(band.CeilingModel), CeilingEffort: roster.Effort(band.CeilingEffort),
	}
	bc := bandcheck.SelfCheck(string(observedModel), roster.Effort(effort), effortDetected, bcBand)

	r := Result{Model: model.Model(bc.Model), Effort: model.Effort(bc.Effort), EffortDetected: bc.EffortDetected, Reason: bc.Reason}
	if bc.RosterStale {
		r.RosterStale = true
		return r
	}
	if !effortDetected {
		r.Warnings = append(r.Warnings, "effort undetectable ($CLAUDE_EFFORT and settings.json effortLevel both absent) -- enforcing the model band only")
	}
	switch bc.Verdict {
	case gate.VerdictAbort:
		r.Abort = true
	case gate.VerdictWarn:
		r.Warnings = append(r.Warnings, bc.Reason)
	}
	return r
}

// modelProbe reads just the model off one transcript line, preferring the message-nested shape
// (the common case) over a top-level model field.
type modelProbe struct {
	Model   string `json:"model"`
	Message *struct {
		Model string `json:"model"`
	} `json:"message"`
}

func (p modelProbe) resolve() string {
	if p.Message != nil && p.Message.Model != "" {
		return p.Message.Model
	}
	return p.Model
}

// LatestTranscriptModel resolves the live session model from the LAST transcript line naming one
// — the model source; a mid-session /model override is reflected here, unlike the launch-time
// $ANTHROPIC_MODEL setting. Best-effort: malformed/empty lines are skipped; ok is false when no
// line names a model. Every line naming a model is a candidate — this does NOT restrict to
// orchestrator-authored lines, because Claude Code transcripts do not universally carry the
// isSidechain marker that authorship resolution keys on, and requiring it would fail to detect a
// model on any transcript that omits it.
func LatestTranscriptModel(r io.Reader) (m model.Model, ok bool) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var p modelProbe
		if json.Unmarshal(line, &p) != nil {
			continue
		}
		if id := p.resolve(); id != "" {
			m, ok = model.Model(id), true
		}
	}
	return m, ok
}

// ParseSettingsEffort extracts effortLevel from Claude Code settings.json bytes.
func ParseSettingsEffort(raw []byte) (model.Effort, bool) {
	e, ok := bandcheck.ParseSettingsEffort(raw)
	return model.Effort(e), ok
}

// sessionIDLineProbe reads just the sessionId off one raw transcript line, for the fallback
// scan LatestTranscriptSessionID uses to count malformed/parseable lines transcript.Turns
// itself does not surface as a count.
type sessionIDLineProbe struct {
	SessionID string `json:"sessionId"`
}

// SessionIDScan is the outcome of scanning a transcript for the session identity its lines
// carry.
type SessionIDScan struct {
	SessionID string
	Lines     int
	Parsed    int
}

// Found reports whether any line named a non-empty sessionId.
func (s SessionIDScan) Found() bool { return s.SessionID != "" }

// LatestTranscriptSessionID scans a transcript and returns the sessionId of the LAST line that
// names one, tolerating malformed lines.
func LatestTranscriptSessionID(r io.Reader) SessionIDScan {
	var scan SessionIDScan
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		scan.Lines++
		var p sessionIDLineProbe
		if json.Unmarshal(line, &p) != nil {
			continue
		}
		scan.Parsed++
		if p.SessionID != "" {
			scan.SessionID = p.SessionID
		}
	}
	return scan
}

// CheckSessionID compares a transcript's resolved sessionId against the caller's own session
// id — the identity guard's hard-fail check against silent cross-session accounting poisoning.
func CheckSessionID(want string, scan SessionIDScan) Result {
	if scan.SessionID == want {
		return Result{SessionIDChecked: true, SessionIDMatch: true, Reason: "session id matches"}
	}
	return Result{
		SessionIDChecked: true, SessionIDMatch: false, Abort: true,
		Reason: fmt.Sprintf("transcript sessionId %q does not match this session's id %q -- cross-session transcript, aborting", scan.SessionID, want),
	}
}
