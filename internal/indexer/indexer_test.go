package indexer

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/sourcegraph/lsif-go/protocol"
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
		false,
	)

	if _, err := indexer.Index(); err != nil {
		t.Fatalf("unexpected error indexing testdata: %s", err.Error())
	}

	t.Run("check Parallel function hover text", func(t *testing.T) {
		r := findRange(w.elements, "file://"+filepath.Join(projectRoot, "parallel.go"), 13, 5)
		if r == nil {
			t.Fatalf("could not find target range")
		}

		hoverResult := findHoverResultByRangeOrResultSetID(w.elements, r.ID)
		if hoverResult == nil || len(hoverResult.Result.Contents) < 2 {
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
		r := findRange(w.elements, "file://"+filepath.Join(projectRoot, "parallel.go"), 4, 2)
		if r == nil {
			t.Fatalf("could not find target range")
		}

		hoverResult := findHoverResultByRangeOrResultSetID(w.elements, r.ID)
		if hoverResult == nil || len(hoverResult.Result.Contents) < 2 {
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
		r := findRange(w.elements, "file://"+filepath.Join(projectRoot, "parallel.go"), 23, 3)
		if r == nil {
			t.Fatalf("could not find target range")
		}

		definitions := findDefinitionRangesByRangeOrResultSetID(w.elements, r.ID)
		if len(definitions) != 1 {
			t.Errorf("incorrect definition count. want=%d have=%d", 1, len(definitions))
		}

		compareRange(t, definitions[0], 15, 1, 15, 5)
	})

	t.Run("check wg references", func(t *testing.T) {
		r := findRange(w.elements, "file://"+filepath.Join(projectRoot, "parallel.go"), 27, 1)
		if r == nil {
			t.Fatalf("could not find target range")
		}

		references := findReferenceRangesByRangeOrResultSetID(w.elements, r.ID)
		if len(references) != 4 {
			t.Errorf("incorrect reference count. want=%d have=%d", 4, len(references))
		}

		sort.Slice(references, func(i, j int) bool { return references[i].Start.Line < references[j].Start.Line })

		compareRange(t, references[0], 14, 5, 14, 7)  // var wg sync.WaitGroup
		compareRange(t, references[1], 18, 2, 18, 4)  // wg.Add(1)
		compareRange(t, references[2], 21, 9, 21, 11) // defer wg.Done()
		compareRange(t, references[3], 27, 1, 27, 3)  // wg.Wait()
	})

	t.Run("check NestedB monikers", func(t *testing.T) {
		r := findRange(w.elements, "file://"+filepath.Join(projectRoot, "data.go"), 26, 2)
		if r == nil {
			t.Fatalf("could not find target range")
		}

		monikers := findMonikersByRangeOrReferenceResultID(w.elements, r.ID)
		if len(monikers) != 1 {
			t.Errorf("incorrect moniker count. want=%d have=%d", 1, len(monikers))
		}

		if value := monikers[0].Scheme; value != "gomod" {
			t.Errorf("incorrect scheme. want=%q have=%q", "gomod", value)
		}

		expectedIdentifier := "github.com/sourcegraph/lsif-go/internal/testdata:TestStruct.FieldWithAnonymousType.NestedB"
		if value := monikers[0].Identifier; value != expectedIdentifier {
			t.Errorf("incorrect identifier. want=%q have=%q", expectedIdentifier, value)
		}
	})
}

func compareRange(t *testing.T, r *protocol.Range, startLine, startCharacter, endLine, endCharacter int) {
	if r.Start.Line != startLine || r.Start.Character != startCharacter || r.End.Line != endLine || r.End.Character != endCharacter {
		t.Errorf(
			"incorrect range. want=[%d:%d,%d:%d) have=[%d:%d,%d:%d)",
			startLine, startCharacter, endLine, endCharacter,
			r.Start.Line, r.Start.Character, r.End.Line, r.End.Character,
		)
	}
}
