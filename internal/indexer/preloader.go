package indexer

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// nodePathLength is the number of ancestor nodes that will be searched when trying to extract a
// comment from a particular AST node.
const nodePathLength = 3

// nodePath is a fixed-size array of AST nodes.
type nodePath = [nodePathLength]ast.Node

// Preloader is a cache of hover text by file and token position.
type Preloader struct {
	m            sync.RWMutex
	hoverText    map[*packages.Package]map[token.Pos]nodePath
	monikerPaths map[*packages.Package]map[token.Pos][]string
}

// Preloader creates a new empty Preloader.
func newPreloader() *Preloader {
	return &Preloader{
		hoverText:    map[*packages.Package]map[token.Pos]nodePath{},
		monikerPaths: map[*packages.Package]map[token.Pos][]string{},
	}
}

// Load will walk the AST of each file in the given package and cache the hover text and moniker
// paths for each of the given positions. This function assumes that the given positions are already
// ordered so that a binary-search can be used to efficiently bound lookups.
func (l *Preloader) Load(p *packages.Package, positions []token.Pos) {
	hoverTextMap := map[token.Pos]nodePath{}
	monikerPathMap := map[token.Pos][]string{}

	for _, root := range p.Syntax {
		visit(root, positions, hoverTextMap, monikerPathMap, nodePath{}, nil)
	}

	l.m.Lock()
	l.hoverText[p] = hoverTextMap
	l.monikerPaths[p] = monikerPathMap
	l.m.Unlock()
}

// Text will return the hover text extracted from the given package. For non-empty hover text to
// be returned from this method, Load must have been previously called with this package and position
// as arguments.
func (l *Preloader) Text(p *packages.Package, position token.Pos) string {
	l.m.RLock()
	defer l.m.RUnlock()
	return commentsFromPath(l.hoverText[p][position])
}

func (l *Preloader) MonikerPath(p *packages.Package, position token.Pos) []string {
	l.m.RLock()
	defer l.m.RUnlock()
	return l.monikerPaths[p][position]
}

// visit walks the AST for a file and assigns hover text and a moniker path to each position. A
// position's hover text is the comment associated with the deepest node that encloses the position.
// A position's moniker path is the name of the object prefixed with the names of the containers that
// enclose that position. Each call to visit is given the unique path of ancestors from the root to
// the parent of the node. This slice should not be directly altered.
func visit(
	node ast.Node,
	positions []token.Pos,
	hoverTextMap map[token.Pos]nodePath,
	monikerPathMap map[token.Pos][]string,
	path nodePath,
	monikerPath []string,
) {
	newPath := updateNodePath(path, node)
	newMonikerPath := updateMonikerPath(monikerPath, node)
	start := sort.Search(len(positions), func(i int) bool {
		return positions[i] >= node.Pos()
	})

	end := start
	for end < len(positions) && positions[end] <= node.End() {
		end++
	}

	for _, child := range childrenOf(node) {
		visit(child, positions[start:end], hoverTextMap, monikerPathMap, newPath, newMonikerPath)
	}

	for i := start; i < end; i++ {
		if _, ok := hoverTextMap[positions[i]]; !ok {
			hoverTextMap[positions[i]] = newPath
		}
	}

	monikerPathMap[node.Pos()] = newMonikerPath
}

// updateNodePath creates a array composed of the previous path plus the given node. This function
// does not modify the input array.
func updateNodePath(path nodePath, node ast.Node) nodePath {
	newPath := nodePath{node}
	for i := 0; i < nodePathLength-1; i++ {
		newPath[i+1] = path[i]
	}
	return newPath
}

// updateMonikerPath appends to the given slice the name of the node if it has a name that
// can uniquely identify it along a path of nodes to the root of the file. Otherwise, the
// given slice is returned unchanged. This function does not modify the input slice.
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

// commentsFromPath returns the first non-empty comment attached to a node in the given path.
func commentsFromPath(path nodePath) (comment string) {
	for _, node := range path {
		if comment != "" || node == nil {
			break
		}

		switch v := node.(type) {
		case *ast.Field:
			// Concat associated documentation with any inline comments
			comment = joinNonEmpty(v.Doc.Text(), v.Comment.Text())
		case *ast.FuncDecl:
			comment = v.Doc.Text()
		case *ast.GenDecl:
			comment = v.Doc.Text()
		case *ast.TypeSpec:
			comment = v.Doc.Text()
		case *ast.ValueSpec:
			comment = v.Doc.Text()
		}
	}

	return comment
}

// joinNonEmpty removes empty strings from the input list and joins the remaining values
// with a newline.
func joinNonEmpty(values ...string) string {
	var parts []string
	for _, value := range values {
		if value != "" {
			parts = append(parts, value)
		}
	}

	return strings.Join(parts, "\n")
}
