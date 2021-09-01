package indexer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type importReference struct {
	rangeID    uint64
	documentID uint64
}

type importState struct {
	document    *DocumentInfo
	position    token.Position
	obj         *types.PkgName
	rangeID     uint64
	resultSetID uint64
}

func makeImportState(i *Indexer, p *packages.Package, pos token.Pos, name string, pkg *packages.Package) importState {
	position, document, _ := i.positionAndDocument(p, pos)
	obj := types.NewPkgName(pos, p.Types, name, pkg.Types)

	rangeID, _ := i.ensureRangeFor(position, obj)
	resultSetID := i.emitter.EmitResultSet()
	_ = i.emitter.EmitNext(rangeID, resultSetID)

	return importState{
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
//
// There was no types.PkgDeclaration available
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
func (p PkgDeclaration) String() string      { return "pkg:" + p.pkg.Name() + ":" + p.name }

// Fulfills types.Type interface
type pkgDeclarationType struct{ decl PkgDeclaration }

func (p pkgDeclarationType) Underlying() types.Type { return p }
func (p pkgDeclarationType) String() string         { return p.decl.Id() }

var packageLen = len("package ")

func newPkgDeclaration(p *packages.Package, f *ast.File) (*PkgDeclaration, token.Position) {
	// import mypackage
	// ^--------------------- pkgKeywordPosition *types.Position
	//        ^-------------- pkgDeclarationPos  *types.Pos
	//        ^-------------- pkgPosition        *types.Position
	pkgKeywordPosition := p.Fset.Position(f.Package)

	pkgDeclarationPos := p.Fset.File(f.Package).Pos(pkgKeywordPosition.Offset + packageLen)
	pkgPosition := p.Fset.Position(pkgDeclarationPos)

	name := f.Name.Name

	return &PkgDeclaration{
		pos:  pkgDeclarationPos,
		pkg:  types.NewPackage(p.PkgPath, name),
		name: name,
	}, pkgPosition
}
