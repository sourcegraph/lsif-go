package indexer

import (
	"path/filepath"
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

	t.Run("check type alias", func(t *testing.T) {
		/* definition, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "typealias.go"), 7, 5)
		if !ok {
			t.Errorf("could not find target range")
		}

		definitions := findDefinitionRangesByRangeOrResultSetID(w.elements, definition.ID)
		if len(definitions) != 1 {
			t.Errorf("incorrect definition count. want=%d have=%d", 1, len(definitions))
		} */

		r, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "typealias.go"), 11, 22)
		if !ok {
			t.Fatalf("could not find target range")
		}

		hoverResult, ok := findHoverResultByRangeOrResultSetID(w.elements, r.ID)
		if !ok || len(hoverResult.Result.Contents) < 2 {
			t.Fatalf("could not find hover text")
		}

		t.Logf("hover %#v", hoverResult)

		/* definitionOG, ok := findRange(w.elements, "file://"+filepath.Join(projectRoot, "typealias.go"), 11, 22)
		if !ok {
			t.Errorf("could not find target range")
		}

		definitionsOG := findDefinitionRangesByRangeOrResultSetID(w.elements, definitionOG.ID)
		if len(definitionsOG) != 1 {
			t.Errorf("incorrect definition count. want=%d have=%d", 1, len(definitionsOG))
		}

		t.Logf("range %v", defRange)
		t.Logf("all defs %v", definitionsOG)
		compareRange(t, definitionsOG[0], 6, 5, 6, 11) */
		/* hover, ok := findHoverResultByRangeOrResultSetID(w.elements, definitionOG.ID)
		if !ok {
			t.Errorf("could not find hover text")
		}

		expectedHover := `Burger is food \n\n`
		if value := hover.Result.Contents[1].Value; value != expectedHover {
			t.Errorf("incorrect hover text. want=%q have=%q", expectedHover, value)
		} */
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
