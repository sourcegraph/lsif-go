package indexer

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"strings"

	doc "github.com/slimsag/godocmd"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/lsif/protocol"
)

const languageGo = "go"

var objKindToSymbolKind = map[ast.ObjKind]protocol.SymbolKind{
	ast.Pkg: protocol.Package,
	ast.Con: protocol.Constant,
	ast.Typ: protocol.Class, // Based on LLVM LSP implementation; no type alias in spec
	ast.Var: protocol.Variable,
	ast.Fun: protocol.Function,
	//ast.Lbl: protocol.:noidea: // No label in spec
}

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

func tagForObject(p *packages.Package, obj *ast.Object, rangeType string, commentMap ast.CommentMap, start, end protocol.Pos) *protocol.RangeTag {
	kind, ok := objKindToSymbolKind[obj.Kind]
	if !ok {
		return nil
	}

	deprecated := false

	// Since we're just looking at definitions right now, we assume that Decl
	// will be defined or that we can kind of just ignore comments.
	if obj.Decl != nil {
		commentGroups := commentMap[obj.Decl.(ast.Node)]
		for _, commentGroup := range commentGroups {
			lines := strings.Split(commentGroup.Text(), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Deprecated: ") {
					deprecated = true
					break
				}
			}

			// The tag range wants us to include comments, so we adjust if the
			// comments fall outside the current range.
			commentStart := p.Fset.Position(commentGroup.Pos())
			startLine := commentStart.Line - 1
			startColumn := commentStart.Column - 1
			if startLine < start.Line || (startLine == start.Line && startColumn < start.Character) {
				start.Line = startLine
				start.Character = startColumn
			}

			commentEnd := p.Fset.Position(commentGroup.End())
			endLine := commentEnd.Line - 1
			endColumn := commentEnd.Column - 1
			if endLine > end.Line || (endLine == end.Line && endColumn > end.Character) {
				end.Line = endLine
				end.Character = endColumn
			}
		}
	}

	fullRange := &protocol.RangeData{
		Start: start,
		End: end,
	}

	tag := &protocol.RangeTag{
		Type: rangeType,
		Text: obj.Name,
		Kind: kind,
		FullRange: fullRange,
	}

	if deprecated {
		tag.Tags = []protocol.SymbolTag{protocol.Deprecated}
	}

	return tag
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
