// Package index is used to generate an LSIF dump for a workspace.
package index

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/packages"
)

const LanguageGo = "go"

// Indexer reads source files and outputs LSIF data.
type Indexer interface {
	Index() (*Stats, error)
}

// Stats contains statistics of data processed during index.
type Stats struct {
	NumPkgs     int
	NumFiles    int
	NumDefs     int
	NumElements int
}

// indexer keeps track of all information needed to generate an LSIF dump.
type indexer struct {
	projectRoot    string
	repositoryRoot string
	toolInfo       protocol.ToolInfo
	w              *protocol.Writer
	hoverLoader    *HoverLoader

	// De-duplication
	packagesImported map[string]bool
	defsIndexed      map[string]bool
	usesIndexed      map[string]bool
	ranges           map[string]map[int]string // filename -> offset -> rangeID
	hoverResultCache map[string]string

	// Type correlation
	files   map[string]*fileInfo      // Keys: filename
	imports map[token.Pos]*defInfo    // Keys: definition position
	funcs   map[string]*defInfo       // Keys: full name (with receiver for methods)
	consts  map[token.Pos]*defInfo    // Keys: definition position
	vars    map[token.Pos]*defInfo    // Keys: definition position
	types   map[string]*defInfo       // Keys: type name
	labels  map[token.Pos]*defInfo    // Keys: definition position
	refs    map[string]*refResultInfo // Keys: definition range ID

	// Monikers
	moduleName            string
	moduleVersion         string
	dependencies          map[string]string
	packageInformationIDs map[string]string
}

// NewIndexer creates a new Indexer.
func NewIndexer(
	projectRoot string,
	repositoryRoot string,
	moduleName string,
	moduleVersion string,
	dependencies map[string]string,
	addContents bool,
	toolInfo protocol.ToolInfo,
	w io.Writer,
) Indexer {
	return &indexer{
		projectRoot:    projectRoot,
		repositoryRoot: repositoryRoot,
		moduleName:     moduleName,
		moduleVersion:  moduleVersion,
		dependencies:   dependencies,
		toolInfo:       toolInfo,
		w:              protocol.NewWriter(w, addContents),
		hoverLoader:    newHoverLoader(),

		// Empty maps
		packagesImported:      map[string]bool{},
		defsIndexed:           map[string]bool{},
		usesIndexed:           map[string]bool{},
		ranges:                map[string]map[int]string{},
		hoverResultCache:      map[string]string{},
		files:                 map[string]*fileInfo{},
		imports:               map[token.Pos]*defInfo{},
		funcs:                 map[string]*defInfo{},
		consts:                map[token.Pos]*defInfo{},
		vars:                  map[token.Pos]*defInfo{},
		types:                 map[string]*defInfo{},
		labels:                map[token.Pos]*defInfo{},
		refs:                  map[string]*refResultInfo{},
		packageInformationIDs: map[string]string{},
	}
}

// Index generates an LSIF dump from a workspace by traversing through source files
// and writing the LSIF equivalent to the output source that implements io.Writer.
// It is caller's responsibility to close the output source if applicable.
func (i *indexer) Index() (*Stats, error) {
	fmt.Fprintf(os.Stdout, "Loading packages\n")
	pkgs, err := i.packages()
	if err != nil {
		return nil, err
	}

	return i.index(pkgs)
}

func (i *indexer) packages() ([]*packages.Package, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles |
			packages.NeedImports | packages.NeedDeps |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:   i.projectRoot,
		Tests: true,
	}, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %v", err)
	}

	return pkgs, nil
}

