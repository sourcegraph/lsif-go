package index

import (
	"bytes"
	"go/ast"
	"strings"

	doc "github.com/slimsag/godocmd"

	"github.com/sourcegraph/lsif-go/internal/indexer"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"golang.org/x/tools/go/packages"
)

type Document struct {
	*scip.Document

	// The package this document is contained in
	pkg *packages.Package

	// pkgSymbols maps positions to symbol names within
	// this document.
	pkgSymbols *PackageSymbols
}

const SymbolDefinition = int32(scip.SymbolRole_Definition)
const SymbolReference = int32(scip.SymbolRole_ReadAccess)

func (d *Document) declareNewSymbol(
	symbol string,
	parent ast.Node,
	ident *ast.Ident,
) {
	documentation := []string{}
	if ident != nil {
		hover := d.extractHoverText(parent, ident)
		signature, extra := indexer.TypeStringForObject(d.pkg.TypesInfo.Defs[ident])

		if signature != "" {
			documentation = append(documentation, formatCode(signature))
		}
		if hover != "" {
			documentation = append(documentation, formatMarkdown(hover))
		}
		if extra != "" {
			documentation = append(documentation, formatCode(extra))
		}
	}

	d.Symbols = append(d.Symbols, &scip.SymbolInformation{
		Symbol:        symbol,
		Documentation: documentation,
	})

	d.pkgSymbols.set(ident.Pos(), symbol)
}

func (d *Document) NewOccurrence(symbol string, rng []int32) {
	d.Occurrences = append(d.Occurrences, &scip.Occurrence{
		Range:       rng,
		Symbol:      symbol,
		SymbolRoles: SymbolDefinition,
	})
}

func (d *Document) appendSymbolReference(symbol string, rng []int32) {
	d.Occurrences = append(d.Occurrences, &scip.Occurrence{
		Range:       rng,
		Symbol:      symbol,
		SymbolRoles: SymbolReference,
	})
}

func (d *Document) extractHoverText(parent ast.Node, node ast.Node) string {
	switch v := node.(type) {
	case *ast.FuncDecl:
		return v.Doc.Text()
	case *ast.GenDecl:
		return v.Doc.Text()
	case *ast.TypeSpec:
		// Typespecs do not have the doc associated with them much
		// of the time. They are often associated with the `type`
		// token itself.
		//
		// This is why we have to pass the declaration node
		doc := v.Doc.Text()
		if doc == "" && parent != nil {
			doc = d.extractHoverText(nil, parent)
		}

		return doc
	case *ast.ValueSpec:
		doc := v.Doc.Text()
		if doc == "" && parent != nil {
			doc = d.extractHoverText(nil, parent)
		}

		return doc
	case *ast.Field:
		return strings.TrimSpace(v.Doc.Text() + "\n" + v.Comment.Text())
	case *ast.Ident:
		if parent != nil {
			return d.extractHoverText(nil, parent)
		}
	}

	return ""
}

func formatCode(v string) string {
	if v == "" {
		return ""
	}

	// reuse MarkedString here as it takes care of code fencing
	return protocol.NewMarkedString(v, "go").String()
}

func formatMarkdown(v string) string {
	if v == "" {
		return ""
	}

	var buf bytes.Buffer
	doc.ToMarkdown(&buf, v, nil)
	return buf.String()
}
