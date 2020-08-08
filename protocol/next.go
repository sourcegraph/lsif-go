package protocol

type Next struct {
	Edge
	OutV string `json:"outV"`
	InV  string `json:"inV"`
}

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
