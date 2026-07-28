package accounting

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// SpecsAsOf extracts anthropic-specifications.json's own `_as_of` date from its raw bytes.
// Best-effort: unreadable/invalid specs or a missing key yields "" — accounting must never
// block a run over a provenance field.
func SpecsAsOf(specs []byte) string {
	var doc struct {
		AsOf string `json:"_as_of"`
	}
	if json.Unmarshal(specs, &doc) != nil {
		return ""
	}
	return doc.AsOf
}

// SetAccounting records a whole-session, per-model true-cost accounting snapshot into run
// config, derives O (the main transcript's own isolated cost) as a distinct line item,
// computes the additive accounting identity (session_total = O + Σ(agent-*.jsonl) + residual),
// and logs it. final marks the finish-time authoritative snapshot in the log.
func SetAccounting(ex *model.ExecState, acct *model.Accounting, mainFileID string, priced, final bool, at, specsAsOf, engineSHA string) {
	acct.At = at
	acct.CostStatus = ""
	acct.SpecsAsOf = specsAsOf
	acct.BuildHelpersSHA = engineSHA
	if o, ok := PriceFile(acct, mainFileID, priced); ok {
		acct.Orchestrator = &o
	}
	knownSubagents := DiscoverSubagents(mainFileID) // best-effort; nil on error, never fatal
	identity := ComputeIdentity(acct, mainFileID, knownSubagents, nil, priced)
	acct.Identity = &identity
	ex.RunConfig.Accounting = acct
	u := Flatten(acct.Models)
	u.At = at
	ex.RunConfig.TrueUsage = &u
	ex.Updated = at

	label := "true-cost snapshot"
	if final {
		label = "final true-cost snapshot"
	}
	line := fmt.Sprintf("%s NOTE %s — $%.6f across %d turns, %d models (%d in + %d out + %d cache = %d total tokens; transcript-derived, best-effort)",
		at, label, acct.CostUSD, acct.Turns, len(acct.Models), u.InputTokens, u.OutputTokens, u.CacheCreationTokens+u.CacheReadTokens, u.TotalTokens)
	if acct.Orchestrator != nil {
		line += fmt.Sprintf(" — O (orchestrator-only) $%.6f", acct.Orchestrator.CostUSD)
	}
	if len(acct.Unpriced) > 0 {
		line += " — UNPRICED models (no rate matched, excluded from cost): " + strings.Join(acct.Unpriced, ", ")
	}
	line += fmt.Sprintf(" — identity residual $%.6f (tolerance $%.6f, %s)", identity.ResidualUSD, identity.ToleranceUSD, identity.CostStatus)
	if identity.CostStatus == "residual-exceeded" {
		line += fmt.Sprintf(" — UNCLASSIFIED transcripts: %d", len(identity.UnclassifiedTranscripts))
	}
	ex.Log = append(ex.Log, line)
}

// SetUnresolved records the non-fatal cost_status:unresolved marker when the main transcript
// could not be read this run. Prior accounting is left untouched.
func SetUnresolved(ex *model.ExecState, transcriptPath, at string) {
	if ex.RunConfig.Accounting == nil {
		ex.RunConfig.Accounting = &model.Accounting{}
	}
	ex.RunConfig.Accounting.CostStatus = "unresolved"
	ex.RunConfig.Accounting.At = at
	ex.Updated = at
	ex.Log = append(ex.Log, fmt.Sprintf("%s NOTE cost_status:unresolved — main transcript %s could not be read this run; prior accounting (if any) left unchanged, O not updated (non-fatal)", at, transcriptPath))
}
