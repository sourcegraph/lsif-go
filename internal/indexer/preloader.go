package indexer

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// Preloader is a cache of hover text and enclosing type identifiers by file and token position.
type Preloader struct {
	m            sync.RWMutex
	hoverText    map[*packages.Package]map[token.Pos]ast.Node
	monikerPaths map[*packages.Package]map[token.Pos][]string
}

// newPreloader creates a new empty Preloader.
func newPreloader() *Preloader {
	return &Preloader{
		hoverText:    map[*packages.Package]map[token.Pos]ast.Node{},
		monikerPaths: map[*packages.Package]map[token.Pos][]string{},
	}
}

// Load will walk the AST of each file in the given package and cache the hover text and moniker
// paths for each interesting position in the
func (l *Preloader) Load(p *packages.Package) {
	hoverTextPositions, monikerPathPositions := interestingPositions(p)
	hoverTextMap := map[token.Pos]ast.Node{}
	monikerPathMap := map[token.Pos][]string{}

	for _, root := range p.Syntax {
		visit(root, hoverTextPositions, monikerPathPositions, hoverTextMap, monikerPathMap, nil, nil)
	}

	l.m.Lock()
	l.hoverText[p] = hoverTextMap
	l.monikerPaths[p] = monikerPathMap
	l.m.Unlock()
}

// Text will return the hover text for the given token position extracted from the given package. For a
// non-empty hover text to be returned from this method, Load must have been previously called with this
// package as an argument.
func (l *Preloader) Text(p *packages.Package, position token.Pos) string {
	l.m.RLock()
	defer l.m.RUnlock()
	return extractHoverText(l.hoverText[p][position])
}

// MonikerPath returns the names of types enclosing the given position extracted from the given package.
// For a non-empty path to be returned from this method, Load must have been previously called with this
// package as an argument.
func (l *Preloader) MonikerPath(p *packages.Package, position token.Pos) []string {
	l.m.RLock()
	defer l.m.RUnlock()
	return l.monikerPaths[p][position]
}

// interestingPositions returns a pair of maps whose keys are token positions for which we want values
// in the preloader's hoverText and monikerPaths maps. Determining which types of types we will query
// for this data and populating values only for those nodes saves a lot of resident memory.
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
	hoverTextPositions map[token.Pos]struct{}, // Positions for which to assign hover text
	monikerPathPositions map[token.Pos]struct{}, // Positions for which to assign moniker paths
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
		if len(q.Names) > 0 {
			// Add names of distinct fields whose type is an anonymous struct type
			// containing the target field (e.g. `X struct { target string }`).
			return addString(monikerPath, q.Names[0].String())
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
func isField(obj types.Object) bool {
	if v, ok := obj.(*types.Var); ok && v.IsField() {
		return true
	}
	return false
}

// shouldHaveHoverText returns true if the object is a type for which we should store hover text. This
// is similar but distinct from the set of types from which we _extract_ hover text. See canExtractHoverText
// for those types. This function returns true for the set of objects for which we actually call the methods
// findHoverContents  or findExternalHoverContents (see hover.go).
func shouldHaveHoverText(obj types.Object) bool {
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
// comments if the type pnode doesn't have any directly attached to it.
func chooseNodeWithHoverText(parent, child ast.Node) ast.Node {
	if _, ok := parent.(*ast.GenDecl); ok {
		return parent
	}
	if _, ok := child.(*ast.Ident); ok {
		return parent
	}

	return nil
}
