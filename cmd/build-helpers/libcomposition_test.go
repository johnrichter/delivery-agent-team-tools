package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// sourceFiles returns every non-test .go file under root (relative to this package: cmd/ and
// internal/ both live two levels up from cmd/build-helpers).
func sourceFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

// bhQualifier matches a package-qualified reference to the original bh package (e.g.
// "bh.ExecState", "bh.RecordTask") — the naming pattern a structural port would carry over.
var bhQualifier = regexp.MustCompile(`\bbh\.[A-Z]`)

// TestLibraryComposition_NoStructuralPortResidue asserts the reimplementation neither imports
// the original build-helpers/bh package nor carries its "bh." naming convention — AC3's "not a
// structural port, no bh naming in the new library layer".
func TestLibraryComposition_NoStructuralPortResidue(t *testing.T) {
	fset := token.NewFileSet()
	for _, root := range []string{"../../cmd", "../../internal"} {
		for _, path := range sourceFiles(t, root) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if bhQualifier.Match(raw) {
				t.Errorf("%s: contains a bh.-qualified reference — structural-port naming residue", path)
			}
			f, err := parser.ParseFile(fset, path, raw, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse imports of %s: %v", path, err)
			}
			for _, imp := range f.Imports {
				p, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					continue
				}
				if strings.Contains(p, "build-helpers/bh") {
					t.Errorf("%s: imports %q — the original package this engine replaces", path, p)
				}
			}
		}
	}
}

// libPrefix is the ai-shared-lib Go module namespace every library-first import lives under.
const libPrefix = "github.com/johnrichter/claude-shared-tooling/go/"

// minComposedLibraries is a deliberately conservative floor, not the full catalog: of the
// libraries ai-shared-lib ships for this class of tool (graph, jsondoc, fsx, schema, retrieve,
// ledger, gate, transcript, cost, bandcheck, state, git, docmirror, clikit), ledger/cost/git
// model a SQLite-backed spend index, a ranked findings register, and repo-mutating git
// operations — none of which this CLI's frozen per-subcommand JSON shapes call for; clikit's
// {status,data,errors,caveats} result envelope would itself break every subcommand's existing
// bespoke stdout shape; jsondoc/schema/docmirror were tried and reverted (see
// internal/planops and internal/rendering) because their canonicalization/diagnostics/marker
// behavior changed frozen output. The rest — graph, fsx, retrieve, gate, transcript,
// bandcheck, state — are genuinely composed below; this floor guards against silently
// regressing back to hand-rolled infra for any of them.
const minComposedLibraries = 7

// TestLibraryComposition_UsesSharedLibraries asserts the reimplementation actually composes a
// meaningful number of ai-shared-lib libraries rather than hand-rolling their concerns —
// SC-LIBFIRST's positive half.
func TestLibraryComposition_UsesSharedLibraries(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, root := range []string{"../../cmd", "../../internal"} {
		for _, path := range sourceFiles(t, root) {
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse imports of %s: %v", path, err)
			}
			for _, imp := range f.Imports {
				p, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					continue
				}
				if lib, ok := strings.CutPrefix(p, libPrefix); ok {
					seen[lib] = true
				}
			}
		}
	}
	if len(seen) < minComposedLibraries {
		t.Fatalf("composed %d ai-shared-lib libraries (%v), want >= %d", len(seen), seen, minComposedLibraries)
	}
}