func (i *indexer) index(pkgs []*packages.Package) (*Stats, error) {
	_, err := i.w.EmitMetaData("file://"+i.repositoryRoot, i.toolInfo)
	if err != nil {
		return nil, fmt.Errorf(`emit "metadata": %v`, err)
	}
	proID, err := i.w.EmitProject(LanguageGo)
	if err != nil {
		return nil, fmt.Errorf(`emit "project": %v`, err)
	}

	_, err = i.w.EmitBeginEvent("project", proID)
	if err != nil {
		return nil, fmt.Errorf(`emit "begin": %v`, err)
	}

	fmt.Fprintf(os.Stdout, "Preparing documents\n")
	for _, p := range pkgs {
		fmt.Fprintf(os.Stdout, ".")

		if err := i.indexPkgDocs(p, proID); err != nil {
			return nil, fmt.Errorf("index package %q: %v", p.Name, err)
		}

		if err := i.addPkgImports(pkgs, p, proID); err != nil {
			return nil, fmt.Errorf("add package imports %q: %v", p.Name, err)
		}
	}
	fmt.Fprintf(os.Stdout, "\n")

	allPackages := map[*packages.Package]struct{}{}
	for _, p := range pkgs {
		allPackages[p] = struct{}{}

		for _, i := range p.Imports {
			allPackages[i] = struct{}{}
		}
	}

	n := runtime.GOMAXPROCS(0)
	sema := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		sema <- struct{}{}
	}

	var wg sync.WaitGroup

	fmt.Fprintf(os.Stdout, "Preloading hover text\n")
	for p := range allPackages {
		wg.Add(1)

		go func(p *packages.Package) {
			defer wg.Done()
			<-sema
			defer func() { sema <- struct{}{} }()

			for _, f := range p.Syntax {
				positions := make([]token.Pos, 0, len(p.TypesInfo.Defs))
				for _, obj := range p.TypesInfo.Defs {
					if obj != nil {
						positions = append(positions, obj.Pos())
					}
				}

				i.hoverLoader.Load(f, positions)
			}

			fmt.Fprintf(os.Stdout, ".")
		}(p)
	}
	wg.Wait()
	fmt.Fprintf(os.Stdout, "\n")

	fmt.Fprintf(os.Stdout, "Indexing definitions\n")
	for _, p := range pkgs {
		fmt.Fprintf(os.Stdout, ".")

		if err := i.indexPkgDefs(pkgs, p, proID); err != nil {
			return nil, fmt.Errorf("index package defs %q: %v", p.Name, err)
		}
	}
	fmt.Fprintf(os.Stdout, "\n")

	fmt.Fprintf(os.Stdout, "Indexing uses\n")
	for _, p := range pkgs {
		fmt.Fprintf(os.Stdout, ".")

		if err := i.indexPkgUses(pkgs, p, proID); err != nil {
			return nil, fmt.Errorf("index package uses %q: %v", p.Name, err)
		}
	}
	fmt.Fprintf(os.Stdout, "\n")

	fmt.Fprintf(os.Stdout, "Finalizing reference results\n")
	for _, f := range i.files {
		fmt.Fprintf(os.Stdout, ".")

		for _, rangeID := range f.defRangeIDs {
			refResultID, err := i.w.EmitReferenceResult()
			if err != nil {
				return nil, fmt.Errorf(`emit "referenceResult": %v`, err)
			}

			_, err = i.w.EmitTextDocumentReferences(i.refs[rangeID].resultSetID, refResultID)
			if err != nil {
				return nil, fmt.Errorf(`emit "textDocument/references": %v`, err)
			}

			for docID, rangeIDs := range i.refs[rangeID].defRangeIDs {
				_, err = i.w.EmitItemOfDefinitions(refResultID, rangeIDs, docID)
				if err != nil {
					return nil, fmt.Errorf(`emit "item": %v`, err)
				}
			}

			for docID, rangeIDs := range i.refs[rangeID].refRangeIDs {
				_, err = i.w.EmitItemOfReferences(refResultID, rangeIDs, docID)
				if err != nil {
					return nil, fmt.Errorf(`emit "item": %v`, err)
				}
			}
		}

		if len(f.defRangeIDs) > 0 || len(f.useRangeIDs) > 0 {
			// Deduplicate ranges before emitting a contains edge
			union := map[string]bool{}
			for _, id := range f.defRangeIDs {
				union[id] = true
			}
			for _, id := range f.useRangeIDs {
				union[id] = true
			}
			allRanges := []string{}
			for id := range union {
				allRanges = append(allRanges, id)
			}

			_, err = i.w.EmitContains(f.docID, allRanges)
			if err != nil {
				return nil, fmt.Errorf(`emit "contains": %v`, err)
			}
		}
	}
	fmt.Fprintf(os.Stdout, "\n")

	// Close all documents. This must be done as a last step as we need
	// to emit everything about a document before sending the end event.

	for _, info := range i.files {
		_, err = i.w.EmitEndEvent("document", info.docID)
		if err != nil {
			return nil, fmt.Errorf(`emit "end": %v`, err)
		}
	}

	_, err = i.w.EmitEndEvent("project", proID)
	if err != nil {
		return nil, fmt.Errorf(`emit "end": %v`, err)
	}

	return &Stats{
		NumPkgs:     len(pkgs),
		NumFiles:    len(i.files),
		NumDefs:     len(i.imports) + len(i.funcs) + len(i.consts) + len(i.vars) + len(i.types) + len(i.labels),
		NumElements: i.w.NumElements(),
	}, nil
}

