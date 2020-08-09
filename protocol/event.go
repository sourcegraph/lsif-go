package protocol

type Event struct {
	Vertex
	Kind  string `json:"kind"`
	Scope string `json:"scope"`
	Data  string `json:"data"`
}

func NewEvent(id uint64, kind, scope, data string) *Event {
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
