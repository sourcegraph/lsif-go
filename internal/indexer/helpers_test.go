package indexer

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	protocol "github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
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

var getTestPackagesOnce sync.Once
var cachedTestPackages []*packages.Package

// getTestPackages loads the testdata package (and subpackages).
func getTestPackages(t *testing.T) []*packages.Package {
	getTestPackagesOnce.Do(func() {
		var err error

		cachedTestPackages, err = packages.Load(
			&packages.Config{Mode: loadMode, Dir: path.Join(getRepositoryRoot(t), "fixtures")},
			"./...",
		)
		if err != nil {
			t.Fatalf("unexpected error loading packages: %s", err)
		}
	})

	return cachedTestPackages
}

// findDefinitionByName looks for a definition with the given name in the given packages. Returns
// the the object with the matching name and the package that contains it.
func findDefinitionByName(t *testing.T, packages []*packages.Package, name string) (*packages.Package, types.Object) {
	for _, p := range packages {
		idents := make([]*ast.Ident, 0, len(p.TypesInfo.Defs))
		for ident := range p.TypesInfo.Defs {
			idents = append(idents, ident)
		}
		sort.Slice(idents, func(i, j int) bool {
			return idents[i].Pos() < idents[j].Pos()
		})

		for _, ident := range idents {
			def := p.TypesInfo.Defs[ident]
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

	// Quicker access for special types of nodes.
	// Could add other node types if desired.
	ranges    map[uint64]protocol.Range
	documents map[uint64]protocol.Document
	contains  map[uint64]uint64
}

func (w *capturingWriter) Write(v interface{}) {
	w.m.Lock()
	w.elements = append(w.elements, v)

	// Store special elements for quicker access
	switch elem := v.(type) {
	case protocol.Range:
		w.ranges[elem.ID] = elem
	case protocol.Document:
		w.documents[elem.ID] = elem
	case protocol.Contains:
		// A range is always only contained by one document.
		for _, inV := range elem.InVs {
			w.contains[inV] = elem.OutV
		}
	}

	w.m.Unlock()
}

func (w *capturingWriter) Flush() error {
	return nil
}

// findDocumentURIByDocumentID returns the URI of the document with the given ID.
func findDocumentURIByDocumentID(w *capturingWriter, id uint64) string {
	document, ok := w.documents[id]
	if !ok {
		return ""
	}

	return document.URI
}

// findRangeByID returns the range with the given identifier.
func findRangeByID(w *capturingWriter, id uint64) (protocol.Range, bool) {
	r, ok := w.ranges[id]

	if !ok {
		return protocol.Range{}, false
	}

	return r, true
}

// findHoverResultByID returns the hover result object with the given identifier.
func findHoverResultByID(w *capturingWriter, id uint64) (protocol.HoverResult, bool) {
	for _, elem := range w.elements {
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
func findMonikerByID(w *capturingWriter, id uint64) (protocol.Moniker, bool) {
	for _, elem := range w.elements {
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
func findPackageInformationByID(w *capturingWriter, id uint64) (protocol.PackageInformation, bool) {
	for _, elem := range w.elements {
		switch v := elem.(type) {
		case protocol.PackageInformation:
			if v.ID == id {
				return v, true
			}
		}
	}

	return protocol.PackageInformation{}, false
}

// findRangesByResultID returns the ranges attached to the result set with the given
// identifier.
func findRangesByResultID(w *capturingWriter, id uint64) (ranges []protocol.Range) {
	elements := w.elements

	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Item:
			if e.OutV == id {
				for _, inV := range e.InVs {
					if r, ok := findRangeByID(w, inV); ok {
						ranges = append(ranges, r)
					}
				}
			}
		}
	}

	return ranges
}

// findDocumentURIContaining finds the URI of the document containing the given ID.
func findDocumentURIContaining(w *capturingWriter, id uint64) string {
	documentID, ok := w.contains[id]
	if !ok {
		return ""
	}

	return findDocumentURIByDocumentID(w, documentID)
}

// findRange returns the range in the given file with the given start line and character.
func findRange(w *capturingWriter, filename string, startLine, startCharacter int) (protocol.Range, bool) {
	for _, elem := range w.elements {
		switch v := elem.(type) {
		case protocol.Range:
			if v.Start.Line == startLine && v.Start.Character == startCharacter {
				if findDocumentURIContaining(w, v.ID) == filename {
					return v, true
				}
			}
		}
	}

	return protocol.Range{}, false
}

// findAllRanges returns a list of ranges in the given file with the given start line and character.
// This can be used to confirm that there is only one range that would match at a particular location
func findAllRanges(w *capturingWriter, filename string, startLine, startCharacter int) []protocol.Range {
	ranges := []protocol.Range{}
	for _, elem := range w.elements {
		switch v := elem.(type) {
		case protocol.Range:
			if v.Start.Line == startLine && v.Start.Character == startCharacter {
				if findDocumentURIContaining(w, v.ID) == filename {
					ranges = append(ranges, v)
				}
			}
		}
	}

	return ranges
}

// findHoverResultByRangeOrResultSetID returns the hover result attached to the range or result
// set with the given identifier.
func findHoverResultByRangeOrResultSetID(w *capturingWriter, id uint64) (protocol.HoverResult, bool) {
	// First see if we're attached to a hover result directly
	for _, elem := range w.elements {
		switch e := elem.(type) {
		case protocol.TextDocumentHover:
			if e.OutV == id {
				return findHoverResultByID(w, e.InV)
			}
		}
	}

	// Try to get the hover result of the result set attached to the given range or result set
	for _, elem := range w.elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				if result, ok := findHoverResultByRangeOrResultSetID(w, e.InV); ok {
					return result, true
				}
			}
		}
	}

	return protocol.HoverResult{}, false
}

// findDefinitionRangesByRangeOrResultSetID returns the definition ranges attached to the range or result set
// with the given identifier.
func findDefinitionRangesByRangeOrResultSetID(w *capturingWriter, id uint64) (ranges []protocol.Range) {
	elements := w.elements

	// First see if we're attached to definition result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.TextDocumentDefinition:
			if e.OutV == id {
				ranges = append(ranges, findRangesByResultID(w, e.InV)...)
			}
		}
	}

	// Try to get the definition result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findDefinitionRangesByRangeOrResultSetID(w, e.InV)...)
			}
		}
	}

	return ranges
}