func (i *indexer) indexPkgDocs(p *packages.Package, proID string) (err error) {
	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		if !strings.HasPrefix(fpos.Filename, i.projectRoot) {
			// Indexing test files means that we're also indexing the code generated by go test;
			// e.g. file://Users/efritz/Library/Caches/go-build/07/{64-character identifier}-d
			continue
		}

		if _, ok := i.files[fpos.Filename]; ok {
			// Emit each document only once
			continue
		}

		docID, err := i.w.EmitDocument(LanguageGo, fpos.Filename)
		if err != nil {
			return fmt.Errorf(`emit "document": %v`, err)
		}

		_, err = i.w.EmitBeginEvent("document", docID)
		if err != nil {
			return fmt.Errorf(`emit "begin": %v`, err)
		}

		_, err = i.w.EmitContains(proID, []string{docID})
		if err != nil {
			return fmt.Errorf(`emit "contains": %v`, err)
		}

		fi := &fileInfo{docID: docID}
		i.files[fpos.Filename] = fi

		// Create the map used to deduplicate ids within this file. This will be used
		// by indexPkgDefs and indexPkgUses, which assumes this key is already populated.
		i.ranges[fpos.Filename] = map[int]string{}
	}

	return nil
}

func (i *indexer) addPkgImports(pkgs []*packages.Package, p *packages.Package, proID string) (err error) {
	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		fi, ok := i.files[fpos.Filename]
		if !ok {
			// File skipped in the loop above
			continue
		}

		if _, ok := i.packagesImported[fpos.Filename]; ok {
			// Defs already imported
			continue
		}
		i.packagesImported[fpos.Filename] = true

		if err = i.addImports(p, f, fi); err != nil {
			return fmt.Errorf("error indexing imports of %q: %v", p.PkgPath, err)
		}
	}

	return nil
}

func (i *indexer) indexPkgDefs(pkgs []*packages.Package, p *packages.Package, proID string) (err error) {
	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		fi, ok := i.files[fpos.Filename]
		if !ok {
			// File skipped in the loop above
			continue
		}

		if _, ok := i.defsIndexed[fpos.Filename]; ok {
			// Defs already indexed
			continue
		}
		i.defsIndexed[fpos.Filename] = true

		if err = i.indexDefs(pkgs, p, f, fi, proID, fpos.Filename); err != nil {
			return fmt.Errorf("error indexing definitions of %q: %v", p.PkgPath, err)
		}
	}

	return nil
}

func (i *indexer) indexPkgUses(pkgs []*packages.Package, p *packages.Package, proID string) (err error) {
	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		fi, ok := i.files[fpos.Filename]
		if !ok {
			// File skipped in the loop above
			continue
		}

		if _, ok := i.usesIndexed[fpos.Filename]; ok {
			// Uses already indexed
			continue
		}
		i.usesIndexed[fpos.Filename] = true

		if err := i.indexUses(pkgs, p, f, fi, fpos.Filename); err != nil {
			return fmt.Errorf("error indexing uses of %q: %v", p.PkgPath, err)
		}
	}

	return nil
}

// addImports constructs *ast.Ident and types.Object out of *ImportSpec and inserts them into
// packages defs map to be indexed within a unified process.
func (i *indexer) addImports(p *packages.Package, f *ast.File, fi *fileInfo) error {
	for _, ispec := range f.Imports {
		// The path value comes from *ImportSpec has surrounding double quotes.
		// We should preserve its original format in constructing related AST objects
		// for any possible consumers. We use trimmed version here only when we need to
		// (trimmed version as a map key or an argument).
		ipath := strings.Trim(ispec.Path.Value, `"`)
		if p.Imports[ipath] == nil {
			// There is no package information if the package cannot be located from the
			// file system (i.i. missing files of a dependency).
			continue
		}

		var name string
		if ispec.Name == nil {
			name = ispec.Path.Value
		} else {
			name = ispec.Name.String()
		}
		p.TypesInfo.Defs[&ast.Ident{
			NamePos: ispec.Pos(),
			Name:    name,
			Obj:     ast.NewObj(ast.Pkg, name),
		}] = types.NewPkgName(ispec.Pos(), p.Types, name, p.Imports[ipath].Types)
	}
	return nil
}

