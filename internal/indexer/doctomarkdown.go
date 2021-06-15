package indexer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/reader"
)

// doctomarkdown converts the LSIF data in r to Markdown by scanning for data in the Sourcegraph LSIF
// documentation extension format.
func doctomarkdown(ctx context.Context, r io.Reader, matchingTags []protocol.DocumentationTag) (string, error) {
	stream := reader.Read(ctx, r)
	var buf bytes.Buffer
	conv := &converter{
		pairs:        map[int]reader.Pair{},
		out:          &buf,
		matchingTags: matchingTags,
	}
	for {
		pair, ok := <-stream
		if !ok {
			break
		}
		conv.pairs[pair.Element.ID] = pair
	}
	if err := conv.convert(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type converter struct {
	pairs        map[int]reader.Pair
	out          io.Writer
	matchingTags []protocol.DocumentationTag

	byStringOutV, byChildrenOutV map[int][]reader.Pair
}

func (c *converter) findElementByID(id int, typ, description string) (*reader.Element, error) {
	p, ok := c.pairs[id]
	if ok {
		if p.Element.Type == typ {
			if p.Err != nil {
				return nil, fmt.Errorf("%s ID=%d (%s) has error: %s", p.Element.Type, p.Element.ID, description, p.Err)
			}
			return &p.Element, nil
		}
	}
	return nil, fmt.Errorf("failed to find %s with ID=%d (%s)", typ, id, description)
}

func (c *converter) findVertexByID(id int, description string) (*reader.Element, error) {
	return c.findElementByID(id, "vertex", description)
}

func (c *converter) findEdgeByID(id int, description string) (*reader.Element, error) {
	return c.findElementByID(id, "edge", description)
}

func (c *converter) convert() error {
	// Find the root "documentationResult" vertex
	var root *reader.Element
	for _, p := range c.pairs {
		if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeSourcegraphDocumentationResult) {
			if p.Err != nil {
				return fmt.Errorf(`root "documentationResult" edge error: %s`, p.Err)
			}
			edge := p.Element.Payload.(reader.Edge)
			documentationResultVertexID := edge.InV
			projectOrResultSetID := edge.OutV

			projectOrResultSet, err := c.findVertexByID(projectOrResultSetID, "'project' or 'resultSet'")
			if err != nil {
				return err
			}
			if projectOrResultSet.Label != string(protocol.VertexProject) {
				// this documentationResult is attached to a ResultSet, e.g. in response to a hover
				// request so we ignore it.
				continue
			}

			root, err = c.findVertexByID(documentationResultVertexID, "'documentationResult'")
			if err != nil {
				return err
			}
			break
		}
	}
	if root == nil {
		return fmt.Errorf("no edge %q referencing a 'project' vertex found", protocol.EdgeSourcegraphDocumentationResult)
	}

	// Recurse on the root element's children, emitting documentation as we go.
	return c.recurse(root, 0, "")
}

func (c *converter) findDocumentationStringsFor(result int) (label, detail *reader.Element, err error) {
	if c.byStringOutV == nil {
		c.byStringOutV = map[int][]reader.Pair{}
		for _, p := range c.pairs {
			if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeSourcegraphDocumentationString) {
				if p.Err != nil {
					return nil, nil, fmt.Errorf(`"documentationString" edge error: %s`, p.Err)
				}
				edge := p.Element.Payload.(reader.DocumentationStringEdge)
				c.byStringOutV[edge.OutV] = append(c.byStringOutV[edge.OutV], p)
			}
		}
	}

	for _, p := range c.byStringOutV[result] {
		edge := p.Element.Payload.(reader.DocumentationStringEdge)
		string, err := c.findVertexByID(edge.InV, "'documentationString'")
		if err != nil {
			return nil, nil, err
		}
		switch edge.Kind {
		case protocol.DocumentationStringKindLabel:
			label = string
		case protocol.DocumentationStringKindDetail:
			detail = string
		default:
			panic("never here")
		}
	}
	if label == nil && detail == nil {
		return nil, nil, fmt.Errorf(`failed to find "documentationString"s for "documentationResult" ID=%v`, result)
	}
	if label == nil {
		return nil, nil, fmt.Errorf(`failed to find "documentationString" label for "documentationResult" ID=%v`, result)
	}
	if detail == nil {
		return nil, nil, fmt.Errorf(`failed to find "documentationString" detail for "documentationResult" ID=%v`, result)
	}
	return
}

