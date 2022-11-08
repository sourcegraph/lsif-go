package index

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// FileVisitor visits an entire file, but it must be called
// after StructVisitor.
//
// Iterates over a file,
type FileVisitor struct {
	// Document to append occurrences to
	doc *Document

	// Current file information
	pkg  *packages.Package
	file *ast.File

	// soething
	pkgLookup map[string]*packages.Module

	// local definition position to symbol
	locals map[token.Pos]string

	// field definition position to symbol

	pkgSymbols    *PackageSymbols
	globalSymbols *GlobalSymbols
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

func (v FileVisitor) Visit(n ast.Node) (w ast.Visitor) {
	if n == nil {
		return nil
	}

	switch node := n.(type) {
	case *ast.Ident:
		info := v.pkg.TypesInfo

		pos := node.NamePos
		position := v.pkg.Fset.Position(pos)

		// Emit Definition
		def := info.Defs[node]
		if def != nil {
			var sym string
			if pkgSymbols, ok := v.pkgSymbols.get(def.Pos()); ok {
				sym = pkgSymbols
			} else if globalSymbol, ok := v.globalSymbols.get(v.pkg.PkgPath, def.Pos()); ok {
				fmt.Println("GLOBAL SYMBOL", globalSymbol)
				sym = globalSymbol
			} else {
				sym = v.createNewLocalSymbol(def.Pos())
			}

			v.doc.NewOccurrence(sym, scipRange(position, def))
		}

		// Emit Reference
		ref := info.Uses[node]
		if ref != nil {
			var symbol string
			if localSymbol, ok := v.locals[ref.Pos()]; ok {
				symbol = localSymbol
			} else {
				refPkgPath := pkgPath(ref)
				mod, ok := v.pkgLookup[refPkgPath]
				if !ok {
					if ref.Pkg() == nil {
						panic(fmt.Sprintf("Failed to find the thing for ref: %s | %+v\n", pkgPath(ref), ref))
					}

					mod = v.pkgLookup[ref.Pkg().Name()]
				}

				if mod == nil {
					// panic(fmt.Sprintf("Very weird, can't figure out this reference: %s", ref))
					return
				}

				switch ref := ref.(type) {
				case *types.Var:
					// For fields, we need to make sure they have the proper symbol path
					//    We iterate over the structs on the first pass to generate these
					//    fields, and then look them up on reference
					if ref.IsField() {
						symbol, _ = v.globalSymbols.get(refPkgPath, ref.Pos())
						// TODO: assert symbol?
					}

				case *types.Nil:
					return nil
				}

				if symbol == "" {
					symbol = scipSymbolFromObject(mod, ref)
				}
			}

			v.doc.appendSymbolReference(symbol, scipRange(position, ref))
		}
	}

	return v
}