func (i *indexer) indexDefs(pkgs []*packages.Package, p *packages.Package, f *ast.File, fi *fileInfo, proID, filename string) error {
	var rangeIDs []string
	for ident, obj := range p.TypesInfo.Defs {
		// Object is nil when not denote an object
		if obj == nil {
			continue
		}

		// Only emit if the object belongs to current file
		ipos := p.Fset.Position(ident.Pos())
		if ipos.Filename != filename {
			continue
		}

		// If we have a range for this offset then we've already indexed
		// this definition. Just early out in this situation.
		if _, ok := i.ranges[filename][ipos.Offset]; ok {
			continue
		}

		isQuotedPkgName := false
		if pkgName, ok := obj.(*types.PkgName); ok {
			isQuotedPkgName = strings.HasPrefix(pkgName.Name(), `"`)
		}

		rangeID, err := i.w.EmitRange(lspRange(ipos, ident.Name, isQuotedPkgName))
		if err != nil {
			return fmt.Errorf(`emit "range": %v`, err)
		}
		i.ranges[filename][ipos.Offset] = rangeID

		refResult, ok := i.refs[rangeID]
		if !ok {
			resultSetID, err := i.w.EmitResultSet()
			if err != nil {
				return fmt.Errorf(`emit "resultSet": %v`, err)
			}

			refResult = &refResultInfo{
				resultSetID: resultSetID,
				defRangeIDs: map[string][]string{},
				refRangeIDs: map[string][]string{},
			}

			i.refs[rangeID] = refResult
		}

		refResult.defRangeIDs[fi.docID] = append(refResult.defRangeIDs[fi.docID], rangeID)

		_, err = i.w.EmitNext(rangeID, refResult.resultSetID)
		if err != nil {
			return fmt.Errorf(`emit "next": %v`, err)
		}

		defResultID, err := i.w.EmitDefinitionResult()
		if err != nil {
			return fmt.Errorf(`emit "definitionResult": %v`, err)
		}

		_, err = i.w.EmitTextDocumentDefinition(refResult.resultSetID, defResultID)
		if err != nil {
			return fmt.Errorf(`emit "textDocument/definition": %v`, err)
		}

		_, err = i.w.EmitItem(defResultID, []string{rangeID}, fi.docID)
		if err != nil {
			return fmt.Errorf(`emit "item": %v`, err)
		}

		defInfo := &defInfo{
			docID:       fi.docID,
			rangeID:     rangeID,
			resultSetID: refResult.resultSetID,
			defResultID: defResultID,
		}

		switch v := obj.(type) {
		case *types.Func:
			i.funcs[v.FullName()] = defInfo

		case *types.Const:
			i.consts[ident.Pos()] = defInfo

		case *types.Var:
			i.vars[ident.Pos()] = defInfo

		case *types.TypeName:
			i.types[obj.Type().String()] = defInfo

		case *types.Label:
			i.labels[ident.Pos()] = defInfo

		case *types.PkgName:
			i.imports[ident.Pos()] = defInfo

			err := i.emitImportMoniker(refResult.resultSetID, strings.Trim(ident.String(), `"`))
			if err != nil {
				return fmt.Errorf(`emit moniker": %v`, err)
			}

		default:
			continue
		}

		if ident.IsExported() {
			err := i.emitExportMoniker(refResult.resultSetID, monikerIdentifier(f, p.PkgPath, ident, obj))
			if err != nil {
				return fmt.Errorf(`emit moniker": %v`, err)
			}
		}

		hoverResultID, err := i.makeCachedHoverResult(pkgs, p, f, obj)
		if err != nil {
			return err
		}

		_, err = i.w.EmitTextDocumentHover(refResult.resultSetID, hoverResultID)
		if err != nil {
			return fmt.Errorf(`emit "textDocument/hover": %v`, err)
		}

		rangeIDs = append(rangeIDs, rangeID)
	}

	fi.defRangeIDs = append(fi.defRangeIDs, rangeIDs...)
	return nil
}

