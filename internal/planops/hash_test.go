package planops

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// TestContentHash_MatchesFixedFieldOrderDigest locks ContentHash to sha256 of the plain,
// declaration-ordered JSON encoding of {summary, deliverable, acceptance} — not a
// sorted-key/canonical form. This is the reconciliation match key a caller persists as a
// task's identity across runs; a canonicalization pass would silently move every existing
// hash on the next binary swap even though nothing about the task's spec changed.
func TestContentHash_MatchesFixedFieldOrderDigest(t *testing.T) {
	task := model.Task{ID: "M1.P1.T1", Summary: "s", Deliverable: "d", Acceptance: []string{"a", "b"}}
	canon, err := json.Marshal(struct {
		Summary     string   `json:"summary"`
		Deliverable string   `json:"deliverable"`
		Acceptance  []string `json:"acceptance"`
	}{task.Summary, task.Deliverable, task.Acceptance})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canon)
	want := fmt.Sprintf("%x", sum)[:hashLen]
	if got := ContentHash(task); got != want {
		t.Fatalf("ContentHash = %q, want %q (sha256 of %s)", got, want, canon)
	}
}

// TestContentHash_ExcludesIDAndTier confirms retuning id/model/effort never moves the hash —
// only the spec-bearing fields do.
func TestContentHash_ExcludesIDAndTier(t *testing.T) {
	base := model.Task{ID: "M1.P1.T1", Summary: "s", Deliverable: "d", Acceptance: []string{"a"}, Model: "claude-sonnet-5", Effort: "medium"}
	retuned := base
	retuned.ID, retuned.Model, retuned.Effort = "M1.P1.T2", "claude-opus-4-8", "high"
	if ContentHash(base) != ContentHash(retuned) {
		t.Fatal("ContentHash must be stable across an id/model/effort-only change")
	}
}

// TestContentHash_SpecChangeMovesHash confirms a genuine spec-field edit does move the hash —
// the counterpart to the exclusion test above.
func TestContentHash_SpecChangeMovesHash(t *testing.T) {
	a := model.Task{Summary: "s1", Deliverable: "d", Acceptance: []string{"a"}}
	b := model.Task{Summary: "s2", Deliverable: "d", Acceptance: []string{"a"}}
	if ContentHash(a) == ContentHash(b) {
		t.Fatal("ContentHash must change when summary changes")
	}
}
