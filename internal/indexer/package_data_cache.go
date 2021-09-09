package indexer

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// PackageDataCache is a cache of hover text and enclosing type identifiers by file and token position.
type PackageDataCache struct {
	m           sync.RWMutex
	packageData map[*packages.Package]*PackageData
}

// NewPackageDataCache creates a new empty PackageDataCache.
func NewPackageDataCache() *PackageDataCache {
	return &PackageDataCache{
		packageData: map[*packages.Package]*PackageData{},
	}
}

// Text will return the hover text extracted from the given package for the symbol at the given position.
// This method will parse the package if the package results haven't been previously calculated or have been
// evicted from the cache.
func (l *PackageDataCache) Text(p *packages.Package, position token.Pos) string {
	return extractHoverText(l.getPackageData(p).HoverText[position])
}

// MonikerPath will return the names of enclosing nodes extracted form the given package for the symbol at
// the given position. This method will parse the package if the package results haven't been previously
// calculated or have been evicted  from the cache.
func (l *PackageDataCache) MonikerPath(p *packages.Package, position token.Pos) []string {
	return l.getPackageData(p).MonikerPaths[position]
}

// Stats returns a PackageDataCacheStats object with the number of unique packages traversed.
func (l *PackageDataCache) Stats() PackageDataCacheStats {
	return PackageDataCacheStats{
		NumPks: uint(len(l.packageData)),
	}
}

// getPackageData will return a package data value for the given package. If the data for this package has not
// already been loaded, it will be loaded immediately. This method will block until the package data has been
// completely loaded before returning to the caller.
func (l *PackageDataCache) getPackageData(p *packages.Package) *PackageData {
	data := l.getPackageDataRaw(p)
	data.load(p)
	return data
}

// getPackageDataRaw will return the package data value for the given package or create one of it doesn't exist.
// It is not guaranteed that the value has bene loaded, so load (which is idempotent) should be called before use.
func (l *PackageDataCache) getPackageDataRaw(p *packages.Package) *PackageData {
	l.m.RLock()
	data, ok := l.packageData[p]
	l.m.RUnlock()
	if ok {
		return data
	}

	l.m.Lock()
	defer l.m.Unlock()
	if data, ok = l.packageData[p]; ok {
		return data
	}

	data = &PackageData{
		HoverText:    map[token.Pos]ast.Node{},
		MonikerPaths: map[token.Pos][]string{},
	}
	l.packageData[p] = data
	return data
}

// PackageData is a cache of hover text and moniker paths by token position within a package.
type PackageData struct {
	once         sync.Once
	HoverText    map[token.Pos]ast.Node
	MonikerPaths map[token.Pos][]string
}

// load will parse the package and populate the maps of hover text and moniker paths. This method is
// idempotent. All calls to this method will block until the first call has completed.
func (data *PackageData) load(p *packages.Package) {
	data.once.Do(func() {
		definitionPositions, fieldPositions := interestingPositions(p)

		for _, root := range p.Syntax {
			visit(root, definitionPositions, fieldPositions, data.HoverText, data.MonikerPaths, nil, nil)
		}
	})
}

// interestingPositions returns a pair of maps whose keys are token positions for which we want values
// in the package data cache's hoverText and monikerPaths maps. Determining which types of types we will
// query for this data and populating values only for those nodes saves a lot of resident memory.
func interestingPositions(p *packages.Package) (map[token.Pos]struct{}, map[token.Pos]struct{}) {
	hoverTextPositions := map[token.Pos]struct{}{}
	monikerPathPositions := map[token.Pos]struct{}{}

	for _, obj := range p.TypesInfo.Defs {
		if shouldHaveHoverText(obj) {
			hoverTextPositions[obj.Pos()] = struct{}{}
		}
		if isField(obj) {
			monikerPathPositions[obj.Pos()] = struct{}{}
		}
	}

	for _, obj := range p.TypesInfo.Uses {
		if isField(obj) {
			monikerPathPositions[obj.Pos()] = struct{}{}
		}
	}

	return hoverTextPositions, monikerPathPositions
}

// visit walks the AST for a file and assigns hover text and a moniker path to interesting positions.
// A position's hover text is the comment associated with the deepest node that encloses the position.
// A position's moniker path is the name of the object prefixed with the names of the containers that
// enclose that position.
func visit(
	node ast.Node, // Current node
	hoverTextPositions map[token.Pos]struct{}, // Positions for hover text assignment
	monikerPathPositions map[token.Pos]struct{}, // Positions for moniker paths assignment
	hoverTextMap map[token.Pos]ast.Node, // Target hover text map
	monikerPathMap map[token.Pos][]string, // Target moniker path map
	nodeWithHoverText ast.Node, // The ancestor node with non-empty hover text (if any)
	monikerPath []string, // The moniker path constructed up to this node
) {
	if canExtractHoverText(node) {
		// If we have hover text replace whatever ancestor node we might
		// have. We have more relevant text on this node, so just use that.
		nodeWithHoverText = node
	}

	// If we're a field or type, update our moniker path
	newMonikerPath := updateMonikerPath(monikerPath, node)

	for _, child := range childrenOf(node) {
		visit(
			child,
			hoverTextPositions,
			monikerPathPositions,
			hoverTextMap,
			monikerPathMap,
			chooseNodeWithHoverText(node, child),
			newMonikerPath,
		)
	}

	if _, ok := hoverTextPositions[node.Pos()]; ok {
		hoverTextMap[node.Pos()] = nodeWithHoverText
	}
	if _, ok := monikerPathPositions[node.Pos()]; ok {
		monikerPathMap[node.Pos()] = newMonikerPath
	}
}

