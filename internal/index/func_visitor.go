package index

import (
	"go/ast"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

type FuncVisitor struct {
	doc *Document
	pkg *packages.Package
	vis ast.Visitor

	name *ast.Ident
}

// Implements ast.Visitor
var _ ast.Visitor = &FuncVisitor{}

func (v *FuncVisitor) Visit(node ast.Node) (w ast.Visitor) {
	switch node := node.(type) {
	case *ast.FuncDecl:
		v.name = node.Name

		pos := node.Name.Pos()
		position := v.pkg.Fset.Position(pos)

		desciptors := []*scip.Descriptor{
			{
				Name:   v.pkg.PkgPath,
				Suffix: scip.Descriptor_Namespace,
			},
		}
		if recv, has := receiverTypeName(node); has {
			desciptors = append(desciptors, descriptorType(recv))
		}
		desciptors = append(desciptors, descriptorMethod(node.Name.Name))
		symbol := scipSymbolFromDescriptors(v.pkg.Module, desciptors)

		v.doc.appendSymbolDefinition(
			symbol,
			scipRangeFromName(position, node.Name.Name, false),
			nil,
			node,
		)

		return v
	case *ast.Ident:
		// We've already emitted the definition of the name,
		// so do not emit any more information
		if node.Pos() == v.name.Pos() {
			return nil
		}

		return v.vis

	default:
		return v.vis
	}
}
