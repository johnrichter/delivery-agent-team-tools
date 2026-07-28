// Package accounting is whole-session, per-model true-cost accounting: it walks the main
// session transcript and every subagent transcript, sums the five priced token buckets per
// model (transcript.ClaudeCodeJSONL), prices each model from the live model roster
// (roster.Price), and tracks a per-file byte watermark so a resumed run parses only appended
// bytes.
package accounting

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/johnrichter/claude-shared-tooling/go/transcript"
)

var jsonl = transcript.ClaudeCodeJSONL{}

// cleanAbs returns path's cleaned absolute form, the stable FileID key every discovery/open
// call in this package normalizes to.
func cleanAbs(path string) string {
	if a, err := filepath.Abs(path); err == nil {
		return filepath.Clean(a)
	}
	return filepath.Clean(path)
}

// DiscoverSubagents returns every subagent transcript spawned from the main transcript at
// path, at any nesting depth, deduped and sorted for a stable key order.
func DiscoverSubagents(mainPath string) []string {
	subs, err := jsonl.DiscoverSubagentTranscripts(mainPath)
	if err != nil {
		return nil
	}
	mainAbs := cleanAbs(mainPath)
	seen := map[string]bool{mainAbs: true}
	var out []string
	for _, a := range subs {
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}

// DiscoverSession resolves a session's whole transcript set: the main transcript itself plus
// every subagent transcript found alongside it. resolved is false when the main transcript
// cannot be stat'd.
func DiscoverSession(mainPath string) (paths []string, resolved bool) {
	mainAbs := cleanAbs(mainPath)
	if _, err := os.Stat(mainAbs); err != nil {
		return nil, false
	}
	seen := map[string]bool{mainAbs: true}
	paths = []string{mainAbs}
	for _, a := range DiscoverSubagents(mainPath) {
		if !seen[a] {
			seen[a] = true
			paths = append(paths, a)
		}
	}
	sort.Strings(paths)
	return paths, true
}

// Source is one transcript file to fold into an Accounting. Reader is positioned at
// StartOffset; folding reads from there to EOF and the new watermark is
// StartOffset+bytes-consumed.
type Source struct {
	FileID      string
	Reader      *os.File
	StartOffset int64
}

// OpenSources opens each path and seeks it to its prior watermark (from a previous Accounting
// snapshot), returning the sources to fold plus the handles the caller must close. A watermark
// past the current file size (rotation/truncation) resets that file to a full re-parse. When
// prior is nil every file starts at offset 0. A file that fails to open is skipped
// (best-effort — whole-session true cost never blocks on one bad file).
func OpenSources(paths []string, priorWatermarks map[string]int64) []Source {
	var sources []Source
	for _, p := range paths {
		fh, err := os.Open(p)
		if err != nil {
			continue
		}
		var start int64
		if wm, ok := priorWatermarks[p]; ok {
			if fi, err := fh.Stat(); err == nil && wm <= fi.Size() {
				if _, err := fh.Seek(wm, os.SEEK_SET); err == nil {
					start = wm
				}
			}
		}
		sources = append(sources, Source{FileID: p, Reader: fh, StartOffset: start})
	}
	return sources
}

// CloseAll closes every source's open handle.
func CloseAll(sources []Source) {
	for _, s := range sources {
		_ = s.Reader.Close()
	}
}
