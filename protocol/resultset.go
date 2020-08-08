package protocol

type ResultSet struct {
	Vertex
}

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
