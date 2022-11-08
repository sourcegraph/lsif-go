package index

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

func visitTypeDefinition(doc *Document, pkg *packages.Package, decl *ast.GenDecl) {
	ast.Walk(TypeVisitor{
		doc: doc,
		pkg: pkg,
	}, decl)
}

type TypeVisitor struct {
	doc     *Document
	pkg     *packages.Package
	curDecl *ast.GenDecl
}

func (v TypeVisitor) Visit(n ast.Node) (w ast.Visitor) {
	switch node := n.(type) {
	case *ast.GenDecl:
		switch node.Tok {
		case token.TYPE:
			v.curDecl = node
			return v
		default:
			return nil
		}
	case *ast.TypeSpec:
		structDescriptors := []*scip.Descriptor{
			{
				Name:   v.pkg.PkgPath,
				Suffix: scip.Descriptor_Namespace,
			},
			{
				Name:   node.Name.Name,
				Suffix: scip.Descriptor_Type,
			},
		}

		typeSymbol := scipSymbolFromDescriptors(v.pkg.Module, structDescriptors)
		v.doc.declareNewSymbol(typeSymbol, v.curDecl, node.Name)

		if node.TypeParams != nil {
			// panic("generics")
		}

		ast.Walk(v, node.Type)
		return nil

	case *ast.StructType, *ast.InterfaceType:
		return v

	case *ast.FieldList:
		for _, field := range node.List {
			for _, name := range field.Names {
				if fieldSymbol, ok := v.doc.pkgSymbols.get(name.NamePos); ok {
					v.doc.declareNewSymbol(fieldSymbol, field, name)
				} else {
					panic(fmt.Sprintf("field with no definition: %v", node))
				}
			}

			switch field.Type.(type) {
			case *ast.InterfaceType, *ast.StructType:
				ast.Walk(v, field.Type)
			}
		}
		return nil

	case *ast.FuncType, *ast.SelectorExpr:
		return nil

	case *ast.Ident:
		return nil

	default:
		return nil
	}
}
