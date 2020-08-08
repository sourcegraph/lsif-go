package protocol

type Item struct {
	Edge
	OutV     string   `json:"outV"`
	InVs     []string `json:"inVs"`
	Document string   `json:"document"`
	Property string   `json:"property,omitempty"`
}

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

func NewItemWithProperty(id, outV string, inVs []string, document, property string) *Item {
	i := NewItem(id, outV, inVs, document)
	i.Property = property
	return i
}

func NewItemOfDefinitions(id, outV string, inVs []string, document string) *Item {
	return NewItemWithProperty(id, outV, inVs, document, "definitions")
}

// informationand in "references" relationship.
func NewItemOfReferences(id, outV string, inVs []string, document string) *Item {
	return NewItemWithProperty(id, outV, inVs, document, "references")
}