// updateMonikerPath returns the given slice plus the name of the given node if it has a name that
// can uniquely identify it along a path of nodes to the root of the file (an enclosing type).
// Otherwise, the given slice is returned unchanged. This function does not modify the input slice.
func updateMonikerPath(monikerPath []string, node ast.Node) []string {
	switch q := node.(type) {
	case *ast.Field:
		// Handle field name/names
		if len(q.Names) > 0 {
			// Handle things like `a, b, c T`. If there are multiple names we just default to the first
			// one as each field must belong on at most one moniker path. This is sub-optimal and
			// should be addressed in https://github.com/sourcegraph/lsif-go/issues/154.
			return addString(monikerPath, q.Names[0].String())
		}

		// Handle embedded types
		if name, ok := q.Type.(*ast.Ident); ok {
			return addString(monikerPath, name.Name)
		}

		// Handle embedded types that are selectors, like http.Client
		if selector, ok := q.Type.(*ast.SelectorExpr); ok {
			return addString(monikerPath, selector.Sel.Name)
		}

	case *ast.TypeSpec:
		// Add the top-level type spec (e.g. `type X struct` and `type Y interface`)
		return addString(monikerPath, q.Name.String())
	}

	return monikerPath
}

// addString creates a new slice composed of the element of slice plus the given value.
// This function does not modify the input slice.
func addString(slice []string, value string) []string {
	newSlice := make([]string, len(slice), len(slice)+1)
	copy(newSlice, slice)
	newSlice = append(newSlice, value)
	return newSlice
}

// childrenOf returns the direct non-nil children of ast.Node n.
func childrenOf(n ast.Node) (children []ast.Node) {
	ast.Inspect(n, func(node ast.Node) bool {
		if node == n {
			return true
		}
		if node != nil {
			children = append(children, node)
		}
		return false
	})

	return children
}

// isField returns true if the given object is a field.
func isField(obj ObjectLike) bool {
	if v, ok := obj.(*types.Var); ok && v.IsField() {
		return true
	}
	return false
}

// shouldHaveHoverText returns true if the object is a type for which we should store hover text. This
// is similar but distinct from the set of types from which we _extract_ hover text. See canExtractHoverText
// for those types. This function returns true for the set of objects for which we actually call the methods
// findHoverContents  or findExternalHoverContents (see hover.go).
func shouldHaveHoverText(obj ObjectLike) bool {
	switch obj.(type) {
	case *types.Const:
		return true
	case *types.Func:
		return true
	case *types.Label:
		return true
	case *types.TypeName:
		return true
	case *types.Var:
		return true
	}

	return false
}

// extractHoverText returns the comments attached to the given node.
func extractHoverText(node ast.Node) string {
	switch v := node.(type) {
	case *ast.FuncDecl:
		return v.Doc.Text()
	case *ast.GenDecl:
		return v.Doc.Text()
	case *ast.TypeSpec:
		return v.Doc.Text()
	case *ast.ValueSpec:
		return v.Doc.Text()
	case *ast.Field:
		return strings.TrimSpace(v.Doc.Text() + "\n" + v.Comment.Text())
	}

	return ""
}

// canExtractHoverText returns true if the node has non-empty comments extractable by extractHoverText.
func canExtractHoverText(node ast.Node) bool {
	switch v := node.(type) {
	case *ast.FuncDecl:
		return !commentGroupsEmpty(v.Doc)
	case *ast.GenDecl:
		return !commentGroupsEmpty(v.Doc)
	case *ast.TypeSpec:
		return !commentGroupsEmpty(v.Doc)
	case *ast.ValueSpec:
		return !commentGroupsEmpty(v.Doc)
	case *ast.Field:
		return !commentGroupsEmpty(v.Doc, v.Comment)
	}

	return false
}

// commentGroupsEmpty returns true if all of the given comments groups are empty.
func commentGroupsEmpty(gs ...*ast.CommentGroup) bool {
	for _, g := range gs {
		if g != nil && len(g.List) > 0 {
			return false
		}
	}

	return true
}

// chooseNodeWithHoverText returns the parent node if the relationship between the parent and child is
// one in which comments can be reasonably shared. This will return a nil node for most relationships,
// except things like (1) FuncDecl -> Ident, in which case we want to store the function's comment
// in the ident, or (2) GenDecl -> TypeSpec, in which case we want to store the generic declaration's
// comments if the type node doesn't have any directly attached to it.
func chooseNodeWithHoverText(parent, child ast.Node) ast.Node {
	if _, ok := parent.(*ast.GenDecl); ok {
		return parent
	}
	if _, ok := child.(*ast.Ident); ok {
		return parent
	}

	return nil
}
