package indexer

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/packages"
)

// getRepositoryRoot returns the absolute path to the testdata directory of this repository.
func getRepositoryRoot(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("unexpected error getting working directory: %s", err)
	}

	root, err := filepath.Abs(filepath.Join(wd, "../../internal/testdata"))
	if err != nil {
		t.Fatalf("unexpected error getting absolute directory: %s", err)
	}

	return root
}

// getTestPackages loads the testdata package (and subpackages).
func getTestPackages(t *testing.T) []*packages.Package {
	packages, err := packages.Load(
		&packages.Config{Mode: loadMode, Dir: getRepositoryRoot(t)},
		"./...",
	)
	if err != nil {
		t.Fatalf("unexpected error loading packages: %s", err)
	}

	return packages
}

// findDefinitionByName looks for a definition with the given name in the given packages. Returns
// the the object with the matching name and the package that contains it.
func findDefinitionByName(t *testing.T, packages []*packages.Package, name string) (*packages.Package, types.Object) {
	for _, p := range packages {
		for _, def := range p.TypesInfo.Defs {
			if def != nil && def.Name() == name {
				return p, def
			}
		}
	}

	t.Fatalf("failed to find target object")
	return nil, nil
}

// findUseByName looks for a use with the given name in the given packages. Returns the the
// object with the matching name and the package that contains it.
func findUseByName(t *testing.T, packages []*packages.Package, name string) (*packages.Package, types.Object) {
	for _, p := range packages {
		for _, use := range p.TypesInfo.Uses {
			if use.Name() == name {
				return p, use
			}
		}
	}

	t.Fatalf("failed to find target object")
	return nil, nil
}

// getFileContaining returns the file containing the given object.
func getFileContaining(t *testing.T, p *packages.Package, obj types.Object) *ast.File {
	for _, f := range p.Syntax {
		if p.Fset.Position(f.Pos()).Filename == p.Fset.Position(obj.Pos()).Filename {
			return f
		}
	}

	t.Fatalf("failed to find file")
	return nil
}

// preload populates and returns a Preloader instance with the hover text and moniker
// paths for all definitions in in the given packages.
func preload(packages []*packages.Package) *Preloader {
	preloader := newPreloader()
	for _, p := range getAllReferencedPackages(packages) {
		positions := getDefinitionPositions(p)

		for _, f := range p.Syntax {
			preloader.Load(f, positions)
		}
	}

	return preloader
}

// normalizeDocstring removes leading indentation from each line, removes empty lines,
// trims trailing whitespace, and returns the remaining lines joined by a single space.
func normalizeDocstring(s string) string {
	var parts []string
	for _, line := range strings.Split(stripIndent(s), "\n") {
		if line == "" {
			continue
		}

		parts = append(parts, strings.TrimRight(line, " \t"))
	}

	return strings.Join(parts, " ")
}

// stripIndent removes leading indentation from each line of the given text.
func stripIndent(s string) string {
	skip, n := findIndent(s)

	var parts []string
	for _, line := range strings.Split(s, "\n")[skip+1:] {
		if len(line) < n {
			parts = append(parts, "")
		} else {
			parts = append(parts, line[n:])
		}
	}

	return strings.Join(parts, "\n")
}

// findIndent returns the number of empty lines, and the number of leading whitespace characters
// in the first non-empty line of the given string.
func findIndent(s string) (emptyLines int, indent int) {
	for j, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		return j - 1, len(line) - len(strings.TrimLeft(line, " \t"))
	}

	return 0, 0
}

// capturingWriter can be used in place of the protocol.JSONWriter used in the binary. This
// captures each of the emitted objects without serializing them so they can be inspected via
// type by the unit tests of this package.
type capturingWriter struct {
	elements []interface{}
}

func (w *capturingWriter) Write(v interface{}) {
	w.elements = append(w.elements, v)
}

func (w *capturingWriter) Flush() error {
	return nil
}

// findDocumentURIByDocumentID returns the URI of the document with the given ID.
func findDocumentURIByDocumentID(elements []interface{}, id uint64) string {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.Document:
			if v.ID == id {
				return v.URI
			}
		}
	}

	return ""
}

// findRangeByID returns the range with the given identifier.
func findRangeByID(elements []interface{}, id uint64) *protocol.Range {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.Range:
			if v.ID == id {
				return v
			}
		}
	}

	return nil
}

// findHoverResultByID returns the hover result object with the given identifier.
func findHoverResultByID(elements []interface{}, id uint64) *protocol.HoverResult {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.HoverResult:
			if v.ID == id {
				return v
			}
		}
	}

	return nil
}

// findMonikerByID returns the moniker with the given identifier.
func findMonikerByID(elements []interface{}, id uint64) *protocol.Moniker {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.Moniker:
			if v.ID == id {
				return v
			}
		}
	}

	return nil
}

