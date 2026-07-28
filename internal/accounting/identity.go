package accounting

import (
	"math"
	"sort"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// ComputeIdentity computes the closed-form additive accounting identity over a's ledger:
// session_total = O + Σ(agent-*.jsonl) + fixed-subagents + residual. Every term is priced from
// its own ledger entry, so a ledger entry outside the caller's known/fixed classification
// surfaces as a nonzero, itemized residual instead of silently vanishing into O.
//
// mainFileID is O's own ledger entry (priced via PriceFile, never adjusted here).
// knownSubagents is the independently-discovered set of non-orchestrator transcript paths that
// fold into the variable-agents sum; fixedSubagents (may be nil — no fixed-model classifier
// exists yet) is the subset classified as fixed-model escalation/review subagents instead. A
// ledger entry that is neither mainFileID nor named in either list is unclassified: excluded
// from every bucket, so it inflates residual and is itemized rather than landing in O.
func ComputeIdentity(a *model.Accounting, mainFileID string, knownSubagents, fixedSubagents []string, priced bool) model.IdentityResult {
	fixedSet := pathSet(fixedSubagents)
	knownSet := pathSet(knownSubagents)

	o, _ := PriceFile(a, mainFileID, priced)

	var variableUSD, fixedUSD float64
	var unclassified []model.UnclassifiedTranscript
	for fileID, entry := range a.Ledger {
		if fileID == mainFileID {
			continue
		}
		_, cost, _, _ := priceModels(entry, priced)
		switch {
		case fixedSet[fileID]:
			fixedUSD += cost
		case knownSet[fileID]:
			variableUSD += cost
		default:
			unclassified = append(unclassified, model.UnclassifiedTranscript{Path: fileID, CostUSD: cost, Model: modelKeysJoined(entry)})
		}
	}
	sort.Slice(unclassified, func(i, j int) bool { return unclassified[i].Path < unclassified[j].Path })

	_, sessionTotal, _, _ := priceModels(a.Models, priced)
	residual := sessionTotal - (o.CostUSD + variableUSD + fixedUSD)
	tolerance := math.Max(1e-6*sessionTotal, 1e-6)

	res := model.IdentityResult{
		SessionTotalUSD: sessionTotal, OUSD: o.CostUSD, VariableAgentsUSD: variableUSD,
		FixedSubagentsUSD: fixedUSD, ResidualUSD: residual, ToleranceUSD: tolerance, CostStatus: "ok",
	}
	if math.Abs(residual) > tolerance {
		res.CostStatus = "residual-exceeded"
		res.UnclassifiedTranscripts = unclassified
	}
	return res
}

func pathSet(ids []string) map[string]bool {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func modelKeysJoined(entry map[string]*model.ModelBuckets) string {
	keys := make([]string, 0, len(entry))
	for m := range entry {
		keys = append(keys, m)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
