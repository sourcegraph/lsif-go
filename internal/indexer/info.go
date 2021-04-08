package indexer

import "sync"

// IndexerStats summarizes the amount of work done by the indexer.
type IndexerStats struct {
	NumPkgs     uint
	NumFiles    uint
	NumDefs     uint
	NumElements uint64
}

// PackageDataCacheStats summarizes the amount of work done by the package data cache.
type PackageDataCacheStats struct {
	NumPks uint
}

// DocumentInfo provides context for constructing the contains relationship between
// a document and the ranges that it contains.
type DocumentInfo struct {
	DocumentID         uint64
	DefinitionRangeIDs []uint64
	ReferenceRangeIDs  []uint64
	SymbolRangeIDs     []uint64
	m                  sync.Mutex
}

// DefinitionInfo provides context about a range that defines an identifier. An object
// of this shape is keyed by type and identifier in the indexer so that it can be
// re-retrieved for a range that uses the definition.
type DefinitionInfo struct {
	DocumentID        uint64
	RangeID           uint64
	ResultSetID       uint64
	ReferenceRangeIDs map[uint64][]uint64
	TypeSwitchHeader  bool
	m                 sync.Mutex
}
