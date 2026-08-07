package surface

import (
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// ChangedResult is the reverse-direction check's verdict: every path in a git-derived
// changed-set must be covered by some declared file_surface entry (required or optional).
type ChangedResult struct {
	OK         bool     `json:"ok"`
	OffSurface []string `json:"off_surface,omitempty"`
}

// VerifyChangedSubsetOfSurface checks changed (bare paths, e.g. from `git status --porcelain`
// with its status+separator stripped) against entries. A changed path no entry covers is an
// off-surface write.
func VerifyChangedSubsetOfSurface(changed []string, entries []model.FileSurfaceEntry) ChangedResult {
	res := ChangedResult{OK: true}
	for _, c := range changed {
		cc := path.Clean(strings.TrimSpace(c))
		if cc == "" || cc == "." {
			continue
		}
		covered := false
		for _, e := range entries {
			if covers(e, cc) {
				covered = true
				break
			}
		}
		if !covered {
			res.OK = false
			res.OffSurface = append(res.OffSurface, cc)
		}
	}
	return res
}

// covers reports whether one declared file_surface entry covers changedPath, per its kind. A
// kind=dir path may carry an optional trailing `/**`, stripped before the prefix check, so
// "some/dir" and "some/dir/**" cover the same changed paths.
func covers(e model.FileSurfaceEntry, changedPath string) bool {
	p := path.Clean(strings.TrimSpace(e.Path))
	switch e.Kind.Resolve() {
	case model.FSGlob:
		ok, err := doublestar.Match(p, changedPath)
		return err == nil && ok
	case model.FSDir:
		root, pattern := doublestar.SplitPattern(p)
		if pattern != "**" {
			root = p
		}
		return changedPath == root || strings.HasPrefix(changedPath, root+"/")
	default:
		return changedPath == p
	}
}
