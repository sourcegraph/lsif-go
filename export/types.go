package export

import (
	"github.com/sourcegraph/lsif-go/protocol"
)

// fileInfo contains LSIF information of a file.
type fileInfo struct {
	// The vertex ID of the document that represents the file.
	docID int
	// The vertices ID of ranges that represents the definition.
	// This information is collected to emit "contains" edge.
	defRangeIDs []int
	// The vertices ID of ranges that represents the definition use cases.
	// This information is collected to emit "contains" edge.
	useRangeIDs []int
}

// defInfo contains LSIF information of a definition.
type defInfo struct {
	// The vertex ID of the range that represents the definition.
	rangeID int
	// The vertex ID of the resultSet that represents the definition.
	resultSetID int
	// The contents will be used as the hover information.
	contents []protocol.MarkedString
}

// refResultInfo contains LSIF information of a reference result.
type refResultInfo struct {
	// The vertex ID of the resultSet that represents the referenceResult.
	resultSetID int
	// The vertices ID of definition ranges that are referenced by the referenceResult.
	// This information is collected to emit `{"label":"item", "property":"definitions"}` edge.
	defRangeIDs []int
	// The vertices ID of reference ranges that are represented by the referenceResult.
	// This information is collected to emit `{"label":"item", "property":"references"}` edge.
	refRangeIDs []int
}
