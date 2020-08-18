package indexer

// Stats summarizes the amount of work done by the indexer.
type Stats struct {
	NumPkgs     uint
	NumFiles    uint
	NumDefs     uint
	NumElements uint64
}

// DocumentInfo provides context for constructing the contains relationship between
// a document and the ranges that it contains.
type DocumentInfo struct {
	DocumentID         uint64
	DefinitionRangeIDs []uint64
	ReferenceRangeIDs  []uint64
}

// DefinitionInfo provides context about a range that defines an identifier. An object
// of this shape is keyed by type and identifier in the indexer so that it can be
// re-retrieved for a range that uses the definition.
type DefinitionInfo struct {
	DocumentID  uint64
	RangeID     uint64
	ResultSetID uint64
}

// ReferenceResultInfo provides context about a definition range. Each definition and
// reference range will be added to an object of this shape as it is processed.
type ReferenceResultInfo struct {
	ResultSetID        uint64
	DefinitionRangeIDs map[uint64][]uint64
	ReferenceRangeIDs  map[uint64][]uint64
}
