package index

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

// StructVisitor collects the all the information for top-level structs
// that can be imported by any other file (they do not have to be exported).
//
// For example, a struct `myStruct` can be imported by other files in the same
// packages. So we need to make those field names global (we only have global
// or file-local).
type StructVisitor struct {
	// mapping from field definitions to symbols
	Fields map[token.Pos]string

	mod      *packages.Module
	curScope []*scip.Descriptor
}

// Implements ast.Visitor
var _ ast.Visitor = &StructVisitor{}

func (s *StructVisitor) getNameOfTypeExpr(ty ast.Expr) string {
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

func (s *StructVisitor) makeSymbol(descriptor *scip.Descriptor) string {
	return scipSymbolFromDescriptors(s.mod, append(s.curScope, descriptor))
}

func (s StructVisitor) Visit(n ast.Node) (w ast.Visitor) {
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
		return nil

	case *ast.Field:
		if len(node.Names) == 0 {
			s.Fields[node.Type.Pos()] = s.makeSymbol(&scip.Descriptor{
				Name:   s.getNameOfTypeExpr(node.Type),
				Suffix: scip.Descriptor_Term,
			})
		} else {
			for _, name := range node.Names {
				s.Fields[name.Pos()] = s.makeSymbol(&scip.Descriptor{
					Name:   name.Name,
					Suffix: scip.Descriptor_Term,
				})

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

		return nil

	default:
		return nil
	}
}
