package indexer

import (
	"go/token"
	"go/types"
)

// TODO: Handle testing packages differently.

type ObjectLike interface {
	Pos() token.Pos
	Pkg() *types.Package
	Name() string
	Type() types.Type
	Exported() bool
	Id() string

	String() string
}

type pkgNameState struct {
	document    *DocumentInfo
	position    token.Position
	obj         *types.PkgName
	name        string
	rangeID     uint64
	resultSetID uint64
}

type PkgDeclaration struct {
	pos  token.Pos
	pkg  *types.Package
	name string
}

func (p PkgDeclaration) Pos() token.Pos      { return p.pos }
func (p PkgDeclaration) Pkg() *types.Package { return p.pkg }
func (p PkgDeclaration) Name() string        { return p.name }
func (p PkgDeclaration) Type() types.Type    { return p }
func (PkgDeclaration) Exported() bool        { return true }
func (p PkgDeclaration) Id() string          { return "pkg:TODO" }

// Type interface
func (p PkgDeclaration) Underlying() types.Type { return p }
func (p PkgDeclaration) String() string         { return "Package Declaration" }
