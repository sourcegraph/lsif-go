package index

import (
	"go/ast"
	"go/token"
	"sort"
	"sync"
)

type HoverLoader struct {
	m     sync.RWMutex
	cache map[*ast.File]map[token.Pos]string
}

// newHoverLoader creates a new empty HoverLoader.
func newHoverLoader() *HoverLoader {
	return &HoverLoader{
		cache: map[*ast.File]map[token.Pos]string{},
	}
}

// Load will walk the AST of the file and cache the hover text for each of the given positions.
func (l *HoverLoader) Load(root *ast.File, positions []token.Pos) {
	textMap := map[token.Pos]string{}
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	visit(root, positions, textMap, nil)

	l.m.Lock()
	l.cache[root] = textMap
	l.m.Unlock()
}

// Text will return the hover text extracted from the given file. For non-empty hover text to be
// returned from this method, Load must have been previously called with this file and position
// as arguments.
func (l *HoverLoader) Text(root *ast.File, position token.Pos) string {
	l.m.RLock()
	text := l.cache[root][position]
	l.m.RUnlock()

	return text
}

// visit walks the AST for a file and assigns hover text to each position. A position's hover text
// is the comment associated with the deepest node that encloses the position. Each call to visit
// is given the unique path of ancestors from the root to the parent of the node. This slice should
// not be directly altered.
func visit(node ast.Node, positions []token.Pos, textMap map[token.Pos]string, path []ast.Node) {
	newPath := append(append([]ast.Node(nil), path...), node)

	for _, child := range childrenOf(node) {
		visit(child, positions, textMap, newPath)
	}

	for i := findFirstIntersectingIndex(node, positions); i < len(positions) && positions[i] <= node.End(); i++ {
		if _, ok := textMap[positions[i]]; ok {
			continue
		}

		textMap[positions[i]] = commentsFromPath(newPath)
	}
}

// findFirstIntersectingIndex finds the first index in positions that is not less than the
// node's starting position. If there is no such index, then the length of the array is
// returned.
func findFirstIntersectingIndex(node ast.Node, positions []token.Pos) int {
	i := 0
	for i < len(positions) && positions[i] < node.Pos() {
		i = (i + 1) * 2
	}

	if i >= len(positions) {
		i = len(positions)
	}

	for i > 0 && positions[i-1] >= node.Pos() {
		i--
	}

	return i
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

const MaxCommentDistance = 3

// commentsFromPath searches the given node path backwards and returns the first comment
// attached to o node that it finds. This will only look at the last MaxCommentDistance
// nodes of the given path.
func commentsFromPath(path []ast.Node) (comment string) {
	for i := 0; i < len(path) && i < MaxCommentDistance && comment == ""; i++ {
		switch v := path[len(path)-i-1].(type) {
		case *ast.Field:
			// Concat associated documentation with any inline comments
			comment = joinCommentGroups(v.Doc, v.Comment)
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
