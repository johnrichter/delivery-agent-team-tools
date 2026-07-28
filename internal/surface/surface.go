// Package surface is the file_surface re-assertion pair: the forward direction (every declared
// entry present on disk, per its pinned match semantics) and the reverse direction (every
// changed path covered by some declared entry).
package surface

import (
	"io/fs"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// Violation is one declared entry that failed its pinned match semantics.
type Violation struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Result is the forward-direction (pre-done) gate's verdict.
type Result struct {
	OK         bool        `json:"ok"`
	Violations []Violation `json:"violations,omitempty"`
}

// Verify applies file_surface's pinned match semantics against fsys, rooted at the task's
// worktree: kind=file must exist as a non-directory; kind=glob must match >=1 entry
// (doublestar dialect, so `**` recurses across separators); kind=dir must exist, be a
// directory, and be non-empty. A Required entry (any kind) additionally demands its matched
// content be non-trivial — non-zero byte size — so a task cannot fake completion with an empty
// placeholder.
func Verify(fsys fs.FS, entries []model.FileSurfaceEntry) Result {
	res := Result{OK: true}
	fail := func(path, reason string) {
		res.OK = false
		res.Violations = append(res.Violations, Violation{Path: path, Reason: reason})
	}
	for _, e := range entries {
		switch e.Kind.Resolve() {
		case model.FSGlob:
			verifyGlob(fsys, e, fail)
		case model.FSDir:
			verifyDir(fsys, e, fail)
		default:
			verifyFile(fsys, e, fail)
		}
	}
	return res
}

// VerifyMerged is the post-merge form of the forward direction: the union of every merged
// task's declared file_surface, checked in one call against the merged tree.
func VerifyMerged(fsys fs.FS, perTaskSurfaces [][]model.FileSurfaceEntry) Result {
	var union []model.FileSurfaceEntry
	for _, s := range perTaskSurfaces {
		union = append(union, s...)
	}
	return Verify(fsys, union)
}

func verifyGlob(fsys fs.FS, e model.FileSurfaceEntry, fail func(path, reason string)) {
	matches, err := doublestar.Glob(fsys, e.Path)
	if err != nil {
		fail(e.Path, "malformed glob pattern: "+err.Error())
		return
	}
	if len(matches) == 0 {
		fail(e.Path, "glob matched zero files")
		return
	}
	if !e.Required {
		return
	}
	for _, m := range matches {
		info, err := fs.Stat(fsys, m)
		switch {
		case err != nil:
			fail(e.Path, "matched file "+m+" could not be stat'd: "+err.Error())
		case !info.IsDir() && info.Size() == 0:
			fail(e.Path, "matched file "+m+" is empty (required entry must be non-trivial)")
		}
	}
}

func verifyDir(fsys fs.FS, e model.FileSurfaceEntry, fail func(path, reason string)) {
	info, err := fs.Stat(fsys, e.Path)
	if err != nil {
		fail(e.Path, "directory does not exist: "+err.Error())
		return
	}
	if !info.IsDir() {
		fail(e.Path, "path exists but is not a directory")
		return
	}
	children, err := fs.ReadDir(fsys, e.Path)
	if err != nil {
		fail(e.Path, "cannot read directory: "+err.Error())
		return
	}
	if len(children) == 0 {
		fail(e.Path, "directory is empty")
	}
}

func verifyFile(fsys fs.FS, e model.FileSurfaceEntry, fail func(path, reason string)) {
	info, err := fs.Stat(fsys, e.Path)
	if err != nil {
		fail(e.Path, "file does not exist: "+err.Error())
		return
	}
	if info.IsDir() {
		fail(e.Path, "path exists but is a directory, not a file")
		return
	}
	if e.Required && info.Size() == 0 {
		fail(e.Path, "file exists but is empty (required entry must be non-trivial)")
	}
}
