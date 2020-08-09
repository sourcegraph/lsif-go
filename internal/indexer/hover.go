package indexer

import (
	"fmt"
	"go/types"

	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/packages"
)

// findHoverContents returns the hover contents of the given object. This method is not cached
// and should only be called wrapped in a call to makeCachedHoverResult.
func findHoverContents(hoverLoader *HoverLoader, pkgs []*packages.Package, o ObjectInfo) []protocol.MarkedString {
	signature, extra := typeString(o.Object)
	docstring := findDocstring(hoverLoader, pkgs, o)
	return toMarkedString(signature, docstring, extra)
}

// findExternalHoverContents returns the hover contents of the given object defined in the given
// package. This method is not cached and should only be called wrapped in a call to makeCachedHoverResult.
func findExternalHoverContents(hoverLoader *HoverLoader, pkgs []*packages.Package, o ObjectInfo) []protocol.MarkedString {
	signature, extra := typeString(o.Object)
	docstring := findExternalDocstring(hoverLoader, pkgs, o)
	return toMarkedString(signature, docstring, extra)
}

// makeCachedHoverResult returns a hover result vertex identifier. If hover text for the given
// identifier has not already been emitted, a new vertex is created. Identifiers will share the
// same hover result if they refer to the same identifier in the same target package.
func (i *Indexer) makeCachedHoverResult(pkg *types.Package, obj types.Object, fn func() []protocol.MarkedString) uint64 {
	key := makeCacheKey(pkg, obj)

	if hoverResultID, ok := i.hoverResultCache[key]; ok {
		return hoverResultID
	}

	hoverResultID := i.emitter.EmitHoverResult(fn())
	if key != "" {
		// Do not store empty cache keys
		i.hoverResultCache[key] = hoverResultID
	}

	return hoverResultID
}

// makeCacheKey returns a string uniquely representing the given package and object pair. If
// the given package is not nil, the key is the concatenation of the package path and the object
// identifier. Otherwise, the key will be the object identifier if it refers to a package import.
// If the given package is nil and the object is not a package import, the returned cache key is
// the empty string (to force a fresh calculation of each local object's hover text).
func makeCacheKey(pkg *types.Package, obj types.Object) string {
	if pkg != nil {
		return fmt.Sprintf("%s::%d", pkg.Path(), obj.Pos())
	}

	if pkgName, ok := obj.(*types.PkgName); ok {
		return pkgName.Imported().Path()
	}

	return ""
}

// findDocstring extracts the comments form the given object. It is assumed that this object is
// declared in an index target (otherwise, findExternalDocstring should be called).
func findDocstring(hoverLoader *HoverLoader, pkgs []*packages.Package, o ObjectInfo) string {
	if o.Object == nil {
		return ""
	}

	switch v := o.Object.(type) {
	case *types.PkgName:
		return findPackageDocstring(pkgs, o.Package, v)
	}

	// Resolve the object o into its respective ast.Node
	return hoverLoader.Text(o.File, o.Object.Pos())
}

// findExternalDocstring extracts the comments form the given object. It is assumed that this object is
// declared in a dependency.
func findExternalDocstring(hoverLoader *HoverLoader, pkgs []*packages.Package, o ObjectInfo) string {
	if o.Object == nil {
		return ""
	}

	switch v := o.Object.(type) {
	case *types.PkgName:
		return findPackageDocstring(pkgs, o.Package, v)
	}

	if target := o.Package.Imports[o.Object.Pkg().Path()]; target != nil {
		// Resolve the object o into its respective ast.Node
		return hoverLoader.TextFromPackage(target, o.Object.Pos())
	}

	return ""
}

// findPackageDocstring searches for the package matching the target package name and returns its
// package-level documentation (the first doc text attached ot a file in the given package).
func findPackageDocstring(pkgs []*packages.Package, p *packages.Package, target *types.PkgName) string {
	pkgPath := target.Imported().Path()

	for _, p := range pkgs {
		if p.PkgPath == pkgPath {
			// The target package is an index target
			return extractPackageDocstring(p)
		}
	}

	if p, ok := p.Imports[pkgPath]; ok {
		// The target package is a dependency
		return extractPackageDocstring(p)
	}

	return ""
}

// extractPackagedocstring returns the first doc text attached to a file in the given package.
func extractPackageDocstring(p *packages.Package) string {
	for _, f := range p.Syntax {
		if f.Doc.Text() != "" {
			return f.Doc.Text()
		}
	}

	return "'"
}
