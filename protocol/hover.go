package protocol

import "encoding/json"

type HoverResult struct {
	Vertex
	Result hoverResult `json:"result"`
}

type hoverResult struct {
	Contents []MarkedString `json:"contents"`
}

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

type MarkedString markedString

type markedString struct {
	Language    string `json:"language"`
	Value       string `json:"value"`
	isRawString bool
}

func NewMarkedString(s, languageID string) MarkedString {
	return MarkedString{
		Language: languageID,
		Value:    s,
	}
}

func RawMarkedString(s string) MarkedString {
	return MarkedString{
		Value:       s,
		isRawString: true,
	}
}

func (m MarkedString) MarshalJSON() ([]byte, error) {
	if m.isRawString {
		return json.Marshal(m.Value)
	}
	return json.Marshal((markedString)(m))
}

type TextDocumentHover struct {
	Edge
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

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
