package index

import (
	"go/ast"
	"go/token"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

type VarVisitor struct {
	doc *scip.Document
	pkg *packages.Package
	vis ast.Visitor
}

var _ ast.Visitor = &VarVisitor{}

func (v VarVisitor) Visit(n ast.Node) (w ast.Visitor) {
	if n == nil {
		return nil
	}

	switch node := n.(type) {
	case *ast.GenDecl:
		switch node.Tok {
		// Only traverse vars and consts
		case token.VAR, token.CONST:
			return v
		default:
			return nil
		}
	case *ast.ValueSpec:
		// Iterate over names, which are the only thing that can be definitions
		for _, name := range node.Names {
			position := v.pkg.Fset.Position(name.Pos())

			v.doc.Occurrences = append(v.doc.Occurrences, &scip.Occurrence{
				Range: scipRangeFromName(position, name.Name, false),
				Symbol: scipSymbolFromDescriptors(v.pkg.Module, []*scip.Descriptor{
					descriptorTerm(name.Name),
				}),
				SymbolRoles: SymbolDefinition,
			})
		}

		// Walk the rest of the struct
		noNames := *node
		noNames.Names = []*ast.Ident{}

		ast.Walk(v.vis, &noNames)

		// ast.Walk(s.vis, node.Doc)
		// ast.Walk(s.vis, node.Type)
		// walkExprList(s.vis, node.Values)
		// ast.Walk(s.vis, node.Comment)

		return nil
	default:
		return nil
	}
}

func walkExprList(v ast.Visitor, list []ast.Expr) {
	for _, x := range list {
		ast.Walk(v, x)
	}
}
