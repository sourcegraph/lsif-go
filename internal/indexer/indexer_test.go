package indexer

import (
	"path/filepath"
	"sort"
	"testing"

	protocol "github.com/sourcegraph/lsif-protocol"
)

func TestIndexer(t *testing.T) {
	w := &capturingWriter{}
	projectRoot := getRepositoryRoot(t)
	indexer := New(
		"/dev/github.com/sourcegraph/lsif-go/internal/testdata",
		projectRoot,
		protocol.ToolInfo{Name: "lsif-go", Version: "dev"},
		"testdata",
		"0.0.1",
		nil,
		w,
		NewPackageDataCache(),
		OutputOptions{},
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
		if !ok || len(hoverResult.Result.Contents) < 2 {
			t.Fatalf("could not find hover text")
		}

		expectedType := `func Parallel(ctx Context, fns ...ParallelizableFunc) error`
		if value := hoverResult.Result.Contents[0].Value; value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}

		expectedDocumentation := normalizeDocstring(`
			Parallel invokes each of the given parallelizable functions in their own goroutines and
			returns the first error to occur. This method will block until all goroutines have returned.
		`)
		if value := normalizeDocstring(hoverResult.Result.Contents[1].Value); value != expectedDocumentation {
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
		if !ok || len(hoverResult.Result.Contents) < 2 {
			t.Fatalf("could not find hover text")
		}

		expectedType := `package "sync"`
		if value := hoverResult.Result.Contents[0].Value; value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}

		expectedDocumentation := normalizeDocstring(`
			Package sync provides basic synchronization primitives such as mutual exclusion locks.
			Other than the Once and WaitGroup types, most are intended for use by low-level library routines.
			Higher-level synchronization is better done via channels and communication.
			Values containing the types defined in this package should not be copied.
		`)
		if value := normalizeDocstring(hoverResult.Result.Contents[1].Value); value != expectedDocumentation {
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
		if !ok || len(intReferenceHoverResult.Result.Contents) < 1 {
			t.Fatalf("could not find hover text")
		}

		expectedType := `var concreteValue int`
		if value := intReferenceHoverResult.Result.Contents[0].Value; value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}

		boolReferenceHoverResult, ok := findHoverResultByRangeOrResultSetID(w.elements, boolReference.ID)
		if !ok || len(boolReferenceHoverResult.Result.Contents) < 1 {
			t.Fatalf("could not find hover text")
		}

		expectedType = `var concreteValue bool`
		if value := boolReferenceHoverResult.Result.Contents[0].Value; value != expectedType {
			t.Errorf("incorrect hover text type. want=%q have=%q", expectedType, value)
		}
	})
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
