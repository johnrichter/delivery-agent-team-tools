package surface

import (
	"testing"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
)

// TestVerifyChangedSubsetOfSurface_KindDir_DoublestarSuffix confirms a changed file nested
// under a /**-suffixed kind:dir surface is reported ON-surface, not off-surface — covers must
// resolve the doublestar suffix to the directory prefix it names, matching Verify's behavior
// for the same entry shape.
func TestVerifyChangedSubsetOfSurface_KindDir_DoublestarSuffix(t *testing.T) {
	entries := []model.FileSurfaceEntry{{Path: "some/dir/**", Kind: model.FSDir}}

	res := VerifyChangedSubsetOfSurface([]string{"some/dir/file.go"}, entries)

	if !res.OK {
		t.Fatalf("VerifyChangedSubsetOfSurface() ok = false, off_surface = %v", res.OffSurface)
	}
}

// TestVerifyChangedSubsetOfSurface_KindDir_PlainPathStillWorks locks in that the doublestar-
// suffix fix leaves the pre-existing plain "some/dir" (no /**) spelling unaffected.
func TestVerifyChangedSubsetOfSurface_KindDir_PlainPathStillWorks(t *testing.T) {
	entries := []model.FileSurfaceEntry{{Path: "some/dir", Kind: model.FSDir}}

	res := VerifyChangedSubsetOfSurface([]string{"some/dir/file.go"}, entries)

	if !res.OK {
		t.Fatalf("VerifyChangedSubsetOfSurface() ok = false, off_surface = %v", res.OffSurface)
	}
}

// TestVerifyChangedSubsetOfSurface_KindDir_UnrelatedPathStaysOffSurface confirms the /**-suffix
// fix doesn't broaden coverage beyond the declared directory — a change outside it still
// reports off-surface.
func TestVerifyChangedSubsetOfSurface_KindDir_UnrelatedPathStaysOffSurface(t *testing.T) {
	entries := []model.FileSurfaceEntry{{Path: "some/dir/**", Kind: model.FSDir}}

	res := VerifyChangedSubsetOfSurface([]string{"other/dir/file.go"}, entries)

	if res.OK {
		t.Fatal("VerifyChangedSubsetOfSurface() ok = true, want false for a path outside the declared dir surface")
	}
	if len(res.OffSurface) != 1 || res.OffSurface[0] != "other/dir/file.go" {
		t.Fatalf("off_surface = %v, want [other/dir/file.go]", res.OffSurface)
	}
}
