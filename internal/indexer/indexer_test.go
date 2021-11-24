package indexer

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hexops/autogold"
	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/output"
	"github.com/sourcegraph/lsif-static-doc/staticdoc"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer"
)

var dependencies = map[string]gomod.GoModule{
	"github.com/sourcegraph/lsif-go": {Name: "github.com/sourcegraph/lsif-go", Version: "dev"},
	"github.com/golang/go":           {Name: "github.com/golang/go", Version: "go1.16"},
}

var projectDependencies = []string{"std"}

func TestIndexer(t *testing.T) {
	w := &capturingWriter{
		ranges:    map[uint64]protocol.Range{},
		documents: map[uint64]protocol.Document{},
		contains:  map[uint64]uint64{},
	}

	projectRoot := path.Join(getRepositoryRoot(t), "fixtures")
	indexer := New(
		"/dev/github.com/sourcegraph/lsif-go/internal/testdata/fixtures",
		"github.com/sourcegraph/lsif-go",
		projectRoot,
		protocol.ToolInfo{Name: "lsif-go", Version: "dev"},
		"testdata",
		"0.0.1",
		dependencies,
		projectDependencies,
		w,
		NewPackageDataCache(),
		output.Options{},
		NewGenerationOptions(),
	)

	if err := indexer.Index(); err != nil {
		t.Fatalf("unexpected error indexing testdata: %s", err.Error())
	}

	t.Run("check Parallel function hover text", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "parallel.go"), 13, 5)

		hoverResult, ok := findHoverResultByRangeOrResultSetID(w, r.ID)
		markupContentSegments := splitMarkupContent(hoverResult.Result.Contents.(protocol.MarkupContent).Value)

		if !ok || len(markupContentSegments) < 2 {
			t.Fatalf("could not find hover text: %v", markupContentSegments)
		}

		expectedType := `func Parallel(ctx Context, fns ...ParallelizableFunc) error`
		if value := unCodeFence(markupContentSegments[0]); value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}

		expectedDocumentation := normalizeDocstring(`
			Parallel invokes each of the given parallelizable functions in their own goroutines and
			returns the first error to occur. This method will block until all goroutines have returned.
		`)
		if value := normalizeDocstring(markupContentSegments[1]); value != expectedDocumentation {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", expectedDocumentation, value)
		}
	})

	t.Run("declares definitions for 'package testdata' identifiers", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "main.go"), 2, 8)

		assertRanges(t, w, findDefinitionRangesByRangeOrResultSetID(w, r.ID), []string{"2:8-2:16"}, "definition")

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		if len(monikers) != 1 {
			t.Errorf("Monikers: %+v\n", monikers)
		}

		moniker := monikers[0]
		value := moniker.Identifier
		expectedLabel := "github.com/sourcegraph/lsif-go/internal/testdata/fixtures"
		if value != expectedLabel {
			t.Errorf("incorrect moniker identifier. want=%q have=%q", expectedLabel, value)
		}
	})

	t.Run("declares definitions for nested 'package *' identifiers", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "internal", "secret", "doc.go"), 1, 8)

		assertRanges(t, w, findDefinitionRangesByRangeOrResultSetID(w, r.ID), []string{"1:8-1:14"}, "definition")

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		if len(monikers) != 1 {
			t.Errorf("Monikers: %+v\n", monikers)
		}

		moniker := monikers[0]
		value := moniker.Identifier
		expectedLabel := "github.com/sourcegraph/lsif-go/internal/testdata/fixtures/internal/secret"
		if value != expectedLabel {
			t.Errorf("incorrect moniker identifier. want=%q have=%q", expectedLabel, value)
		}
	})

	t.Run("check external package hover text", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "parallel.go"), 4, 2)

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		if len(monikers) != 1 {
			t.Fatalf("found too many monikers: %+v\n", monikers)
		}

		// Only important part is linking to the correct moniker.
		// Hover results will be linked accordingly
		moniker := monikers[0]
		expectedMoniker := "github.com/golang/go/std/sync"
		if moniker.Identifier != expectedMoniker {
			t.Errorf("incorrect moniker identifier. want=%q have=%q", expectedMoniker, moniker.Identifier)
		}

		hoverResult, ok := findHoverResultByRangeOrResultSetID(w, r.ID)
		markupContentSegments := splitMarkupContent(hoverResult.Result.Contents.(protocol.MarkupContent).Value)
		if !ok || len(markupContentSegments) < 2 {
			t.Fatalf("could not find hover text: %v", markupContentSegments)
		}

		expectedType := `package "sync"`
		if value := unCodeFence(markupContentSegments[0]); value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}

		expectedDocumentation := normalizeDocstring(`
			Package sync provides basic synchronization primitives such as mutual exclusion locks.
			Other than the Once and WaitGroup types, most are intended for use by low-level library routines.
			Higher-level synchronization is better done via channels and communication.
			Values containing the types defined in this package should not be copied.
		`)
		if value := normalizeDocstring(markupContentSegments[1]); value != expectedDocumentation {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", expectedDocumentation, value)
		}
	})

	t.Run("check errs definition", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "parallel.go"), 21, 3)

		assertRanges(t, w, findDefinitionRangesByRangeOrResultSetID(w, r.ID), []string{"15:1-15:5"}, "definition")
	})

	t.Run("check wg references", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "parallel.go"), 26, 1)

		references := findReferenceRangesByRangeOrResultSetID(w, r.ID)
		if len(references) != 4 {
			t.Fatalf("incorrect reference count. want=%d have=%d", 4, len(references))
		}

		sort.Slice(references, func(i, j int) bool { return references[i].Start.Line < references[j].Start.Line })

		compareRange(t, references[0], 14, 5, 14, 7) // var wg sync.WaitGroup
		compareRange(t, references[1], 18, 2, 18, 4) // wg.Add(1)
		compareRange(t, references[2], 22, 3, 22, 5) // wg.Done()
		compareRange(t, references[3], 26, 1, 26, 3) // wg.Wait()
	})

	t.Run("check NestedB monikers", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "data.go"), 27, 3)

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		if len(monikers) != 1 {
			t.Fatalf("incorrect moniker count. want=%d have=%d", 1, len(monikers))
		}

		if value := monikers[0].Scheme; value != "gomod" {
			t.Errorf("incorrect scheme. want=%q have=%q", "gomod", value)
		}

		expectedIdentifier := "github.com/sourcegraph/lsif-go/internal/testdata/fixtures:TestStruct.FieldWithAnonymousType.NestedB"
		if value := monikers[0].Identifier; value != expectedIdentifier {
			t.Errorf("incorrect identifier. want=%q have=%q", expectedIdentifier, value)
		}
	})

	t.Run("check typeswitch", func(t *testing.T) {
		definition := mustRange(t, w, "file://"+filepath.Join(projectRoot, "typeswitch.go"), 3, 8)
		intReference := mustRange(t, w, "file://"+filepath.Join(projectRoot, "typeswitch.go"), 5, 9)
		boolReference := mustRange(t, w, "file://"+filepath.Join(projectRoot, "typeswitch.go"), 7, 10)

		//
		// Check definition links

		assertRanges(t, w, findDefinitionRangesByRangeOrResultSetID(w, intReference.ID), []string{"3:8-3:21"}, "references")

		//
		// Check reference links

		references := findReferenceRangesByRangeOrResultSetID(w, definition.ID)
		if len(references) != 3 {
			t.Fatalf("incorrect reference count. want=%d have=%d", 2, len(references))
		}

		sort.Slice(references, func(i, j int) bool { return references[i].Start.Line < references[j].Start.Line })
		compareRange(t, references[0], 3, 8, 3, 21)
		compareRange(t, references[1], 5, 9, 5, 22)
		compareRange(t, references[2], 7, 10, 7, 23)

		//
		// Check hover texts

		// TODO(efritz) - update test here if we emit hover text for the header

		intReferenceHoverResult, ok := findHoverResultByRangeOrResultSetID(w, intReference.ID)
		markupContentSegments := splitMarkupContent(intReferenceHoverResult.Result.Contents.(protocol.MarkupContent).Value)
		if !ok || len(markupContentSegments) < 1 {
			t.Fatalf("could not find hover text")
		}

		expectedType := `var concreteValue int`
		if value := unCodeFence(markupContentSegments[0]); value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}

		boolReferenceHoverResult, ok := findHoverResultByRangeOrResultSetID(w, boolReference.ID)
		markupContentSegments = splitMarkupContent(boolReferenceHoverResult.Result.Contents.(protocol.MarkupContent).Value)
		if !ok || len(markupContentSegments) < 1 {
			t.Fatalf("could not find hover text")
		}

		expectedType = `var concreteValue bool`
		if value := unCodeFence(markupContentSegments[0]); value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}
	})

	t.Run("check typealias", func(t *testing.T) {
		typealiasFile := "file://" + filepath.Join(projectRoot, "typealias.go")

		r := mustRange(t, w, typealiasFile, 7, 5)

		assertRanges(t, w, findDefinitionRangesByRangeOrResultSetID(w, r.ID), []string{"7:5-7:17"}, "definition")

		hover, ok := findHoverResultByRangeOrResultSetID(w, r.ID)
		markupContentSegments := splitMarkupContent(hover.Result.Contents.(protocol.MarkupContent).Value)
		if !ok || len(markupContentSegments) < 3 {
			t.Fatalf("incorrect hover text count. want=%d have=%d: %v", 3, len(markupContentSegments), markupContentSegments)
		}

		expectedType := `type SecretBurger = secret.Burger`
		if value := unCodeFence(markupContentSegments[0]); value != expectedType {
			t.Errorf("incorrect hover text type. want=%s have=%s", expectedType, value)
		}

		expectedDocumentation := normalizeDocstring(`
				Type aliased doc
			`)
		if value := normalizeDocstring(markupContentSegments[1]); value != expectedDocumentation {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", expectedDocumentation, value)
		}

		expectedUnderlyingType := stripIndent(`
			struct {
					Field int
			}`)
		if value := strings.ReplaceAll(unCodeFence(markupContentSegments[2]), "  ", "\t"); value != expectedUnderlyingType {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", expectedUnderlyingType, value)
		}
	})

	t.Run("check typealias reference", func(t *testing.T) {
		typealiasFile := "file://" + filepath.Join(projectRoot, "typealias.go")

		r := mustRange(t, w, typealiasFile, 7, 27)

		assertRanges(t, w, findDefinitionRangesByRangeOrResultSetID(w, r.ID), []string{"6:5-6:11"}, "definition")

		p, _ := findDefinitionByName(t, indexer.packages, "Burger")
		if p.Name != "secret" {
			t.Fatalf("incorrect definition source package. want=%s have=%s", "secret", p.Name)
		}

		hover, ok := findHoverResultByRangeOrResultSetID(w, r.ID)
		markupContentSegments := splitMarkupContent(hover.Result.Contents.(protocol.MarkupContent).Value)
		if !ok || len(markupContentSegments) < 3 {
			t.Fatalf("incorrect hover text count. want=%d have=%d: %v", 3, len(markupContentSegments), markupContentSegments)
		}

		expectedType := `type Burger struct`
		if value := unCodeFence(markupContentSegments[0]); value != expectedType {
			t.Errorf("incorrect hover text type. want=%s have=%s", expectedType, value)
		}

		expectedDocumentation := normalizeDocstring(`
				Original doc
			`)
		if value := normalizeDocstring(markupContentSegments[1]); value != expectedDocumentation {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", expectedDocumentation, value)
		}

		expectedUnderlyingType := stripIndent(`
			struct {
					Field int
			}`)
		if value := strings.ReplaceAll(unCodeFence(markupContentSegments[2]), "  ", "\t"); value != expectedUnderlyingType {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", expectedUnderlyingType, value)
		}
	})

	t.Run("check_typealias anonymous struct", func(t *testing.T) {
		typealiasFile := "file://" + filepath.Join(projectRoot, "typealias.go")

		r := mustRange(t, w, typealiasFile, 9, 5)

		assertRanges(t, w, findDefinitionRangesByRangeOrResultSetID(w, r.ID), []string{"9:5-9:14"}, "definition")

		hover, ok := findHoverResultByRangeOrResultSetID(w, r.ID)
		markupContentSegments := splitMarkupContent(hover.Result.Contents.(protocol.MarkupContent).Value)
		if !ok || len(markupContentSegments) < 2 {
			t.Fatalf("incorrect hover text count. want=%d have=%d: %v", 2, len(markupContentSegments), markupContentSegments)
		}

		expectedType := `type BadBurger = struct`
		if value := unCodeFence(markupContentSegments[0]); value != expectedType {
			t.Errorf("incorrect hover text type. want=%s have=%s", expectedType, value)
		}

		expectedUnderlyingType := stripIndent(`
			struct {
					Field string
			}`)
		if value := strings.ReplaceAll(unCodeFence(markupContentSegments[1]), "  ", "\t"); value != expectedUnderlyingType {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", expectedUnderlyingType, value)
		}

		r, ok = findRange(w, typealiasFile, 9, 17)
		if ok {
			t.Fatalf("found range for anonymous struct when not expected")
		}
	})

	t.Run("check nested struct definition", func(t *testing.T) {
		ranges := findAllRanges(w, "file://"+filepath.Join(projectRoot, "composite.go"), 11, 1)
		if len(ranges) != 1 {
			t.Fatalf("found more than one range for a non-selector nested struct: %v", ranges)
		}

		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "composite.go"), 11, 1)

		assertRanges(
			t,
			w,
			findDefinitionRangesByRangeOrResultSetID(w, r.ID),
			[]string{
				// Original definition
				"4:5-4:10",
				// Definition through the moniker
				"11:1-11:6",
			},
			"definitions",
		)

		// Expect to find the reference from the definition and for the time we instantiate it in the function.
		references := findReferenceRangesByRangeOrResultSetID(w, r.ID)
		if len(references) != 2 {
			t.Fatalf("incorrect references count. want=%d have=%d", 2, len(references))
		}

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		if len(monikers) != 1 {
			t.Fatalf("incorrect references count. want=%d have=%d %+v", 2, len(monikers), monikers)
		}

		moniker := monikers[0]
		identifier := moniker.Identifier

		expectedIdentifier := "github.com/sourcegraph/lsif-go/internal/testdata/fixtures:Outer.Inner"
		if identifier != expectedIdentifier {
			t.Fatalf("incorrect moniker identifier. want=%s have=%s", expectedIdentifier, identifier)
		}
	})

	t.Run("check named import definition: non-'.' import", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "named_import.go"), 4, 1)

		x := findDefinitionRangesByRangeOrResultSetID(w, r.ID)
		// TODO 2 definitions are emitted here but have the same range. Seems like a bug.
		assertRanges(t, w, []protocol.Range{x[0]}, []string{"4:1-4:2"}, "definitions")
	})

	t.Run("check named import reference: non-'.' import", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "named_import.go"), 4, 4)

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		if len(monikers) != 1 {
			t.Fatalf("Failed to get the expected single moniker: %+v\n", monikers)
		}

		moniker := monikers[0]
		identifier := moniker.Identifier

		expectedIdentifier := "github.com/golang/go/std/net/http"
		if identifier != expectedIdentifier {
			t.Fatalf("incorrect moniker identifier. want=%s have=%s", expectedIdentifier, identifier)
		}
	})

	t.Run("check named import definition: . import", func(t *testing.T) {
		// There should be no range generated for the `.` in the import.
		_, ok := findRange(w, "file://"+filepath.Join(projectRoot, "named_import.go"), 3, 1)
		if ok {
			t.Fatalf("could not find target range")
		}
	})

	t.Run("check named import reference: . import", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "named_import.go"), 3, 4)

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		if len(monikers) != 1 {
			t.Fatalf("Failed to get the expected single moniker: %+v\n", monikers)
		}

		moniker := monikers[0]
		identifier := moniker.Identifier

		expectedIdentifier := "github.com/golang/go/std/fmt"
		if identifier != expectedIdentifier {
			t.Fatalf("incorrect moniker identifier. want=%s have=%s", expectedIdentifier, identifier)
		}
	})

	t.Run("check external nested struct definition", func(t *testing.T) {
		ranges := findAllRanges(w, "file://"+filepath.Join(projectRoot, "external_composite.go"), 5, 1)
		assertRanges(t, w, ranges, []string{"5:1-5:5", "5:1-5:13"}, "http and http.Handler")
		// line: http.Handler
		//       ^^^^------------ httpRange, for http package reference
		//       ^^^^^^^^^^^^---- anonymousFieldRange, for http.Handler, the entire definition
		//
		//            ^^^^^^^---- Separate range, for Handler reference
		// See docs/structs.md
		httpRange := mustGetRangeInSlice(t, ranges, "5:1-5:5")
		anonymousFieldRange := mustGetRangeInSlice(t, ranges, "5:1-5:13")

		assertRanges(t, w, findDefinitionRangesByRangeOrResultSetID(w, anonymousFieldRange.ID), []string{"5:1-5:13"}, "definition")

		monikers := findMonikersByRangeOrReferenceResultID(w, anonymousFieldRange.ID)
		if len(monikers) != 1 {
			t.Fatalf("incorrect monikers count. want=%d have=%d %+v", 1, len(monikers), monikers)
		}

		moniker := monikers[0]
		identifier := moniker.Identifier

		expectedIdentifier := "github.com/sourcegraph/lsif-go/internal/testdata/fixtures:NestedHandler.Handler"
		if identifier != expectedIdentifier {
			t.Fatalf("incorrect moniker identifier. want=%s have=%s", expectedIdentifier, identifier)
		}

		// Check to make sure that the http range still correctly links to the external package.
		httpMonikers := findMonikersByRangeOrReferenceResultID(w, httpRange.ID)
		if len(httpMonikers) != 1 {
			t.Fatalf("incorrect http monikers count. want=%d have=%d %+v", 1, len(httpMonikers), httpMonikers)
		}

		httpIdentifier := httpMonikers[0].Identifier
		expectedHttpIdentifier := "github.com/golang/go/std/net/http"
		if httpIdentifier != expectedHttpIdentifier {
			t.Fatalf("incorrect moniker identifier. want=%s have=%s", expectedHttpIdentifier, httpIdentifier)
		}
	})

	t.Run("should find implementations of an interface", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 4, 5)

		assertRanges(t, w, findImplementationRangesByRangeOrResultSetID(w, r.ID), []string{"12:5-12:7", "16:5-16:7", "22:5-22:8", "21:5-21:7"}, "implementations of I1")
	})

	t.Run("should find interfaces that a type implements", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 21, 5)

		assertRanges(t, w, findImplementationRangesByRangeOrResultSetID(w, r.ID), []string{"4:5-4:7"}, "what A1 implements")
	})

	t.Run("should find interfaces that a type implements", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 21, 5)

		assertRanges(t, w, findImplementationRangesByRangeOrResultSetID(w, r.ID), []string{"4:5-4:7"}, "what A1 implements")
	})

	t.Run("should emit an item edge with :document set to the target range's document", func(t *testing.T) {
		checkItemDocuments(t, w)
	})

	t.Run("should not find unexported implementations", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "pkg/pkg.go"), 2, 5)
		assertRanges(t, w, findImplementationRangesByRangeOrResultSetID(w, r.ID), []string{"implementations.go:28:5-28:32"}, "interfaces that pkg/pkg.go:Foo implements")
	})

	t.Run("should find implementations of an interface: method shared", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 5, 1)

		assertRanges(t, w, findImplementationRangesByRangeOrResultSetID(w, r.ID), []string{"14:12-14:14", "18:12-18:14"}, "SingleMethod implementations")
	})

	t.Run("should find implementations of an interface: method SingleMethod", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementation_methods.go"), 3, 1)

		assertRanges(t, w, findImplementationRangesByRangeOrResultSetID(w, r.ID), []string{"8:25-8:37"}, "SingleMethod implementations")
	})

	t.Run("should find implementations of an interface: method SingleMethodTwoImpl", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementation_methods.go"), 11, 1)

		assertRanges(t, w, findImplementationRangesByRangeOrResultSetID(w, r.ID), []string{"16:18-16:37", "20:18-20:37"}, "SingleMethodTwoImpl implementations")
	})

	t.Run("should emit an implementation moniker for an interface from a dependency", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 32, 5)

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		for _, m := range monikers {
			if m.Kind == "implementation" && m.Identifier == "github.com/golang/go/std/io:Closer" {
				return
			}
		}

		t.Fatalf("expected github.com/golang/go/std/io:Closer implementation moniker, got %+v", monikers)
	})

	t.Run("should emit an implementation moniker for an interface method from a dependency", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 36, 13)

		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		for _, m := range monikers {
			if m.Kind == "implementation" && m.Identifier == "github.com/golang/go/std/io:Closer.Close" {
				return
			}
		}

		t.Fatalf("expected github.com/golang/go/std/io:Closer.Close implementation moniker, got %+v", monikers)
	})

	t.Run("implementations: shared & distinct", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 50, 15)

		// Make sure you're on Between.Shared
		monikers := findMonikersByRangeOrReferenceResultID(w, r.ID)
		for _, m := range monikers {
			if m.Kind != "export" ||
				m.Identifier != "github.com/sourcegraph/lsif-go/internal/testdata/fixtures:Between.Shared" {

				t.Fatalf("Unexpect Moniker: %v\n", monikers)
			}
		}

		// Should have two implementations here, one from SharedOne and the other SharedTwo
		assertRanges(
			t,
			w,
			findImplementationRangesByRangeOrResultSetID(w, r.ID),
			[]string{"39:1-39:7", "44:1-44:7"},
			"Between.Shared Implementations",
		)

		// But when we look at the implementations from SharedOne, we should only find thing.
		sharedOneRange := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 39, 1)
		assertRanges(
			t,
			w,
			findImplementationRangesByRangeOrResultSetID(w, sharedOneRange.ID),
			[]string{"50:15-50:21"},
			"SharedOne.Shared -> Between.Shared",
		)

		// And same for shared two
		sharedTwoRange := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 44, 1)
		assertRanges(
			t,
			w,
			findImplementationRangesByRangeOrResultSetID(w, sharedTwoRange.ID),
			[]string{"50:15-50:21"},
			"SharedTwo.Shared -> Between.Shared",
		)
	})

	t.Run("implementations: finds implementations in function signature", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 54, 23)
		assertRanges(
			t,
			w,
			findImplementationRangesByRangeOrResultSetID(w, r.ID),
			[]string{"48:5-48:12"},
			"(shared SharedOne.Shared) -> Between.Shared",
		)

	})

	t.Run("implementations: finds implementations on references", func(t *testing.T) {
		r := mustRange(t, w, "file://"+filepath.Join(projectRoot, "implementations.go"), 55, 8)

		// Should have two implementations here, one from SharedOne and the other SharedTwo
		assertRanges(
			t,
			w,
			findImplementationRangesByRangeOrResultSetID(w, r.ID),
			[]string{"50:15-50:21"},
			"Between.Shared Implementations",
		)
	})
}

