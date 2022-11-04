package index

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type GoListOutput struct {
	Path       string
	Dir        string   // directory containing package sources
	ImportPath string   // import path of package in dir
	Name       string   // package name
	Doc        string   // package documentation string
	Standard   bool     // is this package part of the standard Go library?
	Root       string   // Go root or Go path dir containing this package
	Match      []string // command-line patterns matching this package
	DepOnly    bool     // package is only a dependency, not explicitly listed

	// Source files
	GoFiles     []string // .go source files (excluding CgoFiles, TestGoFiles, XTestGoFiles)
	CgoFiles    []string // .go source files that import "C"
	TestGoFiles []string // _test.go files in package

	// Dependency information
	Imports   []string          // import paths used by this package
	ImportMap map[string]string // map from source import to ImportPath (identity entries omitted)
	Deps      []string          // all (recursively) imported dependencies
	Module    packages.Module
	// TestImports  []string          // imports from TestGoFiles
	// XTestImports []string          // imports from XTestGoFiles
}

type PackageInfo struct {
	Name          string
	Path          string
	Dir           string // directory containing package sources
	ImportPath    string // import path of package in dir
	ImportedPaths []string

	GoFiles     []string // .go source files (excluding CgoFiles, TestGoFiles, XTestGoFiles)
	TestGoFiles []string // _test.go files in package

	Fset      *token.FileSet
	Imports   map[string]*PackageInfo
	Module    *packages.Module
	Syntax    []*ast.File
	Types     *types.Package
	TypesInfo *types.Info

	// TODO: Remove
	// originalPackage *packages.Package
}

func NewPackageInfo(raw *GoListOutput, goPkg *packages.Package) PackageInfo {
	pkg := PackageInfo{
		Imports: map[string]*PackageInfo{},
	}

	pkg.updateFromGoList(raw)
	pkg.updateFromPackage(goPkg)

	return pkg
}

func (pkg *PackageInfo) updateFromGoList(raw *GoListOutput) {
	if raw == nil {
		return
	}

	pkg.Path = raw.Path
	pkg.Name = raw.Name
	pkg.Dir = raw.Dir
	pkg.ImportPath = raw.ImportPath
	pkg.ImportedPaths = raw.Imports

	pkg.GoFiles = raw.GoFiles
	pkg.TestGoFiles = raw.TestGoFiles
}

func (pkg *PackageInfo) updateFromPackage(goPkg *packages.Package) {
	if goPkg == nil {
		return
	}

	// pkg.originalPackage = goPkg

	// TODO: Transform the imports to the new style
	// pkg.Imports = goPkg.Imports

	pkg.Fset = goPkg.Fset
	pkg.Module = goPkg.Module
	pkg.Syntax = goPkg.Syntax
	pkg.Types = goPkg.Types
	pkg.TypesInfo = goPkg.TypesInfo
}

// ObjectLike is effectively just types.Object. We needed an interface that we could actually implement
// since types.Object has unexported fields, so it is unimplementable for our package.
type ObjectLike interface {
	Pos() token.Pos
	Pkg() *types.Package
	Name() string
	Type() types.Type

	String() string
}

// PkgDeclaration is similar to types.PkgName, except that instead of for _imported_ packages
// it is for _declared_ packages.
//
// Generated for: `package name`
//
// For more information, see : docs/package_declarations.md
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

// func newPkgDeclaration(p *PackageInfo, f *ast.File) (*PkgDeclaration, token.Position) {
// 	// import mypackage
// 	// ^--------------------- pkgKeywordPosition *types.Position
// 	//        ^-------------- pkgDeclarationPos  *types.Pos
// 	//        ^-------------- pkgPosition        *types.Position
// 	pkgKeywordPosition := p.Fset.Position(f.Package)
//
// 	pkgDeclarationPos := p.Fset.File(f.Package).Pos(pkgKeywordPosition.Offset + packageLen)
// 	pkgPosition := p.Fset.Position(pkgDeclarationPos)
//
// 	name := f.Name.Name
//
// 	return &PkgDeclaration{
// 		pos:  pkgDeclarationPos,
// 		pkg:  types.NewPackage(p.PkgPath, name),
// 		name: name,
// 	}, pkgPosition
// }