// findPackageInformationByID returns the moniker with the given identifier.
func findPackageInformationByID(elements []interface{}, id uint64) *protocol.PackageInformation {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.PackageInformation:
			if v.ID == id {
				return v
			}
		}
	}

	return nil
}

// findDefintionRangesByDefinitionResultID returns the ranges attached to the definition result with the given
// identifier.
func findDefintionRangesByDefinitionResultID(elements []interface{}, id uint64) (ranges []*protocol.Range) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Item:
			if e.OutV == id {
				for _, inV := range e.InVs {
					if r := findRangeByID(elements, inV); r != nil {
						ranges = append(ranges, r)
					}
				}
			}
		}
	}

	return ranges
}

// findReferenceRangesByReferenceResultID returns the ranges attached to the reference result with the given
// identifier.
func findReferenceRangesByReferenceResultID(elements []interface{}, id uint64) (ranges []*protocol.Range) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Item:
			if e.OutV == id {
				for _, inV := range e.InVs {
					if r := findRangeByID(elements, inV); r != nil {
						ranges = append(ranges, r)
					}
				}
			}
		}
	}

	return ranges
}

// findDocumentURIContaining finds the URI of the document containing the given ID.
func findDocumentURIContaining(elements []interface{}, id uint64) string {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Contains:
			for _, inV := range e.InVs {
				if inV == id {
					return findDocumentURIByDocumentID(elements, e.OutV)
				}
			}
		}
	}

	return ""
}

// findRange returns the range in the given file with the given start line and character.
func findRange(elements []interface{}, filename string, startLine, startCharacter int) *protocol.Range {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.Range:
			if v.Start.Line == startLine && v.Start.Character == startCharacter {
				if findDocumentURIContaining(elements, v.ID) == filename {
					return v
				}
			}
		}
	}

	return nil
}

// findHoverResultByRangeOrResultSetID returns the hover result attached to the range or result
// set with the given identifier.
func findHoverResultByRangeOrResultSetID(elements []interface{}, id uint64) *protocol.HoverResult {
	// First see if we're attached to a hover result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.TextDocumentHover:
			if e.OutV == id {
				return findHoverResultByID(elements, e.InV)
			}
		}
	}

	// Try to get the hover result of the result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Next:
			if e.OutV == id {
				if result := findHoverResultByRangeOrResultSetID(elements, e.InV); result != nil {
					return result
				}
			}
		}
	}

	return nil
}

// findDefinitionRangesByRangeOrResultSetID returns the definition ranges attached to the range or result set
// with the given identifier.
func findDefinitionRangesByRangeOrResultSetID(elements []interface{}, id uint64) (ranges []*protocol.Range) {
	// First see if we're attached to definition result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.TextDocumentDefinition:
			if e.OutV == id {
				ranges = append(ranges, findDefintionRangesByDefinitionResultID(elements, e.InV)...)
			}
		}
	}

	// Try to get the definition result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findDefinitionRangesByRangeOrResultSetID(elements, e.InV)...)
			}
		}
	}

	return ranges
}

// findReferenceRangesByRangeOrResultSetID returns the reference ranges attached to the range or result set with
// the given identifier.
func findReferenceRangesByRangeOrResultSetID(elements []interface{}, id uint64) (ranges []*protocol.Range) {
	// First see if we're attached to reference result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.TextDocumentReferences:
			if e.OutV == id {
				ranges = append(ranges, findReferenceRangesByReferenceResultID(elements, e.InV)...)
			}
		}
	}

	// Try to get the reference result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findReferenceRangesByRangeOrResultSetID(elements, e.InV)...)
			}
		}
	}

	return ranges
}

// findMonikersByRangeOrReferenceResultID returns the monikers attached to the range or  reference result
// with the given identifier.
func findMonikersByRangeOrReferenceResultID(elements []interface{}, id uint64) (monikers []*protocol.Moniker) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.MonikerEdge:
			if e.OutV == id {
				if m := findMonikerByID(elements, e.InV); m != nil {
					monikers = append(monikers, m)
				}
			}
		}
	}

	// Try to get the reference result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Next:
			if e.OutV == id {
				monikers = append(monikers, findMonikersByRangeOrReferenceResultID(elements, e.InV)...)
			}
		}
	}

	return monikers
}

// findPackageInformationByMonikerID returns the package information vertexes attached to the moniker with the given identifier.
func findPackageInformationByMonikerID(elements []interface{}, id uint64) (packageInformation []*protocol.PackageInformation) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.PackageInformationEdge:
			if e.OutV == id {
				if m := findPackageInformationByID(elements, e.InV); m != nil {
					packageInformation = append(packageInformation, m)
				}
			}
		}
	}

	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.NextMonikerEdge:
			if e.OutV == id {
				packageInformation = append(packageInformation, findPackageInformationByMonikerID(elements, e.InV)...)
			}
		}
	}

	return packageInformation
}
