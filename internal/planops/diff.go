package planops

import (
	"cmp"
	"slices"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

type IDHash struct {
	ID   string `json:"id"`
	Hash string `json:"hash"`
}

type IDChange struct {
	ID      string `json:"id"`
	OldHash string `json:"old_hash"`
	NewHash string `json:"new_hash"`
}

type DiffResult struct {
	Carried []IDHash   `json:"carried"`
	Changed []IDChange `json:"changed"`
	Added   []IDHash   `json:"added"`
	Removed []IDHash   `json:"removed"`
}

// Diff matches tasks between two plans by id + content hash: same id+hash carries, same
// id/different hash is a spec change, a new id is added, an absent id is removed.
func Diff(oldP, newP model.Plan) DiffResult {
	oldByID := map[string]string{}
	for _, r := range WalkTasks(oldP) {
		oldByID[r.Task.ID] = ContentHash(r.Task)
	}
	res := DiffResult{Carried: []IDHash{}, Changed: []IDChange{}, Added: []IDHash{}, Removed: []IDHash{}}
	newIDs := map[string]bool{}
	for _, r := range WalkTasks(newP) {
		h := ContentHash(r.Task)
		newIDs[r.Task.ID] = true
		old, ok := oldByID[r.Task.ID]
		switch {
		case !ok:
			res.Added = append(res.Added, IDHash{r.Task.ID, h})
		case old == h:
			res.Carried = append(res.Carried, IDHash{r.Task.ID, h})
		default:
			res.Changed = append(res.Changed, IDChange{r.Task.ID, old, h})
		}
	}
	for id := range oldByID {
		if !newIDs[id] {
			res.Removed = append(res.Removed, IDHash{ID: id})
		}
	}
	slices.SortFunc(res.Removed, func(a, b IDHash) int { return cmp.Compare(a.ID, b.ID) })
	return res
}
