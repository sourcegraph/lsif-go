package indexer

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

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

// ObjectLike is effectively just types.Object. We needed an interface that we could actually implement
// since types.Object has unexported fields, so it is unimplementable for our package.
type ObjectLike interface {
	Pos() token.Pos
	Pkg() *types.Package
	Name() string
	Type() types.Type
	Exported() bool
	Id() string

	String() string
}

// PkgDeclaration is similar to types.PkgName, except that instead of for _imported_ packages
// it is for _declared_ packages.
//
// Generated for: `package name`
type PkgDeclaration struct {
	pos  token.Pos
	pkg  *types.Package
	name string
}

func (p PkgDeclaration) Pos() token.Pos      { return p.pos }
func (p PkgDeclaration) Pkg() *types.Package { return p.pkg }
func (p PkgDeclaration) Name() string        { return p.name }
func (p PkgDeclaration) Type() types.Type    { return pkgDeclarationType{p} }
func (p PkgDeclaration) Exported() bool      { return true }
func (p PkgDeclaration) Id() string          { return "pkg:" + p.pkg.Name() + ":" + p.name }

// Fulfills types.Type interface
type pkgDeclarationType struct{ decl PkgDeclaration }

func (p pkgDeclarationType) Underlying() types.Type { return p }
func (p pkgDeclarationType) String() string         { return p.decl.Id() }
