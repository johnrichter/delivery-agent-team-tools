package accounting

import (
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// Account folds sources into prior (a fresh Accounting when prior is nil), keeping a per-file
// bucket ledger so session totals are Σ over per-file entries. A source at StartOffset==0
// (fresh file, a --final full re-parse, or a rotation/truncation reset) REPLACES that file's
// ledger entry; StartOffset>0 (incremental resume) ADDS only the delta onto it. After folding,
// Models is rebuilt from the ledger and per-model + total cost are recomputed; a model matching
// no roster rate is recorded in Unpriced rather than dropped or priced at $0.
func Account(prior *model.Accounting, sources []Source, priced bool, at string) (*model.Accounting, error) {
	acct := prior
	if acct == nil {
		acct = &model.Accounting{}
	}
	if acct.Watermarks == nil {
		acct.Watermarks = map[string]int64{}
	}
	if acct.Ledger == nil {
		acct.Ledger = map[string]map[string]*model.ModelBuckets{}
	}
	for _, s := range sources {
		models, err := FoldModels(s.Reader)
		if err != nil {
			return acct, err
		}
		fi, statErr := s.Reader.Stat()
		var newWatermark int64
		if statErr == nil {
			newWatermark = fi.Size()
		}
		if s.StartOffset == 0 {
			acct.Ledger[s.FileID] = models
		} else {
			entry := acct.Ledger[s.FileID]
			if entry == nil {
				entry = map[string]*model.ModelBuckets{}
				acct.Ledger[s.FileID] = entry
			}
			for m, b := range models {
				agg := entry[m]
				if agg == nil {
					agg = &model.ModelBuckets{}
					entry[m] = agg
				}
				agg.Add(*b)
			}
		}
		acct.Watermarks[s.FileID] = newWatermark
	}
	rebuildModels(acct)
	acct.CostByModel, acct.CostUSD, acct.Turns, acct.Unpriced = priceModels(acct.Models, priced)
	acct.At = at
	return acct, nil
}

func rebuildModels(a *model.Accounting) {
	models := map[string]*model.ModelBuckets{}
	for _, entry := range a.Ledger {
		for m, b := range entry {
			agg := models[m]
			if agg == nil {
				agg = &model.ModelBuckets{}
				models[m] = agg
			}
			agg.Add(*b)
		}
	}
	a.Models = models
}

// PriceFile derives one ledger file's true-cost in isolation — used for O, the top-level
// orchestrator transcript's own cost, isolated per-transcript so a same-model subagent's cost
// can never leak into it.
func PriceFile(a *model.Accounting, fileID string, priced bool) (model.OrchestratorCost, bool) {
	entry, found := a.Ledger[fileID]
	if !found {
		return model.OrchestratorCost{}, false
	}
	_, total, _, _ := priceModels(entry, priced)
	return model.OrchestratorCost{Usage: Flatten(entry), CostUSD: total}, true
}
