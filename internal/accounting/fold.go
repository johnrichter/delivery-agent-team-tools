package accounting

import (
	"io"

	"github.com/johnrichter/claude-shared-tooling/go/transcript"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// bucketsOf folds one turn's usage into the five priced buckets. Cache-write tokens split 5m/1h
// when the source transcript carries the split; when it does not (an older transcript, or a
// turn whose split legitimately came back zero), the flat cache-creation total is assigned to
// the 5m bucket — the standard TTL — so the total is preserved and never double-counted.
func bucketsOf(u transcript.Usage) model.ModelBuckets {
	b := model.ModelBuckets{Input: u.InputTokens, CacheRead: u.CacheReadTokens, Output: u.OutputTokens, Turns: 1}
	if u.CacheCreationEphemeral5m != 0 || u.CacheCreationEphemeral1h != 0 {
		b.CacheWrite5m = u.CacheCreationEphemeral5m
		b.CacheWrite1h = u.CacheCreationEphemeral1h
	} else {
		b.CacheWrite5m = u.CacheCreationTokens
	}
	return b
}

// FoldModels streams r as a Claude Code transcript and sums the five priced token buckets per
// model across every usage-bearing turn (both orchestrator and subagent authorship — the whole-
// session true total counts every turn regardless of who authored it).
func FoldModels(r io.Reader) (map[string]*model.ModelBuckets, error) {
	models := map[string]*model.ModelBuckets{}
	err := jsonl.Turns(r, func(t transcript.Turn) error {
		if t.Malformed || t.Usage == nil {
			return nil
		}
		agg := models[t.Model]
		if agg == nil {
			agg = &model.ModelBuckets{}
			models[t.Model] = agg
		}
		agg.Add(bucketsOf(*t.Usage))
		return nil
	})
	return models, err
}
