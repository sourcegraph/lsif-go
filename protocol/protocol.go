package protocol

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

/*
	Reference: https://github.com/microsoft/lsif-node/blob/master/protocol/src/protocol.ts
*/

const (
	// Version represnets the current LSIF version of implementation.
	Version = "0.4.0"
	// LanguageID is the language ID in LSP, For Go it's "go".
	LanguageID = "go"
	// PositionEncoding is the encoding used to compute line and character values in positions and ranges.
	PositionEncoding = "utf-16"
)

// Element contains basic information of an element in the graph.
type Element struct {
	// The unique identifier of this element within the scope of project.
	ID int `json:"id"`
	// The kind of element in the graph.
	Type ElementType `json:"type"`
}

// ElementType represents the kind of element.
type ElementType string

const (
	ElementVertex ElementType = "vertex"
	ElementEdge   ElementType = "edge"
)

// Vertex contains information of a vertex in the graph.
type Vertex struct {
	Element
	// The kind of vertex in the graph.
	Label VertexLabel `json:"label"`
}

// VertexLabel represents the purpose of vertex.
type VertexLabel string

const (
	VertexMetaData             VertexLabel = "metaData"
	VertexEvent                VertexLabel = "$event"
	VertexProject              VertexLabel = "project"
	VertexRange                VertexLabel = "range"
	VertexLocation             VertexLabel = "location"
	VertexDocument             VertexLabel = "document"
	VertexMoniker              VertexLabel = "moniker"
	VertexPackageInformation   VertexLabel = "packageInformation"
	VertexResultSet            VertexLabel = "resultSet"
	VertexDocumentSymbolResult VertexLabel = "documentSymbolResult"
	VertexFoldingRangeResult   VertexLabel = "foldingRangeResult"
	VertexDocumentLinkResult   VertexLabel = "documentLinkResult"
	VertexDianosticResult      VertexLabel = "diagnosticResult"
	VertexDeclarationResult    VertexLabel = "declarationResult"
	VertexDefinitionResult     VertexLabel = "definitionResult"
	VertexTypeDefinitionResult VertexLabel = "typeDefinitionResult"
	VertexHoverResult          VertexLabel = "hoverResult"
	VertexReferenceResult      VertexLabel = "referenceResult"
	VertexImplementationResult VertexLabel = "implementationResult"
)

// ToolInfo contains information about the tool that created the dump.
type ToolInfo struct {
	// The name of the tool.
	Name string `json:"name"`
	// The version of the tool.
	Version string `json:"version,omitempty"`
	// The arguments passed to the tool.
	Args []string `json:"args,omitempty"`
}

// MetaData contains basic information about the dump.
type MetaData struct {
	Vertex
	// The version of the LSIF format using semver notation.
	Version string `json:"version"`
	// The project root (in form of an URI) used to compute this dump.
	ProjectRoot string `json:"projectRoot"`
	// The string encoding used to compute line and character values in
	// positions and ranges. Currently only 'utf-16' is support due to the
	// limitations in LSP.
	PositionEncoding string `json:"positionEncoding"`
	// The information about the tool that created the dump.
	ToolInfo ToolInfo `json:"toolInfo"`
}

// NewMetaData returns a new MetaData object with given ID, project root
// and tool information.
func NewMetaData(id int, root string, info ToolInfo) *MetaData {
	return &MetaData{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexMetaData,
		},
		Version:          Version,
		ProjectRoot:      root,
		PositionEncoding: PositionEncoding,
		ToolInfo:         info,
	}
}

// Project declares the language of the dump.
type Project struct {
	Vertex
	// The kind of language of the dump.
	Kind string `json:"kind"`
}

// NewProject returns a new Project object with given ID.
func NewProject(id int) *Project {
	return &Project{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexProject,
		},
		Kind: LanguageID,
	}
}

// Document is a vertex of document in the project.
type Document struct {
	Vertex
	// The URI indicates the location of the document.
	URI string `json:"uri"`
	// The language identifier of the document.
	LanguageID string `json:"languageId"`
	// The contents of the the document.
	Contents string `json:"contents,omitempty"`
}

// NewDocument returns a new Document object with given ID, URI and contents.
func NewDocument(id int, uri string, contents []byte) *Document {
	d := &Document{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexDocument,
		},
		URI:        uri,
		LanguageID: LanguageID,
	}

	if len(contents) > 0 {
		d.Contents = base64.StdEncoding.EncodeToString(contents)
	}

	return d
}

