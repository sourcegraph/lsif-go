package protocol

type Project struct {
	Vertex
	Kind string `json:"kind"`
}

func NewProject(id string, languageID string) *Project {
	return &Project{
		Vertex: Vertex{
			Element: Element{
				ID:   id,
				Type: ElementVertex,
			},
			Label: VertexProject,
		},
		Kind: languageID,
	}
}
