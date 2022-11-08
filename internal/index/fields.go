package index

import (
	"go/token"
	"sync"

	"golang.org/x/tools/go/packages"
)

func NewPackageFields(pkg *packages.Package) PackageFields {
	return PackageFields{
		pkg:    pkg,
		fields: map[token.Pos]string{},
	}
}

func NewProjectFields() ProjectFields {
	return ProjectFields{
		fields: map[string]*PackageFields{},
	}
}

type PackageFields struct {
	pkg    *packages.Package
	fields map[token.Pos]string
}

func (p *PackageFields) set(pos token.Pos, symbol string) { p.fields[pos] = symbol }
func (p *PackageFields) get(pos token.Pos) (string, bool) {
	field, ok := p.fields[pos]
	return field, ok
}

type ProjectFields struct {
	m      sync.Mutex
	fields map[string]*PackageFields
}

func (p *ProjectFields) add(pkgFields *PackageFields) {
	p.m.Lock()
	p.fields[pkgFields.pkg.PkgPath] = pkgFields
	p.m.Unlock()
}

func (p *ProjectFields) getPackage(pkg *packages.Package) *PackageFields {
	return p.fields[pkg.PkgPath]
}

func (p *ProjectFields) get(pkgPath string, pos token.Pos) (string, bool) {
	pkgFields, ok := p.fields[pkgPath]
	if !ok {
		return "", false
	}

	field, ok := pkgFields.get(pos)
	return field, ok
}