// ResultSet acts as a hub to be able to store information common to a set of ranges.
type ResultSet struct {
	Vertex
}

// NewResultSet returns a new ResultSet object with given ID.
func NewResultSet(id int) *ResultSet {
	return &ResultSet{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexResultSet,
		},
	}
}

// ReferenceResult acts as a hub to be able to store reference information common to a set of ranges.
type ReferenceResult struct {
	Vertex
}

// NewReferenceResult returns a new ReferenceResult object with given ID.
func NewReferenceResult(id int) *ResultSet {
	return &ResultSet{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexReferenceResult,
		},
	}
}

// Pos contains the precise position information.
type Pos struct {
	// The line number (0-based index)
	Line int `json:"line"`
	// The column of the character (0-based index)
	Character int `json:"character"`
}

// Range contains range information of a vertex object.
type Range struct {
	Vertex
	// The start position of the range.
	Start Pos `json:"start"`
	// The end position of the range.
	End Pos `json:"end"`
}

// NewRange returns a new Range object with given ID and position information.
func NewRange(id int, start, end Pos) *Range {
	return &Range{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexRange,
		},
		Start: start,
		End:   end,
	}
}

// DefinitionResult connects a definition that is spread over multiple ranges or multiple documents.
type DefinitionResult struct {
	Vertex
}

// NewDefinitionResult returns a new DefinitionResult object with given ID.
func NewDefinitionResult(id int) *DefinitionResult {
	return &DefinitionResult{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexDefinitionResult,
		},
	}
}

// MarkedString is the object to describe marked string.
type MarkedString markedString

type markedString struct {
	// The language of the marked string.
	Language string `json:"language"`
	// The value of the marked string.
	Value string `json:"value"`
	// Indicates whether to marshal JSON as raw string.
	isRawString bool
}

func (m *MarkedString) UnmarshalJSON(data []byte) error {
	if d := strings.TrimSpace(string(data)); len(d) > 0 && d[0] == '"' {
		// Raw string
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		m.Value = s
		m.isRawString = true
		return nil
	}
	// Language string
	ms := (*markedString)(m)
	return json.Unmarshal(data, ms)
}

func (m MarkedString) MarshalJSON() ([]byte, error) {
	if m.isRawString {
		return json.Marshal(m.Value)
	}
	return json.Marshal((markedString)(m))
}

// NewMarkedString returns a MarkedString with given string in language "go".
func NewMarkedString(s string) MarkedString {
	return MarkedString{
		Language: LanguageID,
		Value:    s,
	}
}

// RawMarkedString returns a MarkedString consisting of only a raw string
// (i.e., "foo" instead of {"value":"foo", "language":"bar"}).
func RawMarkedString(s string) MarkedString {
	return MarkedString{
		Value:       s,
		isRawString: true,
	}
}

type hoverResult struct {
	Contents []MarkedString `json:"contents"`
}

// HoverResult connects a hover that is spread over multiple ranges or multiple documents.
type HoverResult struct {
	Vertex
	// The result contents as the hover information.
	Result hoverResult `json:"result"`
}

// NewHoverResult returns a new HoverResult object with given ID, signature and extra contents.
func NewHoverResult(id int, contents []MarkedString) *HoverResult {
	return &HoverResult{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexHoverResult,
		},
		Result: hoverResult{
			Contents: contents,
		},
	}
}

// Edge contains information of an edge in the graph.
type Edge struct {
	Element
	// The kind of edge in the graph.
	Label EdgeLabel `json:"label"`
}

// EdgeLabel represents the purpose of an edge.
type EdgeLabel string

