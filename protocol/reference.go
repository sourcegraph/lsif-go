package protocol

type ReferenceResult struct {
	Vertex
}

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

type TextDocumentReferences struct {
	Edge
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

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
