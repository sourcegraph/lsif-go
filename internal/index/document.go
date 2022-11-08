package index

import (
	"go/ast"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

type Document struct {
	*scip.Document

	// pkgSymbols maps positions to symbol names within
	// this document.
	pkgSymbols *PackageSymbols
}

const SymbolDefinition = int32(scip.SymbolRole_Definition)
const SymbolReference = int32(scip.SymbolRole_ReadAccess)

func (d *Document) declareNewSymbol(
	symbol string,
	parent ast.Node,
	node ast.Node,
) {
	documentation := []string{}
	if node != nil {
		hover := extractHoverText(parent, node)
		if hover != "" {
			documentation = append(documentation, hover)
		}
	}

	d.Symbols = append(d.Symbols, &scip.SymbolInformation{
		Symbol:        symbol,
		Documentation: documentation,
	})

	d.pkgSymbols.set(node.Pos(), symbol)
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

func extractHoverText(parent ast.Node, node ast.Node) string {
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
			doc = extractHoverText(nil, parent)
		}

		return doc
	case *ast.ValueSpec:
		doc := v.Doc.Text()
		if doc == "" && parent != nil {
			doc = extractHoverText(nil, parent)
		}

		return doc
	case *ast.Field:
		return strings.TrimSpace(v.Doc.Text() + "\n" + v.Comment.Text())
	case *ast.Ident:
		if parent != nil {
			return extractHoverText(nil, parent)
		}
	}

	return ""
}
