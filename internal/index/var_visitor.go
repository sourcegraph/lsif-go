package index

import (
	"go/ast"
	"go/token"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

func visitVarDefinition(doc *Document, pkg *packages.Package, decl *ast.GenDecl) {
	ast.Walk(VarVisitor{
		doc: doc,
		pkg: pkg,
	}, decl)
}

type VarVisitor struct {
	doc *Document
	pkg *packages.Package

	curDecl ast.Decl
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
			v.curDecl = node
			return v
		default:
			return nil
		}
	case *ast.ValueSpec:
		// Iterate over names, which are the only thing that can be definitions
		for _, name := range node.Names {
			symbol := scipSymbolFromDescriptors(v.pkg.Module, []*scip.Descriptor{
				descriptorTerm(name.Name),
			})

			v.doc.declareNewSymbol(symbol, v.curDecl, name)

			// position := v.pkg.Fset.Position(name.Pos())
			// v.doc.NewOccurrence(symbol, scipRangeFromName(position, name.Name, false))
		}

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
