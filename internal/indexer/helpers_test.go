package indexer

import (
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	m        sync.Mutex
	elements []interface{}
}

func (w *capturingWriter) Write(v interface{}) {
	w.m.Lock()
	w.elements = append(w.elements, v)
	w.m.Unlock()
}

func (w *capturingWriter) Flush() error {
	return nil
}

// findDocumentURIByDocumentID returns the URI of the document with the given ID.
func findDocumentURIByDocumentID(elements []interface{}, id uint64) string {
	for _, elem := range elements {
		switch v := elem.(type) {
		case protocol.Document:
			if v.ID == id {
				return v.URI
			}
		}
	}

	return ""
}

// findRangeByID returns the range with the given identifier.
func findRangeByID(elements []interface{}, id uint64) (protocol.Range, bool) {
	for _, elem := range elements {
		switch v := elem.(type) {
		case protocol.Range:
			if v.ID == id {
				return v, true
			}
		}
	}

	return protocol.Range{}, false
}

// findHoverResultByID returns the hover result object with the given identifier.
func findHoverResultByID(elements []interface{}, id uint64) (protocol.HoverResult, bool) {
	for _, elem := range elements {
		switch v := elem.(type) {
		case protocol.HoverResult:
			if v.ID == id {
				return v, true
			}
		}
	}

	return protocol.HoverResult{}, false
}

// findMonikerByID returns the moniker with the given identifier.
func findMonikerByID(elements []interface{}, id uint64) (protocol.Moniker, bool) {
	for _, elem := range elements {
		switch v := elem.(type) {
		case protocol.Moniker:
			if v.ID == id {
				return v, true
			}
		}
	}

	return protocol.Moniker{}, false
}

// findPackageInformationByID returns the moniker with the given identifier.
func findPackageInformationByID(elements []interface{}, id uint64) (protocol.PackageInformation, bool) {
	for _, elem := range elements {
		switch v := elem.(type) {
		case protocol.PackageInformation:
			if v.ID == id {
				return v, true
			}
		}
	}

	return protocol.PackageInformation{}, false
}

// findDefintionRangesByDefinitionResultID returns the ranges attached to the definition result with the given
// identifier.
func findDefintionRangesByDefinitionResultID(elements []interface{}, id uint64) (ranges []protocol.Range) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Item:
			if e.OutV == id {
				for _, inV := range e.InVs {
					if r, ok := findRangeByID(elements, inV); ok {
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
func findReferenceRangesByReferenceResultID(elements []interface{}, id uint64) (ranges []protocol.Range) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Item:
			if e.OutV == id {
				for _, inV := range e.InVs {
					if r, ok := findRangeByID(elements, inV); ok {
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
		case protocol.Contains:
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
func findRange(elements []interface{}, filename string, startLine, startCharacter int) (protocol.Range, bool) {
	for _, elem := range elements {
		switch v := elem.(type) {
		case protocol.Range:
			if v.Start.Line == startLine && v.Start.Character == startCharacter {
				if findDocumentURIContaining(elements, v.ID) == filename {
					return v, true
				}
			}
		}
	}

	return protocol.Range{}, false
}

// findHoverResultByRangeOrResultSetID returns the hover result attached to the range or result
// set with the given identifier.
func findHoverResultByRangeOrResultSetID(elements []interface{}, id uint64) (protocol.HoverResult, bool) {
	// First see if we're attached to a hover result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.TextDocumentHover:
			if e.OutV == id {
				return findHoverResultByID(elements, e.InV)
			}
		}
	}

	// Try to get the hover result of the result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				if result, ok := findHoverResultByRangeOrResultSetID(elements, e.InV); ok {
					return result, true
				}
			}
		}
	}

	return protocol.HoverResult{}, false
}

// findDefinitionRangesByRangeOrResultSetID returns the definition ranges attached to the range or result set
// with the given identifier.
func findDefinitionRangesByRangeOrResultSetID(elements []interface{}, id uint64) (ranges []protocol.Range) {
	// First see if we're attached to definition result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.TextDocumentDefinition:
			if e.OutV == id {
				ranges = append(ranges, findDefintionRangesByDefinitionResultID(elements, e.InV)...)
			}
		}
	}

	// Try to get the definition result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findDefinitionRangesByRangeOrResultSetID(elements, e.InV)...)
			}
		}
	}

	return ranges
}

// findReferenceRangesByRangeOrResultSetID returns the reference ranges attached to the range or result set with
// the given identifier.
func findReferenceRangesByRangeOrResultSetID(elements []interface{}, id uint64) (ranges []protocol.Range) {
	// First see if we're attached to reference result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.TextDocumentReferences:
			if e.OutV == id {
				ranges = append(ranges, findReferenceRangesByReferenceResultID(elements, e.InV)...)
			}
		}
	}

	// Try to get the reference result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findReferenceRangesByRangeOrResultSetID(elements, e.InV)...)
			}
		}
	}

	return ranges
}

// findMonikersByRangeOrReferenceResultID returns the monikers attached to the range or  reference result
// with the given identifier.
func findMonikersByRangeOrReferenceResultID(elements []interface{}, id uint64) (monikers []protocol.Moniker) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.MonikerEdge:
			if e.OutV == id {
				if m, ok := findMonikerByID(elements, e.InV); ok {
					monikers = append(monikers, m)
				}
			}
		}
	}

	// Try to get the reference result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				monikers = append(monikers, findMonikersByRangeOrReferenceResultID(elements, e.InV)...)
			}
		}
	}

	return monikers
}

// findPackageInformationByMonikerID returns the package information vertexes attached to the moniker with the given identifier.
func findPackageInformationByMonikerID(elements []interface{}, id uint64) (packageInformation []protocol.PackageInformation) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.PackageInformationEdge:
			if e.OutV == id {
				if m, ok := findPackageInformationByID(elements, e.InV); ok {
					packageInformation = append(packageInformation, m)
				}
			}
		}
	}

	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.NextMonikerEdge:
			if e.OutV == id {
				packageInformation = append(packageInformation, findPackageInformationByMonikerID(elements, e.InV)...)
			}
		}
	}

	return packageInformation
}
