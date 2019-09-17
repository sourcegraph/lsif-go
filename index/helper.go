package index

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	doc "github.com/slimsag/godocmd"
	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/ast/astutil"
)

// lspRange transforms go/token.Position (1-based) to LSP start and end ranges (0-based)
// which takes in consideration of identifier's name length.
func lspRange(pos token.Position, name string) (start protocol.Pos, end protocol.Pos) {
	return protocol.Pos{
			Line:      pos.Line - 1,
			Character: pos.Column - 1,
		}, protocol.Pos{
			Line:      pos.Line - 1,
			Character: pos.Column - 1 + len(name),
		}
}

// findContents returns contents used as hover info for given object.
func findContents(f *ast.File, obj types.Object) ([]protocol.MarkedString, error) {
	qf := func(*types.Package) string { return "" }
	var s string
	var extra string
	if f, ok := obj.(*types.Var); ok && f.IsField() {
		// TODO(jchen): make this be like (T).F not "struct field F string".
		s = "struct " + obj.String()
	} else {
		if obj, ok := obj.(*types.TypeName); ok {
			typ := obj.Type().Underlying()
			if _, ok := typ.(*types.Struct); ok {
				s = "type " + obj.Name() + " struct"
				extra = prettyPrintTypesString(types.TypeString(typ, qf))
			}
			if _, ok := typ.(*types.Interface); ok {
				s = "type " + obj.Name() + " interface"
				extra = prettyPrintTypesString(types.TypeString(typ, qf))
			}
		}
		if s == "" {
			s = types.ObjectString(obj, qf)
		}
	}

	contents := []protocol.MarkedString{
		protocol.NewMarkedString(s),
	}
	comments, err := findComments(f, obj)
	if err != nil {
		return nil, fmt.Errorf("find comments: %v", err)
	}
	if comments != "" {
		var b bytes.Buffer
		doc.ToMarkdown(&b, comments, nil)
		contents = append(contents, protocol.RawMarkedString(b.String()))
	}
	if extra != "" {
		contents = append(contents, protocol.NewMarkedString(extra))
	}
	return contents, nil
}

// prettyPrintTypesString is pretty printing specific to the output of
// types.*String. Instead of re-implementing the printer, we can just
// transform its output.
//
// This function is copied from
// https://sourcegraph.com/github.com/sourcegraph/go-langserver@02f4198/-/blob/langserver/hover.go#L332
func prettyPrintTypesString(s string) string {
	// Don't bother including the fields if it is empty
	if strings.HasSuffix(s, "{}") {
		return ""
	}
	var b bytes.Buffer
	b.Grow(len(s))
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ';':
			b.WriteByte('\n')
			for j := 0; j < depth; j++ {
				b.WriteString("    ")
			}
			// Skip following space
			i++

		case '{':
			if i == len(s)-1 {
				// This should never happen, but in case it
				// does give up
				return s
			}

			n := s[i+1]
			if n == '}' {
				// Do not modify {}
				b.WriteString("{}")
				// We have already written }, so skip
				i++
			} else {
				// We expect fields to follow, insert a newline and space
				depth++
				b.WriteString(" {\n")
				for j := 0; j < depth; j++ {
					b.WriteString("    ")
				}
			}

		case '}':
			depth--
			if depth < 0 {
				return s
			}
			b.WriteString("\n}")

		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// findComments traverses the paths found within enclosing interval of the object
// to collect comments.
//
// This function is modified from
// https://sourcegraph.com/github.com/sourcegraph/go-langserver@02f4198/-/blob/langserver/hover.go#L106
func findComments(f *ast.File, o types.Object) (string, error) {
	if o == nil {
		return "", nil
	}

	if _, ok := o.(*types.PkgName); ok {
		// TODO(jchen): add helper to find package doc
		return "", nil
	}

	// Resolve the object o into its respective ast.Node
	paths, _ := astutil.PathEnclosingInterval(f, o.Pos(), o.Pos())
	if paths == nil {
		return "", nil
	}

	// Pull the comment out of the comment map for the file. Do
	// not search too far away from the current path.
	var comments string
	for i := 0; i < 3 && i < len(paths) && comments == ""; i++ {
		switch v := paths[i].(type) {
		case *ast.Field:
			// Concat associated documentation with any inline comments
			comments = joinCommentGroups(v.Doc, v.Comment)
		case *ast.ValueSpec:
			comments = v.Doc.Text()
		case *ast.TypeSpec:
			comments = v.Doc.Text()
		case *ast.GenDecl:
			comments = v.Doc.Text()
		case *ast.FuncDecl:
			comments = v.Doc.Text()
		}
	}
	return comments, nil
}

// joinCommentGroups joins the resultant non-empty comment text from two
// CommentGroups with a newline.
//
// This function is copied from
// https://sourcegraph.com/github.com/sourcegraph/go-langserver@02f4198/-/blob/langserver/hover.go#L190
func joinCommentGroups(a, b *ast.CommentGroup) string {
	aText := a.Text()
	bText := b.Text()
	if aText == "" {
		return bText
	} else if bText == "" {
		return aText
	} else {
		return aText + "\n" + bText
	}
}
