package protocol

type Contains struct {
	Edge
	OutV string   `json:"outV"`
	InVs []string `json:"inVs"`
}

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
