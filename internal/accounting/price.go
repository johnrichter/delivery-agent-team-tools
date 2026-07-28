package accounting

import (
	"sort"

	"github.com/johnrichter/claude-shared-tooling/go/roster"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// cost returns b's dollar cost at the roster's per-million-token rate.
func cost(b model.ModelBuckets, rate roster.PriceTable) float64 {
	return (float64(b.Input)*rate.Input +
		float64(b.CacheWrite5m)*rate.CacheWrite5m +
		float64(b.CacheWrite1h)*rate.CacheWrite1h +
		float64(b.CacheRead)*rate.CacheRead +
		float64(b.Output)*rate.Output) / 1e6
}

// priceModels is the one place buckets become dollars: for each model it sums turns and, when
// priced is true, prices it from the live model roster and collects any model the roster
// cannot price into unpriced (sorted — never silently priced at $0). priced=false is the
// caller's documented opt-out (the `usage` command): buckets are still counted for Turns with
// no cost math run.
func priceModels(models map[string]*model.ModelBuckets, priced bool) (byModel map[string]float64, total float64, turns int64, unpriced []string) {
	byModel = map[string]float64{}
	for m, b := range models {
		turns += b.Turns
		if !priced {
			continue
		}
		rate, err := roster.Price(m)
		if err != nil {
			unpriced = append(unpriced, m)
			continue
		}
		c := cost(*b, rate)
		byModel[m] = c
		total += c
	}
	if len(byModel) == 0 {
		byModel = nil
	}
	if len(unpriced) > 0 {
		sort.Strings(unpriced)
	}
	return byModel, total, turns, unpriced
}

// Flatten collapses per-model buckets into the model-agnostic Usage totals.
func Flatten(models map[string]*model.ModelBuckets) model.Usage {
	var u model.Usage
	for _, b := range models {
		u.InputTokens += b.Input
		u.CacheCreationTokens += b.CacheWrite5m + b.CacheWrite1h
		u.CacheReadTokens += b.CacheRead
		u.OutputTokens += b.Output
		u.Turns += b.Turns
	}
	u.TotalTokens = u.InputTokens + u.CacheCreationTokens + u.CacheReadTokens + u.OutputTokens
	return u
}