func (i *indexer) indexUses(pkgs []*packages.Package, p *packages.Package, f *ast.File, fi *fileInfo, filename string) error {
	var rangeIDs []string
	for ident, obj := range p.TypesInfo.Uses {
		// Only emit if the object belongs to current file
		ipos := p.Fset.Position(ident.Pos())
		if ipos.Filename != filename {
			continue
		}

		var def *defInfo
		switch v := obj.(type) {
		case *types.Func:
			def = i.funcs[v.FullName()]

		case *types.Const:
			def = i.consts[v.Pos()]

		case *types.Var:
			def = i.vars[v.Pos()]

		case *types.TypeName:
			def = i.types[obj.Type().String()]

		case *types.Label:
			def = i.labels[v.Pos()]

		case *types.PkgName:
			def = i.imports[v.Pos()]
		}

		pkg := obj.Pkg()
		if def == nil && pkg == nil {
			// No range to emit because have neither a definition nor a moniker to
			// attach to the range.
			continue
		}

		var err error

		// Make a new range if we haven't already seen a def or a use that had
		// constructed a range at the same position.
		rangeID, ok := i.ranges[filename][ipos.Offset]
		if !ok {
			rangeID, err = i.w.EmitRange(lspRange(ipos, ident.Name, false))
			if err != nil {
				return fmt.Errorf(`emit "range": %v`, err)
			}
			i.ranges[filename][ipos.Offset] = rangeID
		}
		rangeIDs = append(rangeIDs, rangeID)

		if def == nil {
			hoverResultID, err := i.makeCachedExternalHoverResult(pkgs, p, obj, pkg)
			if err != nil {
				return err
			}

			if hoverResultID != "" {
				_, err = i.w.EmitTextDocumentHover(rangeID, hoverResultID)
				if err != nil {
					return fmt.Errorf(`emit "textDocument/hover": %v`, err)
				}
			}

			// If we don't have a definition in this package, emit an import moniker
			// so that we can correlate it with another dump's LSIF data.
			err = i.emitImportMoniker(rangeID, monikerIdentifier(f, pkg.Path(), ident, obj))
			if err != nil {
				return fmt.Errorf(`emit moniker": %v`, err)
			}

			// Emit a reference result edge and create a small set of edges that link
			// the reference result to the range (and vice versa). This is necessary to
			// mark this range as a reference to _something_, even though the definition
			// does not exist in this source code.

			refResultID, err := i.w.EmitReferenceResult()
			if err != nil {
				return fmt.Errorf(`emit "referenceResult": %v`, err)
			}

			_, err = i.w.EmitTextDocumentReferences(rangeID, refResultID)
			if err != nil {
				return fmt.Errorf(`emit "textDocument/references": %v`, err)
			}

			_, err = i.w.EmitItemOfReferences(refResultID, []string{rangeID}, fi.docID)
			if err != nil {
				return fmt.Errorf(`emit "item": %v`, err)
			}

			continue
		}

		_, err = i.w.EmitNext(rangeID, def.resultSetID)
		if err != nil {
			return fmt.Errorf(`emit "next": %v`, err)
		}

		refResult := i.refs[def.rangeID]
		if refResult != nil {
			refResult.refRangeIDs[fi.docID] = append(refResult.refRangeIDs[fi.docID], rangeID)
		}
	}

	fi.useRangeIDs = append(fi.useRangeIDs, rangeIDs...)
	return nil
}

// makeCachedHoverResult creates a hover result vertex and returns its iD. This
// method should be called for each definition in the set of indexed files. This
// method will not create a new vertex for a pkgName object that has been seen
// before. Because package text is likely to be large, and repeated at each import
// and each use in the file (over many multiple files), this can save many copies
// of the same text.
func (i *indexer) makeCachedHoverResult(pkgs []*packages.Package, p *packages.Package, f *ast.File, obj types.Object) (string, error) {
	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		// Defined in file, generate unique text
		return i.makeHoverResult(pkgs, p, f, obj)
	}

	key := pkgName.Imported().Path()
	if hoverResultID, ok := i.hoverResultCache[key]; ok {
		return hoverResultID, nil
	}

	hoverResultID, err := i.makeHoverResult(pkgs, p, f, obj)
	if err != nil {
		return "", err
	}
	i.hoverResultCache[key] = hoverResultID

	return hoverResultID, nil
}