func TestIndexer_documentation(t *testing.T) {
	projectRoot := path.Join(getRepositoryRoot(t), "documentation")
	for _, tst := range []struct {
		name                        string
		repositoryRoot, projectRoot string
		short                       bool
	}{
		{
			name:           "testdata",
			repositoryRoot: projectRoot,
			projectRoot:    projectRoot,
			short:          true,
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			if !tst.short && testing.Short() {
				t.SkipNow()
				return
			}
			// Perform LSIF indexing.
			var buf bytes.Buffer
			indexer := New(
				tst.repositoryRoot,
				"github.com/sourcegraph/lsif-go",
				tst.projectRoot,
				protocol.ToolInfo{Name: "lsif-go", Version: "dev"},
				"testdata",
				"0.0.1",
				dependencies,
				projectDependencies,
				writer.NewJSONWriter(&buf),
				NewPackageDataCache(),
				output.Options{},
				NewGenerationOptions(),
			)
			if err := indexer.Index(); err != nil {
				t.Fatalf("unexpected error indexing testdata: %s", err.Error())
			}

			// Convert documentation to Markdown format.
			files, err := staticdoc.Generate(context.Background(), &buf, tst.projectRoot, staticdoc.TestingOptions)
			if err != nil {
				t.Fatal("failed to generate static doc:", err)
			}
			dir := filepath.Join("testdata", t.Name())
			_ = os.RemoveAll(dir)
			for filePath, fileContents := range files.ByPath {
				filePath = filepath.Join(dir, filePath)
				_ = os.MkdirAll(filepath.Dir(filePath), 0700)
				err := ioutil.WriteFile(filePath, fileContents, 0700)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestIndexer_shouldVisitPackage(t *testing.T) {
	w := &capturingWriter{}
	projectRoot := path.Join(getRepositoryRoot(t), "fixtures")
	indexer := New(
		"/dev/github.com/sourcegraph/lsif-go/internal/testdata/fixtures",
		"github.com/sourcegraph/lsif-go",
		projectRoot,
		protocol.ToolInfo{Name: "lsif-go", Version: "dev"},
		"testdata",
		"0.0.1",
		dependencies,
		projectDependencies,
		w,
		NewPackageDataCache(),
		output.Options{},
		NewGenerationOptions(),
	)

	if err := indexer.loadPackages(false); err != nil {
		t.Fatal(err)
	}

	visited := map[string]bool{}
	for _, pkg := range indexer.packages {
		shortID := strings.Replace(pkg.ID, "github.com/sourcegraph/lsif-go/internal/testdata/fixtures/internal", "…", -1)
		if indexer.shouldVisitPackage(pkg, indexer.packages) {
			visited[shortID] = true
		} else {
			visited[shortID] = false
		}
	}
	autogold.Want("visited", map[string]bool{
		"github.com/sourcegraph/lsif-go/internal/testdata/fixtures":                                                                                                                    true,
		"github.com/sourcegraph/lsif-go/internal/testdata/fixtures/conflicting_test_symbols":                                                                                           false,
		"github.com/sourcegraph/lsif-go/internal/testdata/fixtures/conflicting_test_symbols [github.com/sourcegraph/lsif-go/internal/testdata/fixtures/conflicting_test_symbols.test]": true,
		"github.com/sourcegraph/lsif-go/internal/testdata/fixtures/conflicting_test_symbols.test":                                                                                      false,
		"github.com/sourcegraph/lsif-go/internal/testdata/fixtures/duplicate_path_id":                                                                                                  true,
		"github.com/sourcegraph/lsif-go/internal/testdata/fixtures/illegal_multiple_mains":                                                                                             true,
		"github.com/sourcegraph/lsif-go/internal/testdata/fixtures/cmd/minimal_main":                                                                                                   true,
		"github.com/sourcegraph/lsif-go/internal/testdata/fixtures/pkg":                                                                                                                true,
		"…/secret":              true,
		"…/shouldvisit/notests": true,
		"…/shouldvisit/tests":   false,
		"…/shouldvisit/tests […/shouldvisit/tests.test]":                        true,
		"…/shouldvisit/tests.test":                                              false,
		"…/shouldvisit/tests_separate":                                          true,
		"…/shouldvisit/tests_separate.test":                                     false,
		"…/shouldvisit/tests_separate_test […/shouldvisit/tests_separate.test]": true,
	}).Equal(t, visited)
}

func TestIndexer_findBestPackageDefinitionPath(t *testing.T) {
	t.Run("Should find exact name match", func(t *testing.T) {
		packageName := "smol"
		possibleFilepaths := []DeclInfo{
			{false, "smol.go"},
			{false, "other.go"},
		}

		pkgDefinitionPath, _ := findBestPackageDefinitionPath(packageName, possibleFilepaths)
		if pkgDefinitionPath != "smol.go" {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", "smol.go", pkgDefinitionPath)
		}
	})

	t.Run("Should not pick _test files if package is not a test package", func(t *testing.T) {
		packageName := "mylib"
		possibleFilepaths := []DeclInfo{
			{false, "smol.go"},
			{false, "smol_test.go"},
		}

		pkgDefinitionPath, _ := findBestPackageDefinitionPath(packageName, possibleFilepaths)
		if pkgDefinitionPath != "smol.go" {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", "smol.go", pkgDefinitionPath)
		}
	})

	t.Run("should always pick whatever has the documentation", func(t *testing.T) {
		packageName := "mylib"
		possibleFilepaths := []DeclInfo{
			{true, "smol.go"},
			{false, "mylib.go"},
		}

		pkgDefinitionPath, _ := findBestPackageDefinitionPath(packageName, possibleFilepaths)
		if pkgDefinitionPath != "smol.go" {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", "smol.go", pkgDefinitionPath)
		}
	})

	t.Run("should pick a name that is a closer edit distance than one far away", func(t *testing.T) {
		packageName := "http_router"
		possibleFilepaths := []DeclInfo{
			{false, "httprouter.go"},
			{false, "httpother.go"},
		}

		pkgDefinitionPath, _ := findBestPackageDefinitionPath(packageName, possibleFilepaths)
		if pkgDefinitionPath != "httprouter.go" {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", "smol.go", pkgDefinitionPath)
		}
	})

	t.Run("should prefer test packages over other packages if the package name has test suffix", func(t *testing.T) {
		packageName := "httprouter_test"
		possibleFilepaths := []DeclInfo{
			{false, "httprouter.go"},
			{false, "http_test.go"},
		}

		pkgDefinitionPath, _ := findBestPackageDefinitionPath(packageName, possibleFilepaths)
		if pkgDefinitionPath != "http_test.go" {
			t.Errorf("incorrect hover text documentation. want=%q have=%q", "smol.go", pkgDefinitionPath)
		}
	})
}
