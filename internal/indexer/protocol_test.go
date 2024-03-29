package indexer

import (
	"encoding/json"
	"go/token"
	"go/types"
	"testing"

	"github.com/google/go-cmp/cmp"
	protocol "github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
)

func TestRangeForObject(t *testing.T) {
	start, end := rangeForObject(
		types.NewPkgName(token.Pos(42), nil, "foobar", nil),
		token.Position{Line: 10, Column: 25},
	)

	if diff := cmp.Diff(protocol.Pos{Line: 9, Character: 24}, start); diff != "" {
		t.Errorf("unexpected start (-want +got): %s", diff)
	}
	if diff := cmp.Diff(protocol.Pos{Line: 9, Character: 30}, end); diff != "" {
		t.Errorf("unexpected end (-want +got): %s", diff)
	}
}

func TestRangeForObjectWithQuotedNamed(t *testing.T) {
	start, end := rangeForObject(
		types.NewPkgName(token.Pos(42), nil, `"foobar"`, nil),
		token.Position{Line: 10, Column: 25},
	)

	if diff := cmp.Diff(protocol.Pos{Line: 9, Character: 25}, start); diff != "" {
		t.Errorf("unexpected start (-want +got): %s", diff)
	}
	if diff := cmp.Diff(protocol.Pos{Line: 9, Character: 31}, end); diff != "" {
		t.Errorf("unexpected end (-want +got): %s", diff)
	}
}

func TestToMarkedStringSignature(t *testing.T) {
	content, err := json.Marshal(toMarkupContent("var score int64", "", ""))
	if err != nil {
		t.Errorf("unexpected error marshalling hover content: %s", err)
	}

	if diff := cmp.Diff("{\"kind\":\"markdown\",\"value\":\"```go\\nvar score int64\\n```\"}", string(content)); diff != "" {
		t.Errorf("unexpected hover content (-want +got): %s", diff)
	}
}

func TestToMarkedStringDocstring(t *testing.T) {
	content, err := json.Marshal(toMarkupContent("var score int64", "Score tracks the user's score.", ""))
	if err != nil {
		t.Errorf("unexpected error marshalling hover content: %s", err)
	}

	if diff := cmp.Diff("{\"kind\":\"markdown\",\"value\":\"```go\\nvar score int64\\n```\\n\\n---\\n\\nScore tracks the user's score.\\n\\n\"}", string(content)); diff != "" {
		t.Errorf("unexpected hover content (-want +got): %s", diff)
	}
}

func TestToMarkedStringExtra(t *testing.T) {
	content, err := json.Marshal(toMarkupContent("var score int64", "", "score = 123"))
	if err != nil {
		t.Errorf("unexpected error marshalling hover content: %s", err)
	}

	if diff := cmp.Diff("{\"kind\":\"markdown\",\"value\":\"```go\\nvar score int64\\n```\\n\\n---\\n\\n```go\\nscore = 123\\n```\"}", string(content)); diff != "" {
		t.Errorf("unexpected hover content (-want +got): %s", diff)
	}
}
