package indexer

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hexops/autogold"
	"github.com/sourcegraph/lsif-go/internal/output"
	"github.com/sourcegraph/lsif-static-doc/staticdoc"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer"
)

func TestIndexer(t *testing.T) {
	w := &capturingWriter{}
	projectRoot := getRepositoryRoot(t)
	indexer := New(
		"/dev/github.com/sourcegraph/lsif-go/internal/testdata",
		"github.com/sourcegraph/lsif-go",
		projectRoot,
		protocol.ToolInfo{Name: "lsif-go", Version: "dev"},
		"testdata",
		"0.0.1",
		nil,
		w,
		NewPackageDataCache(),
		output.Options{},
	)

	if err := indexer.Index(); err != nil {
		t.Fatalf("unexpected error indexing testdata: %s", err.Error())
	}

	t.Run("check Parallel function hover text", func(t *testing.T) {
		r, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "parallel.go"), 13, 5)
		if !ok {
			t.Fatalf("could not find target range")
		}

		hoverResult, ok := findHoverResultByRangeOrResultSetID(w.elements, r.ID)
		markupContentSegments := splitMarkupContent(hoverResult.Result.Contents.(protocol.MarkupContent).Value)

		if !ok || len(markupContentSegments) < 2 {
			t.Fatalf("could not find hover text")
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

	// TODO(efritz) - support "package testdata" identifiers

	t.Run("check external package hover text", func(t *testing.T) {
		r, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "parallel.go"), 4, 2)
		if !ok {
			t.Fatalf("could not find target range")
		}

		hoverResult, ok := findHoverResultByRangeOrResultSetID(w.elements, r.ID)
		markupContentSegments := splitMarkupContent(hoverResult.Result.Contents.(protocol.MarkupContent).Value)
		if !ok || len(markupContentSegments) < 2 {
			t.Fatalf("could not find hover text")
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
		r, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "parallel.go"), 21, 3)
		if !ok {
			t.Fatalf("could not find target range")
		}

		definitions := findDefinitionRangesByRangeOrResultSetID(w.elements, r.ID)
		if len(definitions) != 1 {
			t.Fatalf("incorrect definition count. want=%d have=%d", 1, len(definitions))
		}

		compareRange(t, definitions[0], 15, 1, 15, 5)
	})

	t.Run("check wg references", func(t *testing.T) {
		r, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "parallel.go"), 26, 1)
		if !ok {
			t.Fatalf("could not find target range")
		}

		references := findReferenceRangesByRangeOrResultSetID(w.elements, r.ID)
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
		r, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "data.go"), 27, 3)
		if !ok {
			t.Fatalf("could not find target range")
		}

		monikers := findMonikersByRangeOrReferenceResultID(w.elements, r.ID)
		if len(monikers) != 1 {
			t.Fatalf("incorrect moniker count. want=%d have=%d", 1, len(monikers))
		}

		if value := monikers[0].Scheme; value != "gomod" {
			t.Errorf("incorrect scheme. want=%q have=%q", "gomod", value)
		}

		expectedIdentifier := "github.com/sourcegraph/lsif-go/internal/testdata:TestStruct.FieldWithAnonymousType.NestedB"
		if value := monikers[0].Identifier; value != expectedIdentifier {
			t.Errorf("incorrect identifier. want=%q have=%q", expectedIdentifier, value)
		}
	})

	t.Run("check typeswitch", func(t *testing.T) {
		definition, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "typeswitch.go"), 3, 8)
		if !ok {
			t.Fatalf("could not find target range")
		}

		intReference, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "typeswitch.go"), 5, 9)
		if !ok {
			t.Fatalf("could not find target range")
		}

		boolReference, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "typeswitch.go"), 7, 10)
		if !ok {
			t.Fatalf("could not find target range")
		}

		//
		// Check definition links

		definitions := findDefinitionRangesByRangeOrResultSetID(w.elements, intReference.ID)
		if len(definitions) != 1 {
			t.Fatalf("incorrect definition count. want=%d have=%d", 1, len(definitions))
		}
		compareRange(t, definitions[0], 3, 8, 3, 21)

		//
		// Check reference links

		references := findReferenceRangesByRangeOrResultSetID(w.elements, definition.ID)
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

		intReferenceHoverResult, ok := findHoverResultByRangeOrResultSetID(w.elements, intReference.ID)
		markupContentSegments := splitMarkupContent(intReferenceHoverResult.Result.Contents.(protocol.MarkupContent).Value)
		if !ok || len(markupContentSegments) < 1 {
			t.Fatalf("could not find hover text")
		}

		expectedType := `var concreteValue int`
		if value := unCodeFence(markupContentSegments[0]); value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}

		boolReferenceHoverResult, ok := findHoverResultByRangeOrResultSetID(w.elements, boolReference.ID)
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

		r, ok := findRange(w.elements, typealiasFile, 7, 5)
		if !ok {
			t.Fatalf("could not find target range")
		}

		definitions := findDefinitionRangesByRangeOrResultSetID(w.elements, r.ID)
		if len(definitions) != 1 {
			t.Fatalf("incorrection definition count. want=%d have=%d", 1, len(definitions))
		}

		compareRange(t, definitions[0], 7, 5, 7, 17)

		hover, ok := findHoverResultByRangeOrResultSetID(w.elements, r.ID)
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

		r, ok := findRange(w.elements, typealiasFile, 7, 27)
		if !ok {
			t.Fatalf("could not find target range")
		}

		definitions := findDefinitionRangesByRangeOrResultSetID(w.elements, r.ID)
		if len(definitions) != 1 {
			t.Fatalf("incorrection definition count. want=%d have=%d", 1, len(definitions))
		}

		p, _ := findDefinitionByName(t, indexer.packages, "Burger")
		if p.Name != "secret" {
			t.Fatalf("incorrect definition source package. want=%s have=%s", "secret", p.Name)
		}

		compareRange(t, definitions[0], 6, 5, 6, 11)

		hover, ok := findHoverResultByRangeOrResultSetID(w.elements, r.ID)
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

		r, ok := findRange(w.elements, typealiasFile, 9, 5)
		if !ok {
			t.Fatalf("could not find target range")
		}

		definitions := findDefinitionRangesByRangeOrResultSetID(w.elements, r.ID)
		if len(definitions) != 1 {
			t.Fatalf("incorrection definition count. want=%d have=%d", 1, len(definitions))
		}

		compareRange(t, definitions[0], 9, 5, 9, 14)

		hover, ok := findHoverResultByRangeOrResultSetID(w.elements, r.ID)
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

		r, ok = findRange(w.elements, typealiasFile, 9, 17)
		if ok {
			t.Fatalf("found range for anonymous struct when not expected")
		}
	})

	t.Run("check nested struct definition", func(t *testing.T) {
		r, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "composite.go"), 11, 1)
		if !ok {
			t.Fatalf("could not find target range")
		}

		definitions := findDefinitionRangesByRangeOrResultSetID(w.elements, r.ID)
		if len(definitions) != 2 {
			t.Fatalf("incorrect definition count. want=%d have=%d", 2, len(definitions))
		}

		compareRange(t, definitions[0], 4, 5, 4, 10)
		compareRange(t, definitions[1], 11, 1, 11, 6)
	})
}

