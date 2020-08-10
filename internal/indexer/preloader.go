package indexer

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// Preloader is a cache of hover text by file and token position.
type Preloader struct {
	m            sync.RWMutex
	hoverText    map[*ast.File]map[token.Pos]string
	monikerPaths map[*ast.File]map[token.Pos][]string
}

// Preloader creates a new empty Preloader.
func newPreloader() *Preloader {
	return &Preloader{
		hoverText:    map[*ast.File]map[token.Pos]string{},
		monikerPaths: map[*ast.File]map[token.Pos][]string{},
	}
}

// Load will walk the AST of the file and cache the hover text and moniker paths for each of the
// given positions. This function assumes that the given positions are already ordered so that
// a binary-search can be used to efficiently bound lookups.
func (l *Preloader) Load(root *ast.File, positions []token.Pos) {
	hoverTextMap := map[token.Pos]string{}
	monikerPathMap := map[token.Pos][]string{}
	visit(root, positions, hoverTextMap, monikerPathMap, nil, nil)

	l.m.Lock()
	l.hoverText[root] = hoverTextMap
	l.monikerPaths[root] = monikerPathMap
	l.m.Unlock()
}

// Text will return the hover text extracted from the given file. For non-empty hover text to be
// returned from this method, Load must have been previously called with this file and position
// as arguments.
func (l *Preloader) Text(f *ast.File, position token.Pos) string {
	l.m.RLock()
	defer l.m.RUnlock()
	return l.hoverText[f][position]
}

// TextFromPackage will return the hover text extracted from the given package. For non-empty hover
// text to be returned from this method, Load must have been previously called with a file contained
// in this package and this position as arguments.
func (l *Preloader) TextFromPackage(p *packages.Package, position token.Pos) string {
	l.m.RLock()
	defer l.m.RUnlock()

	for _, f := range p.Syntax {
		if text := l.hoverText[f][position]; text != "" {
			return text
		}
	}

	return ""
}

func (l *Preloader) MonikerPath(f *ast.File, position token.Pos) []string {
	l.m.RLock()
	defer l.m.RUnlock()
	return l.monikerPaths[f][position]
}

// visit walks the AST for a file and assigns hover text and a moniker path to each position. A
// position's hover text is the comment associated with the deepest node that encloses the position.
// A position's moniker path is the name of the object prefixed with the names of the containers that
// enclose that position. Each call to visit is given the unique path of ancestors from the root to
// the parent of the node. This slice should not be directly altered.
func visit(
	node ast.Node,
	positions []token.Pos,
	hoverTextMap map[token.Pos]string,
	monikerPathMap map[token.Pos][]string,
	path []ast.Node, monikerPath []string,
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
			hoverTextMap[positions[i]] = commentsFromPath(newPath)
		}
	}

	monikerPathMap[node.Pos()] = newMonikerPath
}

// updateNodePath appends the given node to the given path. This function does not modify
// the input slice.
func updateNodePath(path []ast.Node, node ast.Node) []ast.Node {
	return append(append([]ast.Node(nil), path...), node)
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
			return append(append([]string(nil), monikerPath...), q.Names[0].String())
		}

	case *ast.TypeSpec:
		// Add the top-level type spec (e.g. `type X struct` and `type Y interface`)
		return append(append([]string(nil), monikerPath...), q.Name.String())
	}

	return monikerPath
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

const maxCommentDistance = 3

// commentsFromPath searches the given node path backwards and returns the first comment
// attached to the node that it finds. This will only look at the last MaxCommentDistance
// nodes of the given path.
func commentsFromPath(path []ast.Node) (comment string) {
	for i := 0; i < len(path) && i < maxCommentDistance && comment == ""; i++ {
		switch v := path[len(path)-i-1].(type) {
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
