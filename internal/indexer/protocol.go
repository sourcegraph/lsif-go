package indexer

import (
	"bytes"
	"go/token"
	"go/types"
	"strings"

	doc "github.com/slimsag/godocmd"
	protocol "github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
)

const languageGo = "go"

// rangeForObject transforms the position of the given object (1-indexed) into an LSP range
// (0-indexed). If the object is a quoted package name, the leading and trailing quotes are
// stripped from the resulting range's bounds.
func rangeForObject(obj NoahObject, pos token.Position) (protocol.Pos, protocol.Pos) {
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

// toMarkupContent creates a protocol.MarkupContent object from the given content. The signature
// and extra parameters are formatted as code, if supplied. The docstring is formatted as markdown,
// if supplied.
func toMarkupContent(signature, docstring, extra string) (mss protocol.MarkupContent) {
	var ss []string

	for _, m := range []string{formatCode(signature), formatMarkdown(docstring), formatCode(extra)} {
		if m != "" {
			ss = append(ss, m)
		}
	}

	return protocol.NewMarkupContent(strings.Join(ss, "\n\n---\n\n"), protocol.Markdown)
}

// formatMarkdown creates a string containing a markdown-formatted version
// of the given string.
func formatMarkdown(v string) string {
	if v == "" {
		return ""
	}

	var buf bytes.Buffer
	doc.ToMarkdown(&buf, v, nil)
	return buf.String()
}

// formatCode creates a string containing a code fence-formatted version
// of the given string.
func formatCode(v string) string {
	if v == "" {
		return ""
	}

	// reuse MarkedString here as it takes care of code fencing
	return protocol.NewMarkedString(v, languageGo).String()
}
