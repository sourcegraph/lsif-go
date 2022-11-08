package index

import (
	"go/ast"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

type Document struct {
	*scip.Document
}

const SymbolDefinition = int32(scip.SymbolRole_Definition)
const SymbolReference = int32(scip.SymbolRole_ReadAccess)

func (d *Document) appendSymbolDefinition(
	symbol string,
	rng []int32,
	decl ast.Decl,
	node ast.Node,
) {
	d.Occurrences = append(d.Occurrences, &scip.Occurrence{
		Range:       rng,
		Symbol:      symbol,
		SymbolRoles: SymbolDefinition,
	})

	documentation := []string{}
	if node != nil {
		hover := extractHoverText(decl, node)
		if hover != "" {
			documentation = append(documentation, hover)
		}
	}

	d.Symbols = append(d.Symbols, &scip.SymbolInformation{
		Symbol:        symbol,
		Documentation: documentation,
	})
}

func (d *Document) appendSymbolReference(symbol string, rng []int32) {
	d.Occurrences = append(d.Occurrences, &scip.Occurrence{
		Range:       rng,
		Symbol:      symbol,
		SymbolRoles: SymbolReference,
	})
}

func extractHoverText(decl ast.Decl, node ast.Node) string {
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
		if doc == "" && decl != nil {
			doc = extractHoverText(nil, decl)
		}

		return doc
	case *ast.ValueSpec:
		return v.Doc.Text()
	case *ast.Field:
		return strings.TrimSpace(v.Doc.Text() + "\n" + v.Comment.Text())
	}

	return ""
}
