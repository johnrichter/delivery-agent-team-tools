package schedule

import "github.com/bmatcuk/doublestar/v4"

// matchGlob answers pattern-vs-single-path in the same doublestar dialect (`**` recurses across
// path separators) VerifyFileSurface uses against the real tree, so a batch-time disjointness
// proof and the done-gate's own glob semantics can never disagree on what a pattern matches.
func matchGlob(pattern, name string) (bool, error) {
	return doublestar.Match(pattern, name)
}
