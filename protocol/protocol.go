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
	ID string `json:"id"`
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
func NewMetaData(id, root string, info ToolInfo) *MetaData {
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

// Event is optional metadata emitted to give hints to consumers about
// the beginning and ending of new "socpes" (e.g. a project or document).
type Event struct {
	Vertex
	// The kind of event (begin or end).
	Kind string `json:"kind"`
	// The type of element this event describes (project or document).
	Scope string `json:"scope"`
	// The identifier of the data beginning or ending.
	Data string `json:"data"`
}

// NewEvent returns a new Event object with the given ID, kind, scope,
// and data information.
func NewEvent(id, kind, scope, data string) *Event {
	return &Event{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexEvent,
		},
		Kind:  kind,
		Scope: scope,
		Data:  data,
	}
}

// Project declares the language of the dump.
type Project struct {
	Vertex
	// The kind of language of the dump.
	Kind string `json:"kind"`
}

// NewProject returns a new Project object with given ID.
func NewProject(id string) *Project {
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
func NewDocument(id, uri string, contents []byte) *Document {
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
func NewResultSet(id string) *ResultSet {
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
func NewReferenceResult(id string) *ResultSet {
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
func NewRange(id string, start, end Pos) *Range {
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
func NewDefinitionResult(id string) *DefinitionResult {
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
func NewHoverResult(id string, contents []MarkedString) *HoverResult {
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

// Moniker describes a unique name for a result set or range.
type Moniker struct {
	Vertex
	// The kind of moniker (e.g. local, export, import).
	Kind string `json:"kind"`
	// The kind of moniker, usually a language or package manager.
	Scheme string `json:"scheme"`
	// The unique moniker identifier.
	Identifier string `json:"identifier"`
}

// NewMoniker returns a new Moniker wtih the given ID, kind, scheme, and identifier.
func NewMoniker(id, kind, scheme, identifier string) *Moniker {
	return &Moniker{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexMoniker,
		},
		Kind:       kind,
		Scheme:     scheme,
		Identifier: identifier,
	}
}

// PackageInformation describes a package for a moniker.
type PackageInformation struct {
	Vertex
	// The name of the package.
	Name string `json:"name"`
	// The package manager.
	Manager string `json:"manager"`
	// The version of the package.
	Version string `json:"version"`
}

// NewPackageInformation returns a new PackageInformation with the given ID, name, manager, and version.
func NewPackageInformation(id, name, manager, version string) *PackageInformation {
	return &PackageInformation{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexPackageInformation,
		},
		Name:    name,
		Manager: manager,
		Version: version,
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
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

// NewNext returns a new Next object with given ID and vertices information.
func NewNext(id, outV, inV string) *Next {
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
	OutV string   `json:"outV"`
	InVs []string `json:"inVs"`
}

// NewContains returns a new Contains object with given ID and vertices information.
func NewContains(id, outV string, inVs []string) *Contains {
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
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

// NewTextDocumentDefinition returns a new TextDocumentDefinition object with given ID and
// vertices information.
func NewTextDocumentDefinition(id, outV, inV string) *TextDocumentDefinition {
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
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

// NewTextDocumentHover returns a new TextDocumentHover object with given ID and
// vertices information.
func NewTextDocumentHover(id, outV, inV string) *TextDocumentHover {
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
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

// NewTextDocumentReferences returns a new TextDocumentReferences object with given ID and
// vertices information.
func NewTextDocumentReferences(id, outV, inV string) *TextDocumentReferences {
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
	OutV string   `json:"outV"`
	InVs []string `json:"inVs"`
	// The document the item belongs to.
	Document string `json:"document"`
	// The relationship property of the item.
	Property string `json:"property,omitempty"`
}

// NewItem returns a new Item object with given ID and vertices information.
func NewItem(id, outV string, inVs []string, document string) *Item {
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
func NewItemWithProperty(id, outV string, inVs []string, document, property string) *Item {
	i := NewItem(id, outV, inVs, document)
	i.Property = property
	return i
}

// NewItemOfDefinitions returns a new Item object with given ID, vertices and document
// informationand in "definitions" relationship.
func NewItemOfDefinitions(id, outV string, inVs []string, document string) *Item {
	return NewItemWithProperty(id, outV, inVs, document, "definitions")
}

// NewItemOfReferences returns a new Item object with given ID, vertices and document
// informationand in "references" relationship.
func NewItemOfReferences(id, outV string, inVs []string, document string) *Item {
	return NewItemWithProperty(id, outV, inVs, document, "references")
}

// MonikerEdge connects a moniker to a range or result set.
type MonikerEdge struct {
	Edge
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

// NewMonikerEdge returns a new MonikerEdge with the given ID and vertices.
func NewMonikerEdge(id, outV, inV string) *MonikerEdge {
	return &MonikerEdge{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgeMoniker,
		},
		OutV: outV,
		InV:  inV,
	}
}

// NextMonikerEdge connects a moniker to another moniker.
type NextMonikerEdge struct {
	Edge
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

// NewNextMonikerEdge returns a new NextMonikerEdge with the given ID and vertices.
func NewNextMonikerEdge(id, outV, inV string) *NextMonikerEdge {
	return &NextMonikerEdge{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgeNextMoniker,
		},
		OutV: outV,
		InV:  inV,
	}
}

// PackageInformationEdge connects a moniker and a package information vertex.
type PackageInformationEdge struct {
	Edge
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

// NewPackageInformationEdge returns a new PackageInformationEdge with the given ID and vertices.
func NewPackageInformationEdge(id, outV, inV string) *PackageInformationEdge {
	return &PackageInformationEdge{
		Edge: Edge{
			Element: Element{
				ID:   id,
				Type: ElementEdge,
			},
			Label: EdgePackageInformation,
		},
		OutV: outV,
		InV:  inV,
	}
}
