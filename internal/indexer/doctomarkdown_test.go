package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
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

type DocsRefDefInfo struct {
	Document string
	Ranges   []reader.Range
}

type converter struct {
	pairs        map[int]reader.Pair
	out          io.Writer
	matchingTags []protocol.DocumentationTag

	byStringOutV, byChildrenOutV map[int][]reader.Pair

	resultSets                       map[int]struct{}
	documents                        map[int]string
	ranges                           map[int]reader.Range
	resultSetToDocumentationResult   map[int]int
	documentationResultToHoverResult map[int]interface{} // string or protocol.HoverResult
	documentationResultToDefinitions map[int][]DocsRefDefInfo
	documentationResultToReferences  map[int][]DocsRefDefInfo
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
	if err := c.correlateGeneral(); err != nil {
		return err
	}
	if err := c.correlateDocumentationResultToHoverResult(); err != nil {
		return err
	}
	if err := c.correlateDocumentationResultToDefinitions(); err != nil {
		return err
	}
	if err := c.correlateDocumentationResultToReferences(); err != nil {
		return err
	}

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

func (c *converter) correlateGeneral() error {
	projectRoot := ""
	for _, p := range c.pairs {
		if p.Element.Type == "vertex" && p.Element.Label == string(protocol.VertexMetaData) {
			metadata := p.Element.Payload.(reader.MetaData)
			projectRoot = metadata.ProjectRoot
			break
		}
	}

	c.resultSets = map[int]struct{}{}
	c.documents = map[int]string{}
	c.ranges = map[int]reader.Range{}
	for _, p := range c.pairs {
		if p.Element.Type == "vertex" && p.Element.Label == "resultSet" {
			c.resultSets[p.Element.ID] = struct{}{}
		}
		if p.Element.Type == "vertex" && p.Element.Label == string(protocol.VertexDocument) {
			c.documents[p.Element.ID] = strings.TrimPrefix(p.Element.Payload.(string), projectRoot)
		}
		if p.Element.Type == "vertex" && p.Element.Label == string(protocol.VertexRange) {
			c.ranges[p.Element.ID] = p.Element.Payload.(reader.Range)
		}
	}

	// resultSet ID -> documentationResult IDs
	c.resultSetToDocumentationResult = map[int]int{}
	for _, p := range c.pairs {
		if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeSourcegraphDocumentationResult) {
			edge := p.Element.Payload.(reader.Edge)
			documentationResultVertexID := edge.InV
			projectOrResultSetID := edge.OutV
			if _, isResultSet := c.resultSets[projectOrResultSetID]; isResultSet {
				c.resultSetToDocumentationResult[projectOrResultSetID] = documentationResultVertexID
			}
		}
	}
	return nil
}

func (c *converter) correlateDocumentationResultToHoverResult() error {
	// documentationResult ID -> hoverResult
	hoverResultIDToResultSetID := map[int]int{}
	for _, p := range c.pairs {
		if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeTextDocumentHover) {
			edge := p.Element.Payload.(reader.Edge)
			if _, isResultSet := c.resultSets[edge.OutV]; isResultSet {
				hoverResultIDToResultSetID[edge.InV] = edge.OutV
			}
		}
	}
	c.documentationResultToHoverResult = map[int]interface{}{}
	for _, p := range c.pairs {
		if p.Element.Type == "vertex" && p.Element.Label == string(protocol.VertexHoverResult) {
			resultSet := hoverResultIDToResultSetID[p.Element.ID]
			documentationResult := c.resultSetToDocumentationResult[resultSet]
			hoverResultString, ok := p.Element.Payload.(string)
			if ok {
				c.documentationResultToHoverResult[documentationResult] = hoverResultString
			} else {
				c.documentationResultToHoverResult[documentationResult] = p.Element.Payload.(protocol.HoverResult)
			}
		}
	}
	return nil
}

func (c *converter) correlateDocumentationResultToDefinitions() error {
	// documentationResult ID -> DocsDefinitionInfo
	definitionResultToDocumentID := map[int]int{}
	definitionResultToRangeIDs := map[int][]int{}
	for _, p := range c.pairs {
		if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeItem) {
			edge := p.Element.Payload.(reader.Edge)
			definitionResultID := edge.OutV
			rangeIDs := edge.InVs
			documentID := edge.Document
			definitionResultToRangeIDs[definitionResultID] = rangeIDs
			definitionResultToDocumentID[definitionResultID] = documentID
		}
	}
	c.documentationResultToDefinitions = map[int][]DocsRefDefInfo{}
	for _, p := range c.pairs {
		if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeTextDocumentDefinition) {
			edge := p.Element.Payload.(reader.Edge)
			definitionResult := edge.InV
			resultSet := edge.OutV
			if _, isResultSet := c.resultSets[resultSet]; !isResultSet {
				continue
			}
			documentationResult, ok := c.resultSetToDocumentationResult[resultSet]
			if !ok {
				continue
			}
			documentID, ok := definitionResultToDocumentID[definitionResult]
			if !ok {
				continue
			}
			rangeIDs, ok := definitionResultToRangeIDs[definitionResult]
			if !ok {
				continue
			}

			var decodedRanges []reader.Range
			for _, id := range rangeIDs {
				decodedRanges = append(decodedRanges, c.ranges[id])
			}
			c.documentationResultToDefinitions[documentationResult] = append(
				c.documentationResultToDefinitions[documentationResult],
				DocsRefDefInfo{
					Document: c.documents[documentID],
					Ranges:   decodedRanges,
				},
			)
		}
	}
	for _, definitions := range c.documentationResultToDefinitions {
		sort.Slice(definitions, func(i, j int) bool {
			return definitions[i].Document < definitions[j].Document
		})
	}
	return nil
}

