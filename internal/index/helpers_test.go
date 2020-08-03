package index

import (
	"strings"

	"github.com/sourcegraph/lsif-go/protocol"
)

// stripIndent removes leading indentation from each line of the given text and joins the
// resulting non-empty lines with a single space.
func stripIndent(s string) string {
	var parts []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts = append(parts, line)
	}

	return strings.Join(parts, " ")
}

// capturingWriter can be used in place of the protocol.JSONWriter used in the binary. This
// captures each of the emitted objects without serializing them so they can be inspected via
// type by the unit tests of this package.
type capturingWriter struct {
	elements []interface{}
}

func (w *capturingWriter) Write(v interface{}) error {
	w.elements = append(w.elements, v)
	return nil
}

// findHoverResultByID returns the hover result object with the given identifier.
func findHoverResultByID(elements []interface{}, id string) *protocol.HoverResult {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.HoverResult:
			if v.ID == id {
				return v
			}
		}
	}

	return nil
}

// findHoverResultByRangeOrResultSetID returns the hover result attached to the range or result
// set with the given identifier.
func findHoverResultByRangeOrResultSetID(elements []interface{}, id string) *protocol.HoverResult {
	// First see if we're attached to a hover result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.TextDocumentHover:
			if e.OutV == id {
				return findHoverResultByID(elements, e.InV)
			}
		}
	}

	// Try to get the hover result of the result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Next:
			if e.OutV == id {
				if result := findHoverResultByRangeOrResultSetID(elements, e.InV); result != nil {
					return result
				}
			}
		}
	}

	return nil
}

// findRange returns the range in the given file with the given start line and character.
func findRange(elements []interface{}, filename string, startLine, startCharacter int) *protocol.Range {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.Range:
			if v.Start.Line == startLine && v.Start.Character == startCharacter {
				if findDocumentURIContaining(elements, v.ID) == filename {
					return v
				}
			}
		}
	}

	return nil
}

// findDocumentURIByDocumentID returns the URI of the document with the given ID.
func findDocumentURIByDocumentID(elements []interface{}, id string) string {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.Document:
			if v.ID == id {
				return v.URI
			}
		}
	}

	return ""
}

// findDocumentURIContaining finds the URI of the document containing the given ID.
func findDocumentURIContaining(elements []interface{}, id string) string {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Contains:
			for _, inV := range e.InVs {
				if inV == id {
					return findDocumentURIByDocumentID(elements, e.OutV)
				}
			}
		}
	}

	return ""
}

func findRangeByID(elements []interface{}, id string) *protocol.Range {
	for _, elem := range elements {
		switch v := elem.(type) {
		case *protocol.Range:
			if v.ID == id {
				return v
			}
		}
	}

	return nil
}

func findDefintionRangesByDefinitionResultID(elements []interface{}, id string) (ranges []*protocol.Range) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Item:
			if e.OutV == id {
				for _, inV := range e.InVs {
					if r := findRangeByID(elements, inV); r != nil {
						ranges = append(ranges, r)
					}
				}
			}
		}
	}

	return ranges
}

func findDefinitionRangesByRangeOrResultSetID(elements []interface{}, id string) (ranges []*protocol.Range) {
	// First see if we're attached to definition result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.TextDocumentDefinition:
			if e.OutV == id {
				ranges = append(ranges, findDefintionRangesByDefinitionResultID(elements, e.InV)...)
			}
		}
	}

	// Try to get the definition result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findDefinitionRangesByRangeOrResultSetID(elements, e.InV)...)
			}
		}
	}

	return ranges
}

func findReferenceRangesByReferenceResultID(elements []interface{}, id string) (ranges []*protocol.Range) {
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Item:
			if e.OutV == id {
				for _, inV := range e.InVs {
					if r := findRangeByID(elements, inV); r != nil {
						ranges = append(ranges, r)
					}
				}
			}
		}
	}

	return ranges
}

func findReferenceRangesByRangeOrResultSetID(elements []interface{}, id string) (ranges []*protocol.Range) {
	// First see if we're attached to reference result directly
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.TextDocumentReferences:
			if e.OutV == id {
				ranges = append(ranges, findReferenceRangesByReferenceResultID(elements, e.InV)...)
			}
		}
	}

	// Try to get the reference result of a result set attached to the given range or result set
	for _, elem := range elements {
		switch e := elem.(type) {
		case *protocol.Next:
			if e.OutV == id {
				ranges = append(ranges, findReferenceRangesByRangeOrResultSetID(elements, e.InV)...)
			}
		}
	}

	return ranges
}