// findReferenceRangesByRangeOrResultSetID returns the reference ranges attached to the range or result set with
// the given identifier.
func findReferenceRangesByRangeOrResultSetID(w *capturingWriter, id uint64) (ranges []protocol.Range) {
	elements := w.elements

	// First see if we're attached to reference result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.TextDocumentReferences:
			if e.OutV == id {
				ranges = append(ranges, findRangesByResultID(w, e.InV)...)
			}
		}
	}

	// Try to get the reference result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findReferenceRangesByRangeOrResultSetID(w, e.InV)...)
			}
		}
	}

	return ranges
}

// findImplementationRangesByRangeOrResultSetID returns the implementation ranges attached to the range or result set with
// the given identifier.
func findImplementationRangesByRangeOrResultSetID(w *capturingWriter, id uint64) (ranges []protocol.Range) {
	elements := w.elements

	// First see if we're attached to implementation result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.TextDocumentImplementation:
			if e.OutV == id {
				ranges = append(ranges, findRangesByResultID(w, e.InV)...)
			}
		}
	}

	// Try to get the implementation result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findImplementationRangesByRangeOrResultSetID(w, e.InV)...)
			}
		}
	}

	return ranges
}

// findMonikersByRangeOrReferenceResultID returns the monikers attached to the range or  reference result
// with the given identifier.
func findMonikersByRangeOrReferenceResultID(w *capturingWriter, id uint64) (monikers []protocol.Moniker) {
	for _, elem := range w.elements {
		switch e := elem.(type) {
		case protocol.MonikerEdge:
			if e.OutV == id {
				if m, ok := findMonikerByID(w, e.InV); ok {
					monikers = append(monikers, m)
				}
			}
		}
	}

	// Try to get the reference result of a result set attached to the given range or result set
	for _, elem := range w.elements {
		switch e := elem.(type) {
		case protocol.Next:
			if e.OutV == id {
				monikers = append(monikers, findMonikersByRangeOrReferenceResultID(w, e.InV)...)
			}
		}
	}

	return monikers
}