func TestIndexer_documentation(t *testing.T) {
	projectRoot := getRepositoryRoot(t)
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
				nil,
				writer.NewJSONWriter(&buf),
				NewPackageDataCache(),
				output.Options{},
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

func compareRange(t *testing.T, r protocol.Range, startLine, startCharacter, endLine, endCharacter int) {
	if r.Start.Line != startLine || r.Start.Character != startCharacter || r.End.Line != endLine || r.End.Character != endCharacter {
		t.Errorf(
			"incorrect range. want=[%d:%d,%d:%d) have=[%d:%d,%d:%d)",
			startLine, startCharacter, endLine, endCharacter,
			r.Start.Line, r.Start.Character, r.End.Line, r.End.Character,
		)
	}
}

func TestIndexer_shouldVisitPackage(t *testing.T) {
	w := &capturingWriter{}
	projectRoot := getRepositoryRoot(t)
	indexer := New(
		"/dev/github.com/sourcegraph/lsif-go/internal/testdata",
		"github.com/sourcegraph/lsif-go",
		projectRoot,
		protocol.ToolInfo{Name: "lsif-go", Version: "dev"},
		"testdata",
		"0.0.1",
		nil,
		w,
		NewPackageDataCache(),
		output.Options{},
	)

	if err := indexer.loadPackages(false); err != nil {
		t.Fatal(err)
	}

	visited := map[string]bool{}
	for _, pkg := range indexer.packages {
		shortID := strings.Replace(pkg.ID, "github.com/sourcegraph/lsif-go/internal/testdata/internal", "…", -1)
		if indexer.shouldVisitPackage(pkg, indexer.packages) {
			visited[shortID] = true
		} else {
			visited[shortID] = false
		}
	}
	autogold.Want("visited", map[string]bool{
		"github.com/sourcegraph/lsif-go/internal/testdata":                          true,
		"github.com/sourcegraph/lsif-go/internal/testdata/conflicting_test_symbols": false,
		"github.com/sourcegraph/lsif-go/internal/testdata/conflicting_test_symbols [github.com/sourcegraph/lsif-go/internal/testdata/conflicting_test_symbols.test]": true,
		"github.com/sourcegraph/lsif-go/internal/testdata/conflicting_test_symbols.test":                                                                             false,
		"github.com/sourcegraph/lsif-go/internal/testdata/duplicate_path_id":                                                                                         true,
		"github.com/sourcegraph/lsif-go/internal/testdata/illegal_multiple_mains":                                                                                    true,
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
