package indexer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// Stats summarizes the amount of work done by the indexer.
type Stats struct {
	NumPkgs     int
	NumFiles    int
	NumDefs     int
	NumElements int
}

// FileInfo provides context about a particular file.
type FileInfo struct {
	Package  *packages.Package // the containing
	File     *ast.File         // the parsed AST
	Document *DocumentInfo     // document context
	Filename string            // name of the file
}

// DocumentInfo provides context for constructing the contains relationship between
// a document and the ranges that it contains.
type DocumentInfo struct {
	DocumentID         string
	DefinitionRangeIDs []string
	ReferenceRangeIDs  []string
}

// ObjectInfo provides context about a particular object within a file.
type ObjectInfo struct {
	FileInfo                // the containing file
	Position token.Position // the offset within the file
	Object   types.Object   // the object
	Ident    *ast.Ident     // the identifier
}

// DefinitionInfo provides context about a range that defines an identifier. An object
// of this shape is keyed by type and identifier in the indexer so that it can be
// re-retrieved for a range that uses the definition.
type DefinitionInfo struct {
	DocumentID  string
	RangeID     string
	ResultSetID string
}

// ReferenceResultInfo provides context about a definition range. Each definition and
// reference range will be added to an object of this shape as it is processed.
type ReferenceResultInfo struct {
	ResultSetID        string
	DefinitionRangeIDs map[string][]string
	ReferenceRangeIDs  map[string][]string
}