// findPackageInformationByMonikerID returns the package information vertexes attached to the moniker with the given identifier.
func findPackageInformationByMonikerID(w *capturingWriter, id uint64) (packageInformation []protocol.PackageInformation) {
	for _, elem := range w.elements {
		switch e := elem.(type) {
		case protocol.PackageInformationEdge:
			if e.OutV == id {
				if m, ok := findPackageInformationByID(w, e.InV); ok {
					packageInformation = append(packageInformation, m)
				}
			}
		}
	}

	for _, elem := range w.elements {
		switch e := elem.(type) {
		case protocol.NextMonikerEdge:
			if e.OutV == id {
				packageInformation = append(packageInformation, findPackageInformationByMonikerID(w, e.InV)...)
			}
		}
	}

	return packageInformation
}

func splitMarkupContent(value string) []string {
	return strings.Split(value, "\n\n---\n\n")
}

func unCodeFence(value string) string {
	return strings.Replace(strings.Replace(value, "```go\n", "", -1), "\n```", "", -1)
}

func compareRange(t *testing.T, r protocol.Range, startLine, startCharacter, endLine, endCharacter int) {
	if r.Start.Line != startLine || r.Start.Character != startCharacter || r.End.Line != endLine || r.End.Character != endCharacter {
		t.Fatalf(
			"incorrect range. want=[%d:%d,%d:%d) have=[%d:%d,%d:%d)",
			startLine, startCharacter, endLine, endCharacter,
			r.Start.Line, r.Start.Character, r.End.Line, r.End.Character,
		)
	}
}

func stringifyRange(r protocol.Range) string {
	return fmt.Sprintf("%d:%d-%d:%d", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
}

func mustRange(t *testing.T, ranges []protocol.Range, needle string) protocol.Range {
	for _, r := range ranges {
		if stringifyRange(r) == needle {
			return r
		}
	}
	t.Fatalf("mustRange: range not found %s", needle)
	panic("this should never happen")
}

// assertRanges throws an error if the expected and actual range sets are not equal.
func assertRanges(t *testing.T, actual []protocol.Range, expected []string, msg string) {
	actuals := map[string]struct{}{}
	duplicates := []string{}
	for _, r := range actual {
		if _, ok := actuals[stringifyRange(r)]; ok {
			duplicates = append(duplicates, stringifyRange(r))
		} else {
			actuals[stringifyRange(r)] = struct{}{}
		}
	}

	expecteds := map[string]struct{}{}
	for _, r := range expected {
		expecteds[r] = struct{}{}
	}

	// Detect missing ranges
	missings := []string{}
	for r := range expecteds {
		if _, ok := actuals[r]; !ok {
			missings = append(missings, r)
		}
	}

	// Detect extra ranges
	extras := []string{}
	for r := range actuals {
		if _, ok := expecteds[r]; !ok {
			extras = append(extras, r)
		}
	}

	// Report differences
	errors := []string{msg + ":"}
	if len(missings) > 0 {
		errors = append(errors, fmt.Sprintf("missing [%s]", strings.Join(missings, ", ")))
	}
	if len(extras) > 0 {
		errors = append(errors, fmt.Sprintf("extra [%s]", strings.Join(extras, ", ")))
	}
	if len(duplicates) > 0 {
		errors = append(errors, fmt.Sprintf("duplicates [%s]", strings.Join(duplicates, ", ")))
	}
	if len(errors) > 1 {
		t.Fatalf("%s: %s", msg, strings.Join(errors, " "))
	}
}
