package index

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"golang.org/x/tools/go/packages"
)

// FileVisitor visits an entire file, but it must be called
// after StructVisitor.
//
// Iterates over a file,
type FileVisitor struct {
	// Document to append occurrences to
	doc *scip.Document

	// Current file information
	pkg  *packages.Package
	file *ast.File

	// soething
	pkgLookup map[string]*packages.Module

	// local definition position to symbol
	locals map[token.Pos]string

	// field definition position to symbol
	fields map[token.Pos]string
}

// Implements ast.Visitor
var _ ast.Visitor = &FileVisitor{}

func (f *FileVisitor) createNewLocalSymbol(pos token.Pos) string {
	if _, ok := f.locals[pos]; ok {
		panic("Cannot create a new local symbol for an ident that has already been created")
	}

	f.locals[pos] = fmt.Sprintf("local %d", len(f.locals))
	return f.locals[pos]
}

func (f *FileVisitor) findModule(ref types.Object) *packages.Module {
	mod, ok := f.pkgLookup[pkgPath(ref)]
	if !ok {
		if ref.Pkg() == nil {
			panic(fmt.Sprintf("Failed to find the thing for ref: %s | %+v\n", pkgPath(ref), ref))
		}

		mod = f.pkgLookup[ref.Pkg().Name()]
	}

	if mod == nil {
		panic(fmt.Sprintf("Very weird, can't figure out this reference: %s", ref))
	}

	return mod
}

func (f FileVisitor) Visit(n ast.Node) (w ast.Visitor) {
	if n == nil {
		return nil
	}

	switch node := n.(type) {
	case *ast.Ident:
		info := f.pkg.TypesInfo

		def := info.Defs[node]
		ref := info.Uses[node]

		pos := node.NamePos
		position := f.pkg.Fset.Position(pos)

		// This happens for composite structs
		//    We need to figure out a bit more information for this.
		//    Asked eric :)
		//
		// if def != nil && ref != nil {
		// 	panic("Didn't think this was possible")
		// }

		// Append definition
		if def != nil {
			var sym string
			if fieldSymbol, ok := f.fields[def.Pos()]; ok {
				sym = fieldSymbol
			} else {
				sym = f.createNewLocalSymbol(def.Pos())
			}

			f.doc.Occurrences = append(f.doc.Occurrences, &scip.Occurrence{
				Range:       scipRange(position, def),
				Symbol:      sym,
				SymbolRoles: int32(scip.SymbolRole_Definition),
			})
		}

		if ref != nil {
			var symbol string
			if localSymbol, ok := f.locals[ref.Pos()]; ok {
				symbol = localSymbol
			} else {
				mod, ok := f.pkgLookup[pkgPath(ref)]
				if !ok {
					if ref.Pkg() == nil {
						panic(fmt.Sprintf("Failed to find the thing for ref: %s | %+v\n", pkgPath(ref), ref))
					}

					mod = f.pkgLookup[ref.Pkg().Name()]
				}

				if mod == nil {
					panic(fmt.Sprintf("Very weird, can't figure out this reference: %s", ref))
				}

				switch ref := ref.(type) {
				case *types.Var:
					// For fields, we need to make sure they have the proper symbol path
					//    We iterate over the structs on the first pass to generate these
					//    fields, and then look them up on reference
					if ref.IsField() {
						symbol = f.fields[ref.Pos()]
					}

				case *types.Nil:
					return nil
				}

				if symbol == "" {
					symbol = scipSymbolFromObject(mod, ref)
				}
			}

			f.doc.Occurrences = append(f.doc.Occurrences, &scip.Occurrence{
				Range:       scipRange(position, ref),
				Symbol:      symbol,
				SymbolRoles: int32(scip.SymbolRole_ReadAccess),
			})
		}

	// explicit fail
	case *ast.File:
		panic("Should not find a file. Only call from within a file")

	case *ast.FuncDecl:
		// explicit pass

	default:
		// fmt.Printf("unhandled: %T %v\n", n, n)
	}

	return f
}
