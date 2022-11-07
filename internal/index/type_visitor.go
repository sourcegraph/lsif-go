package index

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

type TypeVisitor struct {
	doc *scip.Document
	pkg *packages.Package
	vis ast.Visitor

	fields map[token.Pos]string
}

func (v TypeVisitor) Visit(n ast.Node) (w ast.Visitor) {
	switch node := n.(type) {
	case *ast.GenDecl:
		switch node.Tok {
		case token.TYPE:
			return v
		default:
			return nil
		}
	case *ast.TypeSpec:
		pkgDescriptor := &scip.Descriptor{
			Name:   v.pkg.PkgPath,
			Suffix: scip.Descriptor_Namespace,
		}

		structDescriptors := []*scip.Descriptor{
			pkgDescriptor,
			{
				Name:   node.Name.Name,
				Suffix: scip.Descriptor_Type,
			},
		}

		symbol := scipSymbolFromDescriptors(v.pkg.Module, structDescriptors)
		position := v.pkg.Fset.Position(node.Name.NamePos)
		v.doc.Occurrences = append(v.doc.Occurrences, &scip.Occurrence{
			Range:       scipRangeFromName(position, node.Name.Name, false),
			Symbol:      symbol,
			SymbolRoles: SymbolDefinition,
		})

		if node.TypeParams != nil {
			panic("generics")
		}

		ast.Walk(v, node.Type)

		return nil

	case *ast.StructType, *ast.InterfaceType:
		return v

	case *ast.FieldList:
		for _, field := range node.List {
			for _, name := range field.Names {
				if fieldSymbol, ok := v.fields[name.NamePos]; ok {
					namePosition := v.pkg.Fset.Position(name.NamePos)
					v.doc.Occurrences = append(v.doc.Occurrences, &scip.Occurrence{
						Range:       scipRangeFromName(namePosition, name.Name, false),
						Symbol:      fieldSymbol,
						SymbolRoles: SymbolDefinition,
					})
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
