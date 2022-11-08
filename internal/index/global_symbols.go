package index

import (
	"go/token"
	"sync"

	"golang.org/x/tools/go/packages"
)

func NewPackageSymbols(pkg *packages.Package) *PackageSymbols {
	return &PackageSymbols{
		pkg:    pkg,
		fields: map[token.Pos]string{},
	}
}

func NewGlobalSymbols() GlobalSymbols {
	return GlobalSymbols{
		symbols: map[string]*PackageSymbols{},
	}
}

type PackageSymbols struct {
	pkg    *packages.Package
	fields map[token.Pos]string
}

func (p *PackageSymbols) set(pos token.Pos, symbol string) {
	p.fields[pos] = symbol
}

func (p *PackageSymbols) get(pos token.Pos) (string, bool) {
	field, ok := p.fields[pos]
	return field, ok
}

type GlobalSymbols struct {
	m       sync.Mutex
	symbols map[string]*PackageSymbols
}

func (p *GlobalSymbols) add(pkgSymbols *PackageSymbols) {
	p.m.Lock()
	p.symbols[pkgSymbols.pkg.PkgPath] = pkgSymbols
	p.m.Unlock()
}

func (p *GlobalSymbols) getPackage(pkg *packages.Package) *PackageSymbols {
	return p.symbols[pkg.PkgPath]
}

func (p *GlobalSymbols) get(pkgPath string, pos token.Pos) (string, bool) {
	pkgFields, ok := p.symbols[pkgPath]
	if !ok {
		return "", false
	}

	field, ok := pkgFields.get(pos)
	return field, ok
}
