package indexer

import (
	"fmt"
	"go/types"

	protocol "github.com/sourcegraph/lsif-protocol"
	"golang.org/x/tools/go/packages"
)

// findHoverContents returns the hover contents of the given object. This method is not cached
// and should only be called wrapped in a call to makeCachedHoverResult.
func findHoverContents(packageDataCache *PackageDataCache, pkgs []*packages.Package, p *packages.Package, obj types.Object) []protocol.MarkedString {
	signature, extra := typeString(obj)
	docstring := findDocstring(packageDataCache, pkgs, p, obj)
	return toMarkedString(signature, docstring, extra)
}

// findExternalHoverContents returns the hover contents of the given object defined in the given
// package. This method is not cached and should only be called wrapped in a call to makeCachedHoverResult.
func findExternalHoverContents(packageDataCache *PackageDataCache, pkgs []*packages.Package, p *packages.Package, obj types.Object) []protocol.MarkedString {
	signature, extra := typeString(obj)
	docstring := findExternalDocstring(packageDataCache, pkgs, p, obj)
	return toMarkedString(signature, docstring, extra)
}

// makeCachedHoverResult returns a hover result vertex identifier. If hover text for the given
// identifier has not already been emitted, a new vertex is created. Identifiers will share the
// same hover result if they refer to the same identifier in the same target package.
func (i *Indexer) makeCachedHoverResult(pkg *types.Package, obj types.Object, fn func() []protocol.MarkedString) uint64 {
	key := makeCacheKey(pkg, obj)
	if key == "" {
		// Do not store empty cache keys
		return i.emitter.EmitHoverResult(fn())
	}

	i.hoverResultCacheMutex.RLock()
	hoverResultID, ok := i.hoverResultCache[key]
	i.hoverResultCacheMutex.RUnlock()
	if ok {
		return hoverResultID
	}

	// Note: we calculate this outside of the critical section
	contents := fn()

	i.hoverResultCacheMutex.Lock()
	defer i.hoverResultCacheMutex.Unlock()

	if hoverResultID, ok := i.hoverResultCache[key]; ok {
		return hoverResultID
	}

	hoverResultID = i.emitter.EmitHoverResult(contents)
	i.hoverResultCache[key] = hoverResultID
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

// findDocstring extracts the comments from the given object. It is assumed that this object is
// declared in an index target (otherwise, findExternalDocstring should be called).
func findDocstring(packageDataCache *PackageDataCache, pkgs []*packages.Package, p *packages.Package, obj types.Object) string {
	if obj == nil {
		return ""
	}

	if v, ok := obj.(*types.PkgName); ok {
		return findPackageDocstring(pkgs, p, v)
	}

	// Resolve the object into its respective ast.Node
	return packageDataCache.Text(p, obj.Pos())
}

// findExternalDocstring extracts the comments from the given object. It is assumed that this object is
// declared in a dependency.
func findExternalDocstring(packageDataCache *PackageDataCache, pkgs []*packages.Package, p *packages.Package, obj types.Object) string {
	if obj == nil {
		return ""
	}

	if v, ok := obj.(*types.PkgName); ok {
		return findPackageDocstring(pkgs, p, v)
	}

	if target := p.Imports[obj.Pkg().Path()]; target != nil {
		// Resolve the object obj into its respective ast.Node
		return packageDataCache.Text(target, obj.Pos())
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
		if text := f.Doc.Text(); text != "" {
			return text
		}
	}

	return ""
}
