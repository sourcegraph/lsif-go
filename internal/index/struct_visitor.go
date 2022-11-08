package index

import (
	"fmt"
	"go/ast"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

func visitFieldsInFile(doc *Document, pkg *packages.Package, file *ast.File) {
	visitor := FieldVisitor{
		mod: pkg.Module,
		doc: doc,
		curScope: []*scip.Descriptor{
			{
				Name:   pkg.PkgPath,
				Suffix: scip.Descriptor_Namespace,
			},
		},
	}

	ast.Walk(visitor, file)
}

// FieldVisitor collects the all the information for top-level structs
// that can be imported by any other file (they do not have to be exported).
//
// For example, a struct `myStruct` can be imported by other files in the same
// packages. So we need to make those field names global (we only have global
// or file-local).
type FieldVisitor struct {
	doc      *Document
	mod      *packages.Module
	curScope []*scip.Descriptor
}

// Implements ast.Visitor
var _ ast.Visitor = &FieldVisitor{}

func (s *FieldVisitor) getNameOfTypeExpr(ty ast.Expr) string {
	switch ty := ty.(type) {
	case *ast.Ident:
		return ty.Name
	case *ast.SelectorExpr:
		return ty.Sel.Name
	case *ast.StarExpr:
		return s.getNameOfTypeExpr(ty.X)
	default:
		panic(fmt.Sprintf("Unhandled unamed struct field: %T %+v", ty, ty))
	}
}

func (s *FieldVisitor) makeSymbol(descriptor *scip.Descriptor) string {
	return scipSymbolFromDescriptors(s.mod, append(s.curScope, descriptor))
}

func (s FieldVisitor) Visit(n ast.Node) (w ast.Visitor) {
	if n == nil {
		return nil
	}

	switch node := n.(type) {
	case
		// Continue down file and decls
		*ast.File,
		*ast.GenDecl,

		// Toplevel types that are important
		*ast.StructType,
		*ast.InterfaceType,

		// Continue traversing subtypes
		*ast.FieldList,
		*ast.Ident:

		return s

	case *ast.TypeSpec:
		s.curScope = append(s.curScope, &scip.Descriptor{
			Name:   node.Name.Name,
			Suffix: scip.Descriptor_Type,
		})

		defer func() {
			s.curScope = s.curScope[:len(s.curScope)-1]
		}()

		ast.Walk(s, node.Type)
	case *ast.Field:
		if len(node.Names) == 0 {
			s.doc.declareNewSymbol(s.makeSymbol(&scip.Descriptor{
				Name:   s.getNameOfTypeExpr(node.Type),
				Suffix: scip.Descriptor_Term,
			}), nil, node)
		} else {
			for _, name := range node.Names {
				s.doc.declareNewSymbol(s.makeSymbol(&scip.Descriptor{
					Name:   name.Name,
					Suffix: scip.Descriptor_Term,
				}), nil, name)

				switch node.Type.(type) {
				case *ast.StructType, *ast.InterfaceType:
					// Current scope is now embedded in the anonymous struct
					//   So we walk the rest of the type expression and save
					//   the nested names
					s.curScope = append(s.curScope, &scip.Descriptor{
						Name:   name.Name,
						Suffix: scip.Descriptor_Term,
					})

					ast.Walk(s, node.Type)

					s.curScope = s.curScope[:len(s.curScope)-1]
				}
			}
		}
	}

	return nil
}
