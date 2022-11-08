package index

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

type TypeVisitor struct {
	doc *Document
	pkg *packages.Package
	vis ast.Visitor

	fields *PackageFields

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
		typeRange := scipRangeFromName(v.pkg.Fset.Position(node.Name.NamePos), node.Name.Name, false)
		v.doc.appendSymbolDefinition(typeSymbol, typeRange, v.curDecl, node)

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
				if fieldSymbol, ok := v.fields.get(name.NamePos); ok {
					namePosition := v.pkg.Fset.Position(name.NamePos)
					nameRange := scipRangeFromName(namePosition, name.Name, false)
					v.doc.appendSymbolDefinition(fieldSymbol, nameRange, nil, field)
				} else {
					panic(fmt.Sprintf("field with no definition: %v", node))
				}
			}

			switch field.Type.(type) {
			case *ast.InterfaceType:
				ast.Walk(v, field.Type)
			default:
				ast.Walk(v.vis, field.Type)
			}
		}
		return nil

	case *ast.FuncType, *ast.SelectorExpr:
		return v.vis

	case *ast.Ident:
		ast.Walk(v.vis, node)
		return nil

	default:
		return v.vis
	}
}
