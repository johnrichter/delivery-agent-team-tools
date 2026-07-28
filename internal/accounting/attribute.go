package accounting

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/johnrichter/claude-shared-tooling/go/transcript"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

var taskIDRE = regexp.MustCompile(`[A-Z][0-9]+\.P[0-9]+\.T[0-9]+`)

// userTurnProbe reads just enough of a raw transcript line to detect a `user` turn and reach
// its text content — the spawning-task-id carrier no accounting-scoped library models, since
// it is this workspace's own dispatch-prompt convention, not a generic transcript field.
type userTurnProbe struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func firstUserText(line []byte) (string, bool) {
	var p userTurnProbe
	if json.Unmarshal(line, &p) != nil || p.Message == nil {
		return "", false
	}
	if p.Type != "user" && p.Message.Role != "user" {
		return "", false
	}
	return contentText(p.Message.Content), true
}

func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var b strings.Builder
		for _, blk := range blocks {
			if blk.Text != "" {
				b.WriteString(blk.Text)
				b.WriteByte(' ')
			}
		}
		return b.String()
	}
	return ""
}

// parseForAttribution reads one subagent transcript fully and returns the task id extracted
// from its first user turn ("" if none) plus the per-model buckets summed across every
// usage-bearing turn. A whole-file measured parse: the final line folds even without a
// trailing newline.
func parseForAttribution(r io.Reader) (taskID string, models map[string]*model.ModelBuckets, err error) {
	models = map[string]*model.ModelBuckets{}
	br := bufio.NewReaderSize(r, 1<<20)
	gotUser := false
	for {
		line, e := br.ReadBytes('\n')
		if len(line) > 0 {
			foldRawLine(models, line)
			if !gotUser {
				if text, ok := firstUserText(line); ok {
					taskID = taskIDRE.FindString(text)
					gotUser = true
				}
			}
		}
		if e == io.EOF {
			return taskID, models, nil
		}
		if e != nil {
			return taskID, models, e
		}
	}
}

// foldRawLine folds one already-read transcript line into models, reusing the ClaudeCodeJSONL
// single-line parse so attribution's bucket math can never drift from the whole-session fold.
func foldRawLine(models map[string]*model.ModelBuckets, line []byte) {
	if len(strings.TrimSpace(string(line))) == 0 {
		return
	}
	err := transcript.ClaudeCodeJSONL{}.Turns(strings.NewReader(string(line)), func(t transcript.Turn) error {
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
	_ = err
}

// AttribSource is one subagent transcript to attribute.
type AttribSource struct {
	FileID string
	Reader io.Reader
}

// TaskCost is one task's MEASURED cost: the per-model buckets summed across every subagent
// transcript that mapped to it.
type TaskCost struct {
	Models          map[string]*model.ModelBuckets `json:"models"`
	CostByModel     map[string]float64             `json:"cost_by_model,omitempty"`
	CostUSD         float64                        `json:"cost_usd"`
	Turns           int64                          `json:"turns"`
	Unpriced        []string                       `json:"unpriced_models,omitempty"`
	Transcripts     []string                       `json:"transcripts"`
	CostAttribution string                         `json:"cost_attribution"`
}

// EvenSplitPool is the flagged fallback: the summed cost of every unmappable transcript,
// distributed evenly across the batch's known tasks.
type EvenSplitPool struct {
	Models          map[string]*model.ModelBuckets `json:"models"`
	CostByModel     map[string]float64             `json:"cost_by_model,omitempty"`
	CostUSD         float64                        `json:"cost_usd"`
	PerTaskCostUSD  float64                        `json:"per_task_cost_usd"`
	Turns           int64                          `json:"turns"`
	Unpriced        []string                       `json:"unpriced_models,omitempty"`
	Tasks           []string                       `json:"tasks"`
	Transcripts     []string                       `json:"transcripts"`
	CostAttribution string                         `json:"cost_attribution"`
}

// UnmappableTranscript surfaces a transcript that could not be attributed to a known task.
type UnmappableTranscript struct {
	Transcript  string `json:"transcript"`
	ExtractedID string `json:"extracted_id,omitempty"`
	Reason      string `json:"reason"`
}

// Attribution is the per-task measured breakdown for a batch of subagent transcripts, plus the
// flagged even-split pool for any unmappable transcript.
type Attribution struct {
	Tasks      map[string]*TaskCost   `json:"tasks"`
	KnownTasks []string               `json:"known_tasks"`
	EvenSplit  *EvenSplitPool         `json:"even_split,omitempty"`
	Unmappable []UnmappableTranscript `json:"unmappable,omitempty"`
	At         string                 `json:"at,omitempty"`
}

// Attribute maps each subagent transcript to its spawning task: extract the first task id from
// the first user turn, match it by exact equality against known. Mapped transcripts accumulate
// measured per-task cost; unmappable transcripts feed the flagged even-split pool.
func Attribute(sources []AttribSource, known []string, priced bool, at string) (*Attribution, error) {
	knownSet := map[string]bool{}
	for _, k := range known {
		if k = strings.TrimSpace(k); k != "" {
			knownSet[k] = true
		}
	}
	knownList := sortedKeys(knownSet)

	attr := &Attribution{Tasks: map[string]*TaskCost{}, KnownTasks: knownList, At: at}
	poolModels := map[string]*model.ModelBuckets{}
	var poolTranscripts []string

	for _, s := range sources {
		id, models, err := parseForAttribution(s.Reader)
		if err != nil {
			return nil, err
		}
		if id != "" && knownSet[id] {
			tc := attr.Tasks[id]
			if tc == nil {
				tc = &TaskCost{Models: map[string]*model.ModelBuckets{}, CostAttribution: "measured"}
				attr.Tasks[id] = tc
			}
			mergeModels(tc.Models, models)
			tc.Transcripts = append(tc.Transcripts, s.FileID)
			continue
		}
		reason := "extracted task-id not in known set"
		if id == "" {
			reason = "no task-id in first user turn"
		}
		attr.Unmappable = append(attr.Unmappable, UnmappableTranscript{Transcript: s.FileID, ExtractedID: id, Reason: reason})
		mergeModels(poolModels, models)
		poolTranscripts = append(poolTranscripts, s.FileID)
	}

	for _, tc := range attr.Tasks {
		tc.CostByModel, tc.CostUSD, tc.Turns, tc.Unpriced = priceModels(tc.Models, priced)
		sort.Strings(tc.Transcripts)
	}
	if len(poolTranscripts) > 0 {
		sort.Strings(poolTranscripts)
		byModel, total, turns, unpriced := priceModels(poolModels, priced)
		pool := &EvenSplitPool{
			Models: poolModels, CostByModel: byModel, CostUSD: total, Turns: turns, Unpriced: unpriced,
			Tasks: knownList, Transcripts: poolTranscripts, CostAttribution: "batch-even-split",
		}
		if n := len(knownList); n > 0 {
			pool.PerTaskCostUSD = total / float64(n)
		}
		attr.EvenSplit = pool
	}
	return attr, nil
}

func mergeModels(dst, src map[string]*model.ModelBuckets) {
	for m, b := range src {
		agg := dst[m]
		if agg == nil {
			agg = &model.ModelBuckets{}
			dst[m] = agg
		}
		agg.Add(*b)
	}
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
