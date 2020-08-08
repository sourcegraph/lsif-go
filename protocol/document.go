package protocol

import "encoding/base64"

type Document struct {
	Vertex
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Contents   string `json:"contents,omitempty"`
}

func NewDocument(id, languageID, uri string, contents []byte) *Document {
	d := &Document{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexDocument,
		},
		URI:        uri,
		LanguageID: languageID,
	}

	if len(contents) > 0 {
		d.Contents = base64.StdEncoding.EncodeToString(contents)
	}

	return d
}
