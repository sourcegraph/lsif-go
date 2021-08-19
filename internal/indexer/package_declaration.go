package indexer

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
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
	rangeID     uint64
	resultSetID uint64
}

func makePkgNameState(i *Indexer, p *packages.Package, pos token.Pos, name string, pkg *packages.Package) pkgNameState {
	position, document, _ := i.positionAndDocument(p, pos)
	obj := types.NewPkgName(pos, p.Types, name, pkg.Types)

	rangeID, _ := i.ensureRangeFor(position, obj)
	resultSetID := i.emitter.EmitResultSet()
	_ = i.emitter.EmitNext(rangeID, resultSetID)

	return pkgNameState{
		document,
		position,
		obj,
		rangeID,
		resultSetID,
	}
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