func (c *converter) correlateDocumentationResultToReferences() error {
	// documentationResult ID -> DocsReferenceInfo
	referenceResultToDocumentID := map[int]int{}
	referenceResultToRangeIDs := map[int][]int{}
	for _, p := range c.pairs {
		if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeItem) {
			edge := p.Element.Payload.(reader.Edge)
			referenceResultID := edge.OutV
			rangeIDs := edge.InVs
			documentID := edge.Document
			referenceResultToRangeIDs[referenceResultID] = rangeIDs
			referenceResultToDocumentID[referenceResultID] = documentID
		}
	}
	c.documentationResultToReferences = map[int][]DocsRefDefInfo{}
	for _, p := range c.pairs {
		if p.Element.Type == "edge" && p.Element.Label == string(protocol.EdgeTextDocumentReferences) {
			edge := p.Element.Payload.(reader.Edge)
			referenceResult := p.Element.ID
			resultSet := edge.OutV
			if _, isResultSet := c.resultSets[resultSet]; !isResultSet {
				continue
			}
			documentationResult := c.resultSetToDocumentationResult[resultSet]
			documentID := referenceResultToDocumentID[referenceResult]
			rangeIDs := referenceResultToRangeIDs[referenceResult]

			var decodedRanges []reader.Range
			for _, id := range rangeIDs {
				decodedRanges = append(decodedRanges, c.ranges[id])
			}
			c.documentationResultToReferences[documentationResult] = append(
				c.documentationResultToReferences[documentationResult],
				DocsRefDefInfo{
					Document: c.documents[documentID],
					Ranges:   decodedRanges,
				},
			)
		}
	}
	for _, references := range c.documentationResultToReferences {
		sort.Slice(references, func(i, j int) bool {
			return references[i].Document < references[j].Document
		})
	}
	return nil
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

func (c *converter) recurse(this *reader.Element, depth int, identifier string) error {
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
		identifier = joinIdentifiers(identifier, doc.Identifier)
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
	if _, err := fmt.Fprintf(c.out, "%s <a name=\"%s\">%s%s</a>\n\n", depthStr, identifier, label.Value, annotations); err != nil {
		return err
	}
	writeDetails := func(summary string, contents string) error {
		_, err := fmt.Fprintf(c.out, "<details><summary>%s</summary>\n\n%s\n\n</details>\n\n", summary, contents)
		return err
	}
	hover, ok := c.documentationResultToHoverResult[this.ID]
	if ok {
		lines := strings.Split(fmt.Sprint(hover), "\n")
		for i, line := range lines {
			lines[i] = "> " + line
		}
		writeDetails("hover", strings.Join(lines, "\n"))
	}
	definitions, ok := c.documentationResultToDefinitions[this.ID]
	if ok && len(definitions) > 0 {
		j, _ := json.MarshalIndent(definitions, "", " ")
		writeDetails("definitions", "```json\n"+string(j)+"\n```")
	}
	references, ok := c.documentationResultToDefinitions[this.ID]
	if ok && len(references) > 0 {
		j, _ := json.MarshalIndent(references, "", " ")
		writeDetails("references", "```json\n"+string(j)+"\n```")
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
		if err := c.recurse(child, depth+1, identifier); err != nil {
			return err
		}
	}
	return nil
}

func (c *converter) recurseIndex(this *reader.Element, depth, parentDepth int, end bool, identifier string) error {
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
		identifier = joinIdentifiers(identifier, doc.Identifier)
	}
	if depth == 0 {
		if _, err := fmt.Fprintf(c.out, "%s Index\n\n", strings.Repeat("#", parentDepth+1)); err != nil {
			return err
		}
	} else {
		depthStr := strings.Repeat("  ", depth-1)
		if _, err := fmt.Fprintf(c.out, "%s- [%s](#%s)\n", depthStr, label.Value, identifier); err != nil {
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
		if err := c.recurseIndex(child, depth+1, parentDepth, childDoc.NewPage, identifier); err != nil {
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

func joinIdentifiers(a, b string) string {
	s := []string{}
	if a != "" {
		s = append(s, a)
	}
	if b != "" {
		s = append(s, b)
	}
	return strings.Join(s, "-")
}