// makeHoverResult create a hover result vertex and returns its ID. This method
// should be called for each definition in the set of indexed files.
func (i *indexer) makeHoverResult(pkgs []*packages.Package, p *packages.Package, f *ast.File, obj types.Object) (string, error) {
	contents, err := findContents(pkgs, p, f, obj, i.hoverLoader)
	if err != nil {
		return "", fmt.Errorf("find contents: %v", err)
	}

	hoverResultID, err := i.w.EmitHoverResult(contents)
	if err != nil {
		return "", fmt.Errorf(`emit "hoverResult": %v`, err)
	}

	return hoverResultID, nil
}

// makeCachedExternalHoverResult creates a hover result vertex and returns its ID.
// This method should be called for objects defined externally with a resolvable
// package. This method will only make a new vertex if the path and object-position
// pair has not been seen before. Because some methods in an external package are
// likely to be used more than once in a project, this can save many copies of the
// same text.
func (i *indexer) makeCachedExternalHoverResult(pkgs []*packages.Package, p *packages.Package, obj types.Object, pkg *types.Package) (string, error) {
	key := fmt.Sprintf("%s::%d", pkg.Path(), obj.Pos())
	if hoverResultID, ok := i.hoverResultCache[key]; ok {
		return hoverResultID, nil
	}

	contents, err := externalHoverContents(pkgs, p, obj, pkg, i.hoverLoader)
	if err != nil || contents == nil {
		return "", err
	}

	hoverResultID, err := i.w.EmitHoverResult(contents)
	if err != nil {
		return "", fmt.Errorf(`emit "hoverResult": %v`, err)
	}
	i.hoverResultCache[key] = hoverResultID

	return hoverResultID, nil
}

func (i *indexer) ensurePackageInformation(packageName, version string) (string, error) {
	packageInformationID, ok := i.packageInformationIDs[packageName]
	if ok {
		return packageInformationID, nil
	}

	packageInformationID, err := i.w.EmitPackageInformation(packageName, "gomod", version)
	if err != nil {
		return "", err
	}

	i.packageInformationIDs[packageName] = packageInformationID
	return packageInformationID, nil
}

func (i *indexer) emitImportMoniker(sourceID, identifier string) error {
	for _, moduleName := range packagePrefixes(strings.Split(identifier, ":")[0]) {
		moduleVersion, ok := i.dependencies[moduleName]
		if !ok {
			continue
		}

		packageInformationID, err := i.ensurePackageInformation(moduleName, moduleVersion)
		if err != nil {
			return err
		}

		return i.addMonikers("import", identifier, sourceID, packageInformationID)
	}

	return nil
}

func (i *indexer) emitExportMoniker(sourceID, identifier string) error {
	if i.moduleName == "" {
		// Unknown dependencies, skip export monikers
		return nil
	}

	packageInformationID, err := i.ensurePackageInformation(i.moduleName, i.moduleVersion)
	if err != nil {
		return err
	}

	return i.addMonikers("export", identifier, sourceID, packageInformationID)
}

// addMonikers outputs a "gomod" moniker vertex, attaches the given package vertex
// identifier to it, and attaches the new moniker to the source moniker vertex.
func (i *indexer) addMonikers(kind, identifier, sourceID, packageID string) error {
	monikerID, err := i.w.EmitMoniker(kind, "gomod", identifier)
	if err != nil {
		return err
	}

	if _, err := i.w.EmitPackageInformationEdge(monikerID, packageID); err != nil {
		return err
	}

	if _, err := i.w.EmitMonikerEdge(sourceID, monikerID); err != nil {
		return err
	}

	return nil
}

// packagePrefixes returns all prefixes of the go package path.
// For example, the package `foo/bar/baz` will return
//   - `foo/bar/baz`
//   - `foo/bar`
//   - `foo`
func packagePrefixes(packageName string) []string {
	parts := strings.Split(packageName, "/")
	prefixes := make([]string, len(parts))

	for i := 1; i <= len(parts); i++ {
		prefixes[len(parts)-i] = strings.Join(parts[:i], "/")
	}

	return prefixes
}
