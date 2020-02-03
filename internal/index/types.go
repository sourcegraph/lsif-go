package index

// fileInfo contains LSIF information of a file.
type fileInfo struct {
	// The vertex ID of the document that represents the file.
	docID string
	// The vertices ID of ranges that represents the definition.
	// This information is collected to emit "contains" edge.
	defRangeIDs []string
	// The vertices ID of ranges that represents the definition use cases.
	// This information is collected to emit "contains" edge.
	useRangeIDs []string
}

// defInfo contains LSIF information of a definition.
type defInfo struct {
	// The identifier of the containing document. This is necessary
	// to track when emitting item edges as we need to store the
	// document to which it belongs (not where it is referenced).
	docID string
	// The vertex ID of the range that represents the definition.
	rangeID string
	// The vertex ID of the resultSet that represents the definition.
	resultSetID string
	// The lazily initialized definition result ID upon first use found.
	defResultID string
}

// refResultInfo contains LSIF information of a reference result.
type refResultInfo struct {
	// The vertex ID of the resultSet that represents the referenceResult.
	resultSetID string
	// The vertices ID of definition ranges that are referenced by the referenceResult.
	// This is a map from the document ID to the set of range IDs contained within it.
	// This information is collected to emit `{"label":"item", "property":"definitions"}` edge.
	defRangeIDs map[string][]string
	// The vertices ID of reference ranges that are represented by the referenceResult.
	// This is a map from the document ID to the set of range IDs contained within it.
	// This information is collected to emit `{"label":"item", "property":"references"}` edge.
	refRangeIDs map[string][]string
}
