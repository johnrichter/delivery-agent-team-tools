package planops

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// hashLen is how many hex characters of the sha256 digest a content hash keeps — enough to be
// practically collision-free for one plan's task count, short enough to stay readable in a
// diff or a log line.
const hashLen = 16

// specFields is the reconciliation match key's spec-bearing subset of a task: id and tier are
// deliberately excluded, so retuning a model/effort is never mistaken for a spec change. Field
// order is fixed by this struct's declaration (encoding/json preserves it), not sorted, so the
// digest a caller persists as a task's identity stays stable across a rebuild of this binary —
// only a change to the fields themselves ever moves it.
type specFields struct {
	Summary     string   `json:"summary"`
	Deliverable string   `json:"deliverable"`
	Acceptance  []string `json:"acceptance"`
}

// ContentHash is the reconciliation match key: a sha256 digest of a task's spec-bearing fields.
func ContentHash(t model.Task) string {
	canon, err := json.Marshal(specFields{t.Summary, t.Deliverable, append([]string{}, t.Acceptance...)})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(canon)
	return fmt.Sprintf("%x", sum)[:hashLen]
}

// Hashes returns {taskID: contentHash} for the whole plan.
func Hashes(p model.Plan) map[string]string {
	out := map[string]string{}
	for _, r := range WalkTasks(p) {
		out[r.Task.ID] = ContentHash(r.Task)
	}
	return out
}
