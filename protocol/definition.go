package protocol

type DefinitionResult struct {
	Vertex
}

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

type TextDocumentDefinition struct {
	Edge
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

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
