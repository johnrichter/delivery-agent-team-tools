// Package docio is the engine's one seam onto the filesystem for reading/writing the plan,
// execution, archive, and feedback JSON documents. Every command package composes these
// helpers rather than calling os/encoding-json directly, so the read/decode/write contract
// (and its errors) stay uniform across the whole CLI.
package docio

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/johnrichter/claude-shared-tooling/go/fsx"
)

// ReadJSON reads path and decodes it into v.
func ReadJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// WriteJSON marshals v as indented JSON and durably writes it to path via fsx.WriteAtomic — a
// crash mid-write leaves either the previous contents or the new ones, never a partial file.
func WriteJSON(path string, v any, perm fs.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	data = append(data, '\n')
	return fsx.WriteAtomic(path, data, perm)
}

// WriteText durably writes raw text (e.g. a rendered Markdown mirror) to path via
// fsx.WriteAtomic.
func WriteText(path, content string) error {
	return fsx.WriteAtomic(path, []byte(content), DefaultPerm)
}

// DefaultPerm is the file mode every document write in this engine uses.
const DefaultPerm fs.FileMode = 0o644

// Now returns the current UTC timestamp in the RFC3339 form every command stamps by default.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// OrNow returns at if non-empty, else Now() — the shared "--at override, default now" rule
// every timestamped command applies.
func OrNow(at string) string {
	if at != "" {
		return at
	}
	return Now()
}