func (c *converter) findDocumentationChildrenFor(result int) (children []*reader.Element, err error) {
	if c.byChildrenOutV == nil {
		c.byChildrenOutV = map[int][]reader.Pair{}
		for _, p := range c.pairs {
			if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeSourcegraphDocumentationChildren) {
				if p.Err != nil {
					return nil, fmt.Errorf(`"documentationChildren" edge error: %s`, p.Err)
				}
				edge := p.Element.Payload.(reader.Edge)
				c.byChildrenOutV[edge.OutV] = append(c.byChildrenOutV[edge.OutV], p)
			}
		}
	}

	for _, p := range c.byChildrenOutV[result] {
		edge := p.Element.Payload.(reader.Edge)
		for _, childID := range edge.InVs {
			child, err := c.findVertexByID(childID, "'documentationChildren'")
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
	}
	return
}

func (c *converter) recurse(this *reader.Element, depth int, slug string) error {
	labelVertex, detailVertex, err := c.findDocumentationStringsFor(this.ID)
	if err != nil {
		return err
	}
	label := labelVertex.Payload.(protocol.MarkupContent)
	detail := detailVertex.Payload.(protocol.MarkupContent)
	doc := this.Payload.(protocol.Documentation)

	if !tagsMatch(c.matchingTags, doc.Tags) {
		return nil
	}
	emptyRootDocumentation := depth == 0 && detail.Value == ""
	if !emptyRootDocumentation {
		slug = joinSlugs(slug, doc.Slug)
	}
	depthStr := strings.Repeat("#", depth+1)
	var infos []string
	if doc.NewPage {
		infos = append(infos, "new page")
	}
	for _, tag := range doc.Tags {
		infos = append(infos, string(tag))
	}
	annotations := strings.Join(infos, ",")
	if annotations != "" {
		annotations = fmt.Sprintf(" <small>(%s)</small>", annotations)
	}
	if _, err := fmt.Fprintf(c.out, "%s <a name=\"%s\">%s%s</a>\n\n", depthStr, slug, label.Value, annotations); err != nil {
		return err
	}
	if detail.Value != "" {
		if _, err := fmt.Fprintf(c.out, "%s", detail.Value); err != nil {
			return err
		}
	}
	if doc.NewPage {
		if err := c.recurseIndex(this, 0, depth, false, ""); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.out, "\n"); err != nil {
			return err
		}
	}

	children, err := c.findDocumentationChildrenFor(this.ID)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := c.recurse(child, depth+1, slug); err != nil {
			return err
		}
	}
	return nil
}

func (c *converter) recurseIndex(this *reader.Element, depth, parentDepth int, end bool, slug string) error {
	labelVertex, _, err := c.findDocumentationStringsFor(this.ID)
	if err != nil {
		return err
	}
	label := labelVertex.Payload.(protocol.MarkupContent)
	doc := this.Payload.(protocol.Documentation)

	if !tagsMatch(c.matchingTags, doc.Tags) {
		return nil
	}

	if parentDepth != 0 || depth != 0 {
		slug = joinSlugs(slug, doc.Slug)
	}
	if depth == 0 {
		if _, err := fmt.Fprintf(c.out, "%s Index\n\n", strings.Repeat("#", parentDepth+1)); err != nil {
			return err
		}
	} else {
		depthStr := strings.Repeat("  ", depth-1)
		if _, err := fmt.Fprintf(c.out, "%s- [%s](#%s)\n", depthStr, label.Value, slug); err != nil {
			return err
		}
	}

	if end {
		return nil
	}
	children, err := c.findDocumentationChildrenFor(this.ID)
	if err != nil {
		return err
	}
	for _, child := range children {
		childDoc := child.Payload.(protocol.Documentation)
		if err := c.recurseIndex(child, depth+1, parentDepth, childDoc.NewPage, slug); err != nil {
			return err
		}
	}
	return nil
}

func tagsMatch(want, have []protocol.DocumentationTag) bool {
	for _, want := range want {
		got := false
		for _, have := range have {
			if have == want {
				got = true
			}
		}
		if !got {
			return false
		}
	}
	return true
}

func joinSlugs(a, b string) string {
	s := []string{}
	if a != "" {
		s = append(s, a)
	}
	if b != "" {
		s = append(s, b)
	}
	return strings.Join(s, "-")
}
