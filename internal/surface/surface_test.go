package surface

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// mkPopulatedDir creates dir/file.go under t.TempDir() with non-empty content and returns the
// tempdir root, ready to be wrapped in os.DirFS.
func mkPopulatedDir(t *testing.T, rel string) string {
	t.Helper()
	root := t.TempDir()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", full, err)
	}
	if err := os.WriteFile(filepath.Join(full, "file.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return root
}

// TestVerify_KindDir_DoublestarSuffix confirms a /**-suffixed kind:dir surface passes against a
// real populated directory — the doublestar suffix must resolve to the directory itself, not a
// literal (and never-existing) "some/dir/**" path on disk.
func TestVerify_KindDir_DoublestarSuffix(t *testing.T) {
	root := mkPopulatedDir(t, "some/dir")
	entries := []model.FileSurfaceEntry{{Path: "some/dir/**", Kind: model.FSDir}}

	res := Verify(os.DirFS(root), entries)

	if !res.OK {
		t.Fatalf("Verify() ok = false, violations = %+v", res.Violations)
	}
}

// TestVerify_KindDir_PlainPathStillWorks locks in that the doublestar-suffix fix leaves the
// pre-existing plain "some/dir" (no /**) spelling of a kind:dir surface unaffected.
func TestVerify_KindDir_PlainPathStillWorks(t *testing.T) {
	root := mkPopulatedDir(t, "some/dir")
	entries := []model.FileSurfaceEntry{{Path: "some/dir", Kind: model.FSDir}}

	res := Verify(os.DirFS(root), entries)

	if !res.OK {
		t.Fatalf("Verify() ok = false, violations = %+v", res.Violations)
	}
}

// TestVerify_KindDir_MissingRequiredDirFails confirms a required kind:dir entry at a directory
// that genuinely doesn't exist still fails — the doublestar-suffix fix must not turn a missing
// directory into a false pass.
func TestVerify_KindDir_MissingRequiredDirFails(t *testing.T) {
	root := t.TempDir() // empty tempdir: "some/missing" never gets created
	entries := []model.FileSurfaceEntry{{Path: "some/missing/**", Kind: model.FSDir, Required: true}}

	res := Verify(os.DirFS(root), entries)

	if res.OK {
		t.Fatal("Verify() ok = true, want false for a missing required directory")
	}
	if len(res.Violations) != 1 || res.Violations[0].Path != "some/missing/**" {
		t.Fatalf("violations = %+v, want one violation for path %q", res.Violations, "some/missing/**")
	}
}

// TestFileSurfaceEntry_HasNoDeletedField guards the deleted+kind:dir invariant this task must
// not touch: the surface package can only special-case a Deleted field if FileSurfaceEntry
// declares one. It doesn't, so verifyDir's doublestar-suffix fix cannot have changed
// deleted-directory handling — there's nothing here to consult.
func TestFileSurfaceEntry_HasNoDeletedField(t *testing.T) {
	if _, ok := reflect.TypeOf(model.FileSurfaceEntry{}).FieldByName("Deleted"); ok {
		t.Fatal("FileSurfaceEntry gained a Deleted field — re-check verifyDir/covers still ignore it for kind:dir before relying on this invariant")
	}
}
