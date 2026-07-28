package model

// ModelBuckets holds the five priced token buckets summed for one model across the turns
// counted, mirroring the rate dimensions the model roster prices.
type ModelBuckets struct {
	Input        int64 `json:"input_tokens"`
	CacheWrite5m int64 `json:"cache_write_5m_tokens"`
	CacheWrite1h int64 `json:"cache_write_1h_tokens"`
	CacheRead    int64 `json:"cache_read_tokens"`
	Output       int64 `json:"output_tokens"`
	Turns        int64 `json:"turns"`
}

// Add folds o into b in place.
func (b *ModelBuckets) Add(o ModelBuckets) {
	b.Input += o.Input
	b.CacheWrite5m += o.CacheWrite5m
	b.CacheWrite1h += o.CacheWrite1h
	b.CacheRead += o.CacheRead
	b.Output += o.Output
	b.Turns += o.Turns
}

// Accounting is the resumable, whole-session true-cost state persisted in RunConfig: a
// per-file byte watermark (so a re-run parses only appended bytes), a per-file bucket ledger,
// the session-total per-model buckets (derived, never hand-summed), and the derived dollar cost.
type Accounting struct {
	Watermarks      map[string]int64                    `json:"watermarks"`
	Ledger          map[string]map[string]*ModelBuckets `json:"ledger,omitempty"`
	Models          map[string]*ModelBuckets            `json:"models"`
	CostByModel     map[string]float64                  `json:"cost_by_model,omitempty"`
	Unpriced        []string                            `json:"unpriced_models,omitempty"`
	CostUSD         float64                             `json:"cost_usd"`
	Turns           int64                               `json:"turns"`
	At              string                              `json:"at,omitempty"`
	Orchestrator    *OrchestratorCost                   `json:"orchestrator,omitempty"`
	CostStatus      string                              `json:"cost_status,omitempty"`
	SpecsAsOf       string                              `json:"specs_as_of,omitempty"`
	BuildHelpersSHA string                              `json:"build_helpers_sha,omitempty"`
	Identity        *IdentityResult                     `json:"identity,omitempty"`
}

// OrchestratorCost is O: the top-level orchestrator transcript's own true-cost, isolated
// per-transcript so a same-model subagent's cost can never leak into it.
type OrchestratorCost struct {
	Usage
	CostUSD float64 `json:"cost_usd"`
}

// IdentityResult is the closed-form check session_total = O + Σ(agent-*.jsonl) + residual.
type IdentityResult struct {
	SessionTotalUSD         float64                  `json:"session_total_usd"`
	OUSD                    float64                  `json:"o_usd"`
	VariableAgentsUSD       float64                  `json:"variable_agents_usd"`
	FixedSubagentsUSD       float64                  `json:"fixed_subagents_usd"`
	ResidualUSD             float64                  `json:"residual_usd"`
	ToleranceUSD            float64                  `json:"tolerance_usd"`
	CostStatus              string                   `json:"cost_status"`
	UnclassifiedTranscripts []UnclassifiedTranscript `json:"unclassified_transcripts,omitempty"`
}

// UnclassifiedTranscript is one ledger entry the identity check could not place into
// {O, agent-*, fixed}.
type UnclassifiedTranscript struct {
	Path    string  `json:"path"`
	CostUSD float64 `json:"cost_usd"`
	Model   string  `json:"model,omitempty"`
}
