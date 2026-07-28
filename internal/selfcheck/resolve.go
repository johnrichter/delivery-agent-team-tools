package selfcheck

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/johnrichter/claude-shared-tooling/go/transcript"
)

// sessionIDPattern is the UUID shape (8-4-4-4-12 hex) Claude Code session ids take.
var sessionIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidSessionID reports whether id has the shape of a real Claude Code session id.
func ValidSessionID(id string) bool { return sessionIDPattern.MatchString(id) }

var nonAlnumRE = regexp.MustCompile(`[^A-Za-z0-9]`)

// SlugifyCWD reproduces Claude Code's project-directory slug: every rune outside [A-Za-z0-9]
// becomes '-'.
func SlugifyCWD(cwd string) string { return nonAlnumRE.ReplaceAllString(cwd, "-") }

// ParseScratchpadPath extracts (cwdSlug, sessionID) from a path under a session's own
// scratchpad directory: .../<cwd-slug>/<session-id>/scratchpad[/...].
func ParseScratchpadPath(path string) (cwdSlug, sessionID string, ok bool) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	idx := -1
	for i, p := range parts {
		if p == "scratchpad" {
			idx = i
		}
	}
	if idx < 2 {
		return "", "", false
	}
	sessionID = parts[idx-1]
	cwdSlug = parts[idx-2]
	if !ValidSessionID(sessionID) || cwdSlug == "" {
		return "", "", false
	}
	return cwdSlug, sessionID, true
}

// ResolveTranscriptPath builds the ONE deterministic transcript path for a session — a pure
// join of the projects root, the cwd slug, and the session id. No directory listing and no
// mtime comparison: a concurrently-written sibling session's transcript can never be picked by
// mistake because none is ever looked at.
func ResolveTranscriptPath(projectsDir, cwdSlug, sessionID string) string {
	return transcript.ClaudeCodeJSONL{}.ResolvePath(projectsDir, cwdSlug, sessionID)
}
