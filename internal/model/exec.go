package model

// LegacyExecSchemaVersion is the implicit version of every execution.json written before the
// field existed. CurrentExecSchemaVersion is what a fresh init-exec stamps today.
const (
	LegacyExecSchemaVersion  = 1
	CurrentExecSchemaVersion = 3
)

const ExecSchema = "execution-state/v1"

// ExecState is the canonical, mutable build state: one row per task plus resumable run config.
type ExecState struct {
	Schema        string     `json:"schema"`
	SchemaVersion int        `json:"schema_version,omitempty"`
	Project       string     `json:"project"`
	Name          string     `json:"name"`
	Topic         string     `json:"topic"`
	Goal          string     `json:"goal"`
	Provenance    Provenance `json:"provenance"`
	Started       string     `json:"started"`
	Updated       string     `json:"updated"`
	RunConfig     RunConfig  `json:"run_config"`
	Tasks         []ExecTask `json:"tasks"`
	// Archived is the compact tombstone index for tasks the `archive` op moved out of Tasks.
	Archived         []Tombstone       `json:"archived,omitempty"`
	Log              []string          `json:"log"`
	PauseEvents      []PauseEvent      `json:"pause_events,omitempty"`
	EscalationEvents []EscalationEvent `json:"escalation_events,omitempty"`
}

// RunConfig is the resumable run configuration; it survives pause-gate turns.
type RunConfig struct {
	PauseMode        string      `json:"pause_mode"`
	Budget           string      `json:"budget"`
	BudgetCeilingUSD *float64    `json:"budget_ceiling_usd"`
	SpentUSD         float64     `json:"spent_usd"`
	TokensOut        int64       `json:"tokens_out"`
	Usage            *Usage      `json:"usage,omitempty"`
	TrueUsage        *Usage      `json:"true_usage,omitempty"`
	Accounting       *Accounting `json:"accounting,omitempty"`
	Rates            string      `json:"rates"`
	Override         string      `json:"override,omitempty"`
	LastRunID        string      `json:"last_run_id,omitempty"`
}

// Usage is a transcript-derived true token total, including all subagent turns.
type Usage struct {
	InputTokens         int64  `json:"input_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_input_tokens"`
	CacheReadTokens     int64  `json:"cache_read_input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	TotalTokens         int64  `json:"total_tokens"`
	Turns               int64  `json:"turns"`
	At                  string `json:"at,omitempty"`
}

// Add folds src's token classes into u in place.
func (u *Usage) Add(src Usage) {
	u.InputTokens += src.InputTokens
	u.CacheCreationTokens += src.CacheCreationTokens
	u.CacheReadTokens += src.CacheReadTokens
	u.OutputTokens += src.OutputTokens
	u.TotalTokens += src.TotalTokens
	u.Turns += src.Turns
}

// ExecTask is one task's live row. CostUSD/TokensOut are OUTPUT-only (a lower bound); Usage
// carries the fuller four-class measured basis once recorded.
type ExecTask struct {
	ID        string          `json:"id"`
	Summary   string          `json:"summary"`
	Kind      DeliverableKind `json:"deliverable_kind,omitempty"`
	Model     Model           `json:"model"`
	Effort    Effort          `json:"effort"`
	Status    Status          `json:"status"`
	Test      string          `json:"test,omitempty"`
	Review    string          `json:"review,omitempty"`
	Commit    string          `json:"commit,omitempty"`
	CostUSD   float64         `json:"cost_usd"`
	TokensOut int64           `json:"tokens_out"`
	Usage     *Usage          `json:"usage,omitempty"`
	Updated   string          `json:"updated"`
	Notes     string          `json:"notes,omitempty"`
}

// Tombstone is the compact per-archived-task index ExecState.Archived carries.
type Tombstone struct {
	ID        string  `json:"id"`
	Summary   string  `json:"summary"`
	Status    Status  `json:"status"`
	Commit    string  `json:"commit,omitempty"`
	CostUSD   float64 `json:"cost_usd"`
	TokensOut int64   `json:"tokens_out"`
	Usage     *Usage  `json:"usage,omitempty"`
}

// PauseEvent is one structured pause occurrence.
type PauseEvent struct {
	ReasonEnum PauseReason `json:"reason_enum"`
	At         string      `json:"at"`
	TaskID     string      `json:"task_id,omitempty"`
}

// EscalationTrigger is a member of the closed named set the magistrate judges.
type EscalationTrigger string

// EscalationEvent is one structured magistrate-firing occurrence.
type EscalationEvent struct {
	Trigger EscalationTrigger `json:"trigger"`
	Tier    string            `json:"tier"`
	Route   string            `json:"route"`
	At      string            `json:"at"`
	TaskID  string            `json:"task_id,omitempty"`
}
