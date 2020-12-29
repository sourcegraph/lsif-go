package indexer

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	doc "github.com/slimsag/godocmd"
	protocol "github.com/sourcegraph/lsif-protocol"
)

const languageGo = "go"

// rangeForObject transforms the position of the given object (1-indexed) into an LSP range
// (0-indexed). If the object is a quoted package name, the leading and trailing quotes are
// stripped from the resulting range's bounds.
func rangeForObject(obj types.Object, pos token.Position) (protocol.Pos, protocol.Pos) {
	adjustment := 0
	if pkgName, ok := obj.(*types.PkgName); ok && strings.HasPrefix(pkgName.Name(), `"`) {
		adjustment = 1
	}

	line := pos.Line - 1
	column := pos.Column - 1
	n := len(obj.Name())

	start := protocol.Pos{Line: line, Character: column + adjustment}
	end := protocol.Pos{Line: line, Character: column + n - adjustment}
	return start, end
}

func rangeForNode(fset *token.FileSet, node ast.Node) protocol.RangeData {
	start := fset.Position(node.Pos())
	end := fset.Position(node.End())
	return protocol.RangeData{
		Start: protocol.Pos{
			Line:      start.Line - 1,
			Character: start.Column - 1,
		},
		End: protocol.Pos{
			Line:      end.Line - 1,
			Character: end.Column - 1,
		},
	}
}

// toMarkedString creates a protocol.MarkedString object from the given content. The signature
// and extra parameters are formatted as code, if supplied. The docstring is formatted as markdown,
// if supplied.
func toMarkedString(signature, docstring, extra string) (mss []protocol.MarkedString) {
	for _, m := range []*protocol.MarkedString{formatCode(signature), formatMarkdown(docstring), formatCode(extra)} {
		if m != nil {
			mss = append(mss, *m)
		}
	}

	return mss
}

// formatMarkdown creates a protocol.MarkedString object containing a markdown-formatted version
// of the given string. If the given string is empty, nil is returned.
func formatMarkdown(v string) *protocol.MarkedString {
	if v == "" {
		return nil
	}

	var buf bytes.Buffer
	doc.ToMarkdown(&buf, v, nil)
	ms := protocol.RawMarkedString(buf.String())
	return &ms
}

// formatCode creates a protocol.MarkedString object containing a code fence-formatted version
// of the given string. If the given string is empty, nil is returned.
func formatCode(v string) *protocol.MarkedString {
	if v == "" {
		return nil
	}

	ms := protocol.NewMarkedString(v, languageGo)
	return &ms
}