const (
	EdgeContains                   EdgeLabel = "contains"
	EdgeItem                       EdgeLabel = "item"
	EdgeNext                       EdgeLabel = "next"
	EdgeMoniker                    EdgeLabel = "moniker"
	EdgeNextMoniker                EdgeLabel = "nextMoniker"
	EdgePackageInformation         EdgeLabel = "packageInformation"
	EdgeTextDocumentDocumentSymbol EdgeLabel = "textDocument/documentSymbol"
	EdgeTextDocumentFoldingRange   EdgeLabel = "textDocument/foldingRange"
	EdgeTextDocumentDocumentLink   EdgeLabel = "textDocument/documentLink"
	EdgeTextDocumentDiagnostic     EdgeLabel = "textDocument/diagnostic"
	EdgeTextDocumentDefinition     EdgeLabel = "textDocument/definition"
	EdgeTextDocumentDeclaration    EdgeLabel = "textDocument/declaration"
	EdgeTextDocumentTypeDefinition EdgeLabel = "textDocument/typeDefinition"
	EdgeTextDocumentHover          EdgeLabel = "textDocument/hover"
	EdgeTextDocumentReferences     EdgeLabel = "textDocument/references"
	EdgeTextDocumentImplementation EdgeLabel = "textDocument/implementation"
)

// Next is an edge object that represents "next" relation.
type Next struct {
	Edge
	OutV int `json:"outV"`
	InV  int `json:"inV"`
}

// NewNext returns a new Next object with given ID and vertices information.
func NewNext(id int, outV, inV int) *Next {
	return &Next{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgeNext,
		},
		OutV: outV,
		InV:  inV,
	}
}

// Contains is an edge object that represents 1:n "contains" relation.
type Contains struct {
	Edge
	OutV int   `json:"outV"`
	InVs []int `json:"inVs"`
}

// NewContains returns a new Contains object with given ID and vertices information.
func NewContains(id int, outV int, inVs []int) *Contains {
	return &Contains{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgeContains,
		},
		OutV: outV,
		InVs: inVs,
	}
}

// TextDocumentDefinition is an edge object that represents "textDocument/definition" relation.
type TextDocumentDefinition struct {
	Edge
	OutV int `json:"outV"`
	InV  int `json:"inV"`
}

// NewTextDocumentDefinition returns a new TextDocumentDefinition object with given ID and
// vertices information.
func NewTextDocumentDefinition(id int, outV, inV int) *TextDocumentDefinition {
	return &TextDocumentDefinition{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgeTextDocumentDefinition,
		},
		OutV: outV,
		InV:  inV,
	}
}

// TextDocumentHover is an edge object that represents "textDocument/hover" relation.
type TextDocumentHover struct {
	Edge
	OutV int `json:"outV"`
	InV  int `json:"inV"`
}

// NewTextDocumentHover returns a new TextDocumentHover object with given ID and
// vertices information.
func NewTextDocumentHover(id int, outV, inV int) *TextDocumentHover {
	return &TextDocumentHover{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgeTextDocumentHover,
		},
		OutV: outV,
		InV:  inV,
	}
}

// TextDocumentReferences is an edge object that represents "textDocument/references" relation.
type TextDocumentReferences struct {
	Edge
	OutV int `json:"outV"`
	InV  int `json:"inV"`
}

// NewTextDocumentReferences returns a new TextDocumentReferences object with given ID and
// vertices information.
func NewTextDocumentReferences(id int, outV, inV int) *TextDocumentReferences {
	return &TextDocumentReferences{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgeTextDocumentReferences,
		},
		OutV: outV,
		InV:  inV,
	}
}

// Item is an edge object that represents "item" relation.
type Item struct {
	Edge
	OutV int   `json:"outV"`
	InVs []int `json:"inVs"`
	// The document the item belongs to.
	Document int `json:"document"`
	// The relationship property of the item.
	Property string `json:"property,omitempty"`
}

// NewItem returns a new Item object with given ID and vertices information.
func NewItem(id int, outV int, inVs []int, document int) *Item {
	return &Item{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgeItem,
		},
		OutV:     outV,
		InVs:     inVs,
		Document: document,
	}
}

// NewItemWithProperty returns a new Item object with given ID, vertices, document and
// property information.
func NewItemWithProperty(id int, outV int, inVs []int, document int, property string) *Item {
	i := NewItem(id, outV, inVs, document)
	i.Property = property
	return i
}

// NewItemOfDefinitions returns a new Item object with given ID, vertices and document
// informationand in "definitions" relationship.
func NewItemOfDefinitions(id int, outV int, inVs []int, document int) *Item {
	return NewItemWithProperty(id, outV, inVs, document, "definitions")
}

// NewItemOfReferences returns a new Item object with given ID, vertices and document
// informationand in "references" relationship.
func NewItemOfReferences(id int, outV int, inVs []int, document int) *Item {
	return NewItemWithProperty(id, outV, inVs, document, "references")
}
