// Package index is used to generate an LSIF dump for a workspace.
package index

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/sourcegraph/lsif-go/log"
	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/packages"
)

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

// indexer keeps track of all information needed to generate a LSIF dump.
type indexer struct {
	projectRoot       string
	excludeContent    bool
	printProgressDots bool
	toolInfo          protocol.ToolInfo
	w                 io.Writer

	// De-duplication
	defsIndexed map[string]bool
	usesIndexed map[string]bool
	ranges      map[string]map[int]string // filename -> offset -> rangeID

	// Type correlation
	id      int                       // The ID counter of the last element emitted
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
func NewIndexer(projectRoot, moduleName, moduleVersion string, dependencies map[string]string, excludeContent, printProgressDots bool, toolInfo protocol.ToolInfo, w io.Writer) Indexer {
	return &indexer{
		projectRoot:       projectRoot,
		moduleName:        moduleName,
		moduleVersion:     moduleVersion,
		dependencies:      dependencies,
		excludeContent:    excludeContent,
		printProgressDots: printProgressDots,
		toolInfo:          toolInfo,
		w:                 w,

		// Empty maps
		defsIndexed:           map[string]bool{},
		usesIndexed:           map[string]bool{},
		ranges:                map[string]map[int]string{},
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

// Index generates an LSIF dump for a workspace by traversing through source files
// and storing LSP responses to output source that implements io.Writer. It is
// caller's responsibility to close the output source if applicable.
func (e *indexer) Index() (*Stats, error) {
	pkgs, err := e.packages()
	if err != nil {
		return nil, err
	}

	return e.index(pkgs)
}

func (e *indexer) packages() ([]*packages.Package, error) {
	log.Infoln("Loading packages...")

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles |
			packages.NeedImports | packages.NeedDeps |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:   e.projectRoot,
		Tests: true,
		Logf: func(format string, args ...interface{}) {
			// Print progress while the packages are loading
			// We don't need to log this information, though
			// (it's incredibly verbose)
			if e.printProgressDots {
				fmt.Fprintf(os.Stdout, ".")
			}
		},
	}, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %v", err)
	}

	return pkgs, nil
}

func (e *indexer) index(pkgs []*packages.Package) (*Stats, error) {
	_, err := e.emitMetaData("file://"+e.projectRoot, e.toolInfo)
	if err != nil {
		return nil, fmt.Errorf(`emit "metadata": %v`, err)
	}
	proID, err := e.emitProject()
	if err != nil {
		return nil, fmt.Errorf(`emit "project": %v`, err)
	}

	_, err = e.emitBeginEvent("project", proID)
	if err != nil {
		return nil, fmt.Errorf(`emit "begin": %v`, err)
	}

	for _, p := range pkgs {
		if err := e.indexPkgDocs(p, proID); err != nil {
			return nil, fmt.Errorf("index package %q: %v", p.Name, err)
		}

		if err := e.indexPkgDefs(p, proID); err != nil {
			return nil, fmt.Errorf("index package defs %q: %v", p.Name, err)
		}
	}

	for _, p := range pkgs {
		if err := e.indexPkgUses(p, proID); err != nil {
			return nil, fmt.Errorf("index package uses %q: %v", p.Name, err)
		}
	}

	log.Infoln("Linking references...")

	for _, f := range e.files {
		if e.printProgressDots {
			fmt.Fprintf(os.Stdout, ".")
		}

		for _, rangeID := range f.defRangeIDs {
			refResultID, err := e.emitReferenceResult()
			if err != nil {
				return nil, fmt.Errorf(`emit "referenceResult": %v`, err)
			}

			_, err = e.emitTextDocumentReferences(e.refs[rangeID].resultSetID, refResultID)
			if err != nil {
				return nil, fmt.Errorf(`emit "textDocument/references": %v`, err)
			}

			for docID, rangeIDs := range e.refs[rangeID].defRangeIDs {
				_, err = e.emitItemOfDefinitions(refResultID, rangeIDs, docID)
				if err != nil {
					return nil, fmt.Errorf(`emit "item": %v`, err)
				}
			}

			for docID, rangeIDs := range e.refs[rangeID].refRangeIDs {
				_, err = e.emitItemOfReferences(refResultID, rangeIDs, docID)
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

			_, err = e.emitContains(f.docID, allRanges)
			if err != nil {
				return nil, fmt.Errorf(`emit "contains": %v`, err)
			}
		}
	}

	// Close all documents. This must be done as a last step as we need
	// to emit everything about a document before sending the end event.

	for _, info := range e.files {
		_, err = e.emitEndEvent("document", info.docID)
		if err != nil {
			return nil, fmt.Errorf(`emit "end": %v`, err)
		}
	}

	_, err = e.emitEndEvent("project", proID)
	if err != nil {
		return nil, fmt.Errorf(`emit "end": %v`, err)
	}

	return &Stats{
		NumPkgs:     len(pkgs),
		NumFiles:    len(e.files),
		NumDefs:     len(e.imports) + len(e.funcs) + len(e.consts) + len(e.vars) + len(e.types) + len(e.labels),
		NumElements: e.id,
	}, nil
}

func (e *indexer) indexPkgDocs(p *packages.Package, proID string) (err error) {
	log.Infoln("Emitting documents for package", p.Name)
	defer log.Infoln()

	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		if !strings.HasPrefix(fpos.Filename, e.projectRoot) {
			// Indexing test files means that we're also indexing the code generated by go test;
			// e.g. file://Users/efritz/Library/Caches/go-build/07/{64-character identifier}-d
			continue
		}

		if _, ok := e.files[fpos.Filename]; ok {
			// Emit each document only once
			continue
		}

		log.Infoln("\tFile:", fpos.Filename)

		docID, err := e.emitDocument(fpos.Filename)
		if err != nil {
			return fmt.Errorf(`emit "document": %v`, err)
		}

		_, err = e.emitBeginEvent("document", docID)
		if err != nil {
			return fmt.Errorf(`emit "begin": %v`, err)
		}

		_, err = e.emitContains(proID, []string{docID})
		if err != nil {
			return fmt.Errorf(`emit "contains": %v`, err)
		}

		fi := &fileInfo{docID: docID}
		e.files[fpos.Filename] = fi

		// Create the map used to deduplicate ids within this file. This will be used
		// by indexPkgDefs and indexPkgUses, which assumes this key is already populated.
		e.ranges[fpos.Filename] = map[int]string{}
	}

	return nil
}

func (e *indexer) indexPkgDefs(p *packages.Package, proID string) (err error) {
	log.Infoln("Emitting definitions for package", p.Name)
	defer log.Infoln()

	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		fi, ok := e.files[fpos.Filename]
		if !ok {
			// File skipped in the loop above
			continue
		}

		if _, ok := e.defsIndexed[fpos.Filename]; ok {
			// Defs already indexed
			continue
		}
		e.defsIndexed[fpos.Filename] = true

		log.Infoln("\tFile:", fpos.Filename)

		if err = e.addImports(p, f, fi); err != nil {
			return fmt.Errorf("error indexing imports of %q: %v", p.PkgPath, err)
		}

		if err = e.indexDefs(p, f, fi, proID, fpos.Filename); err != nil {
			return fmt.Errorf("error indexing definitions of %q: %v", p.PkgPath, err)
		}
	}

	return nil
}

func (e *indexer) indexPkgUses(p *packages.Package, proID string) (err error) {
	log.Infoln("Emitting references for package", p.Name)
	defer log.Infoln()

	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		fi, ok := e.files[fpos.Filename]
		if !ok {
			// File skipped in the loop above
			continue
		}

		if _, ok := e.usesIndexed[fpos.Filename]; ok {
			// Uses already indexed
			continue
		}
		e.usesIndexed[fpos.Filename] = true

		log.Infoln("\tFile:", fpos.Filename)

		if err := e.indexUses(p, fi, fpos.Filename); err != nil {
			return fmt.Errorf("error indexing uses of %q: %v", p.PkgPath, err)
		}
	}

	return nil
}

// addImports constructs *ast.Ident and types.Object out of *ImportSpec and inserts them into
// packages defs map to be indexed within a unified process.
func (e *indexer) addImports(p *packages.Package, f *ast.File, fi *fileInfo) error {
	for _, ispec := range f.Imports {
		// The path value comes from *ImportSpec has surrounding double quotes.
		// We should preserve its original format in constructing related AST objects
		// for any possible consumers. We use trimmed version here only when we need to
		// (trimmed version as a map key or an argument).
		ipath := strings.Trim(ispec.Path.Value, `"`)
		if p.Imports[ipath] == nil {
			// There is no package information if the package cannot be located from the
			// file system (i.e. missing files of a dependency).
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
		log.Debugln("[import] Path:", ipath)
		log.Debugln("[import] Name:", ispec.Name)
		log.Debugln("[import] iPos:", p.Fset.Position(ispec.Pos()))
	}
	return nil
}

func (e *indexer) indexDefs(p *packages.Package, f *ast.File, fi *fileInfo, proID, filename string) error {
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
		if _, ok := e.ranges[filename][ipos.Offset]; ok {
			continue
		}

		rangeID, err := e.emitRange(lspRange(ipos, ident.Name))
		if err != nil {
			return fmt.Errorf(`emit "range": %v`, err)
		}
		e.ranges[filename][ipos.Offset] = rangeID

		refResult, ok := e.refs[rangeID]
		if !ok {
			refResult = &refResultInfo{
				resultSetID: e.nextID(),
				defRangeIDs: map[string][]string{},
				refRangeIDs: map[string][]string{},
			}

			e.refs[rangeID] = refResult
		}

		if _, ok := refResult.defRangeIDs[fi.docID]; !ok {
			refResult.defRangeIDs[fi.docID] = []string{}
		}
		refResult.defRangeIDs[fi.docID] = append(refResult.defRangeIDs[fi.docID], rangeID)

		if !ok {
			err = e.emit(protocol.NewResultSet(refResult.resultSetID))
			if err != nil {
				return fmt.Errorf(`emit "resultSet": %v`, err)
			}
		}

		_, err = e.emitNext(rangeID, refResult.resultSetID)
		if err != nil {
			return fmt.Errorf(`emit "next": %v`, err)
		}

		defResultID, err := e.emitDefinitionResult()
		if err != nil {
			return fmt.Errorf(`emit "definitionResult": %v`, err)
		}

		_, err = e.emitTextDocumentDefinition(refResult.resultSetID, defResultID)
		if err != nil {
			return fmt.Errorf(`emit "textDocument/definition": %v`, err)
		}

		_, err = e.emitItem(defResultID, []string{rangeID}, fi.docID)
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
			log.Debugln("[func] Def:", ident.Name)
			log.Debugln("[func] FullName:", v.FullName())
			log.Debugln("[func] iPos:", ipos)
			e.funcs[v.FullName()] = defInfo

		case *types.Const:
			log.Debugln("[const] Def:", ident.Name)
			log.Debugln("[const] iPos:", ipos)
			e.consts[ident.Pos()] = defInfo

		case *types.Var:
			log.Debugln("[var] Def:", ident.Name)
			log.Debugln("[var] iPos:", ipos)
			e.vars[ident.Pos()] = defInfo

		case *types.TypeName:
			log.Debugln("[typename] Def:", ident.Name)
			log.Debugln("[typename] Type:", obj.Type())
			log.Debugln("[typename] iPos:", ipos)
			e.types[obj.Type().String()] = defInfo

		case *types.Label:
			log.Debugln("[label] Def:", ident.Name)
			log.Debugln("[label] iPos:", ipos)
			e.labels[ident.Pos()] = defInfo

		case *types.PkgName:
			log.Debugln("[pkgname] Def:", ident)
			log.Debugln("[pkgname] iPos:", ipos)
			e.imports[ident.Pos()] = defInfo

			err := e.emitImportMoniker(refResult.resultSetID, strings.Trim(ident.String(), `"`))
			if err != nil {
				return fmt.Errorf(`emit moniker": %v`, err)
			}

		default:
			log.Debugf("[default] ---> %T\n", obj)
			log.Debugln("[default] Def:", ident)
			log.Debugln("[default] iPos:", ipos)
			continue
		}

		if ident.IsExported() {
			err := e.emitExportMoniker(refResult.resultSetID, fmt.Sprintf("%s:%s", p.PkgPath, ident.String()))
			if err != nil {
				return fmt.Errorf(`emit moniker": %v`, err)
			}
		}

		contents, err := findContents(f, obj)
		if err != nil {
			return fmt.Errorf("find contents: %v", err)
		}

		hoverResultID, err := e.emitHoverResult(contents)
		if err != nil {
			return fmt.Errorf(`emit "hoverResult": %v`, err)
		}

		_, err = e.emitTextDocumentHover(refResult.resultSetID, hoverResultID)
		if err != nil {
			return fmt.Errorf(`emit "textDocument/hover": %v`, err)
		}

		rangeIDs = append(rangeIDs, rangeID)
	}

	fi.defRangeIDs = append(fi.defRangeIDs, rangeIDs...)

	return nil
}

func (e *indexer) indexUses(p *packages.Package, fi *fileInfo, filename string) error {
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
			log.Debugln("[func] Use:", ident.Name)
			log.Debugln("[func] FullName:", v.FullName())
			log.Debugln("[func] iPos:", ipos)
			def = e.funcs[v.FullName()]

		case *types.Const:
			log.Debugln("[const] Use:", ident)
			log.Debugln("[const] iPos:", ipos)
			log.Debugln("[const] vPos:", p.Fset.Position(v.Pos()))
			def = e.consts[v.Pos()]

		case *types.Var:
			log.Debugln("[var] Use:", ident)
			log.Debugln("[var] iPos:", ipos)
			log.Debugln("[var] vPos:", p.Fset.Position(v.Pos()))
			def = e.vars[v.Pos()]

		case *types.TypeName:
			log.Debugln("[typename] Use:", ident.Name)
			log.Debugln("[typename] Type:", obj.Type())
			log.Debugln("[typename] iPos:", ipos)
			def = e.types[obj.Type().String()]

		case *types.Label:
			log.Debugln("[label] Use:", ident.Name)
			log.Debugln("[label] iPos:", ipos)
			log.Debugln("[label] vPos:", p.Fset.Position(v.Pos()))
			def = e.labels[v.Pos()]

		case *types.PkgName:
			log.Debugln("[pkgname] Use:", ident)
			log.Debugln("[pkgname] iPos:", ipos)
			log.Debugln("[pkgname] vPos:", p.Fset.Position(v.Pos()))
			def = e.imports[v.Pos()]

		// TODO(jchen): case *types.Builtin:

		// TODO(jchen): case *types.Nil:

		default:
			log.Debugln("[default] Use:", ident)
			log.Debugln("[default] iPos:", ipos)
			log.Debugln("[default] vPos:", p.Fset.Position(v.Pos()))
			continue
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
		rangeID, ok := e.ranges[filename][ipos.Offset]
		if !ok {
			rangeID, err = e.emitRange(lspRange(ipos, ident.Name))
			if err != nil {
				return fmt.Errorf(`emit "range": %v`, err)
			}
			e.ranges[filename][ipos.Offset] = rangeID
		}

		rangeIDs = append(rangeIDs, rangeID)

		if def == nil {
			// If we don't have a definition in this package, emit an import moniker
			// so that we can correlate it with another dump's LSIF data.
			err = e.emitImportMoniker(rangeID, fmt.Sprintf("%s:%s", pkg.Path(), obj.Id()))
			if err != nil {
				return fmt.Errorf(`emit moniker": %v`, err)
			}

			// Emit a reference result edge and create a small set of edges that link
			// the reference result to the range (and vice versa). This is necessary to
			// mark this range as a reference to _something_, even though the definition
			// does not exist in this source code.

			refResultID, err := e.emitReferenceResult()
			if err != nil {
				return fmt.Errorf(`emit "referenceResult": %v`, err)
			}

			_, err = e.emitTextDocumentReferences(rangeID, refResultID)
			if err != nil {
				return fmt.Errorf(`emit "textDocument/references": %v`, err)
			}

			_, err = e.emitItemOfReferences(refResultID, []string{rangeID}, fi.docID)
			if err != nil {
				return fmt.Errorf(`emit "item": %v`, err)
			}

			continue
		}

		_, err = e.emitNext(rangeID, def.resultSetID)
		if err != nil {
			return fmt.Errorf(`emit "next": %v`, err)
		}

		refResult := e.refs[def.rangeID]
		if refResult != nil {
			if _, ok := refResult.refRangeIDs[fi.docID]; !ok {
				refResult.refRangeIDs[fi.docID] = []string{}
			}
			refResult.refRangeIDs[fi.docID] = append(refResult.refRangeIDs[fi.docID], rangeID)
		}
	}

	fi.useRangeIDs = append(fi.useRangeIDs, rangeIDs...)
	return nil
}

func (e *indexer) writeNewLine() error {
	_, err := e.w.Write([]byte("\n"))
	return err
}

func (e *indexer) nextID() string {
	e.id++
	return strconv.Itoa(e.id)
}

func (e *indexer) emit(v interface{}) error {
	return json.NewEncoder(e.w).Encode(v)
}

func (e *indexer) emitMetaData(root string, info protocol.ToolInfo) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewMetaData(id, root, info))
}

func (e *indexer) emitBeginEvent(scope string, data string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewEvent(id, "begin", scope, data))
}

func (e *indexer) emitEndEvent(scope string, data string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewEvent(id, "end", scope, data))
}

func (e *indexer) emitProject() (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewProject(id))
}

func (e *indexer) emitDocument(path string) (string, error) {
	var contents []byte
	if !e.excludeContent {
		var err error
		contents, err = ioutil.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read file: %v", err)
		}
	}

	id := e.nextID()
	return id, e.emit(protocol.NewDocument(id, "file://"+path, contents))
}

func (e *indexer) emitContains(outV string, inVs []string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewContains(id, outV, inVs))
}

func (e *indexer) emitResultSet() (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewResultSet(id))
}

func (e *indexer) emitRange(start, end protocol.Pos) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewRange(id, start, end))
}

func (e *indexer) emitNext(outV, inV string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewNext(id, outV, inV))
}

func (e *indexer) emitDefinitionResult() (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewDefinitionResult(id))
}

func (e *indexer) emitTextDocumentDefinition(outV, inV string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentDefinition(id, outV, inV))
}

func (e *indexer) emitHoverResult(contents []protocol.MarkedString) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewHoverResult(id, contents))
}

func (e *indexer) emitTextDocumentHover(outV, inV string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentHover(id, outV, inV))
}

func (e *indexer) emitReferenceResult() (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewReferenceResult(id))
}

func (e *indexer) emitTextDocumentReferences(outV, inV string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentReferences(id, outV, inV))
}

func (e *indexer) emitItem(outV string, inVs []string, docID string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItem(id, outV, inVs, docID))
}

func (e *indexer) emitItemOfDefinitions(outV string, inVs []string, docID string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItemOfDefinitions(id, outV, inVs, docID))
}

func (e *indexer) emitItemOfReferences(outV string, inVs []string, docID string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItemOfReferences(id, outV, inVs, docID))
}

func (e *indexer) emitImportMoniker(sourceID, identifier string) error {
	for _, moduleName := range packagePrefixes(strings.Split(identifier, ":")[0]) {
		moduleVersion, ok := e.dependencies[moduleName]
		if !ok {
			continue
		}

		packageInformationID, err := e.ensurePackageInformation(moduleName, moduleVersion)
		if err != nil {
			return err
		}

		return e.addMonikers("import", identifier, sourceID, packageInformationID)
	}

	return nil
}

func (e *indexer) emitExportMoniker(sourceID, identifier string) error {
	packageInformationID, err := e.ensurePackageInformation(e.moduleName, e.moduleVersion)
	if err != nil {
		return err
	}

	return e.addMonikers("export", identifier, sourceID, packageInformationID)
}

func (e *indexer) ensurePackageInformation(packageName, version string) (string, error) {
	packageInformationID, ok := e.packageInformationIDs[packageName]
	if !ok {
		packageInformationID = e.nextID()
		err := e.emit(protocol.NewPackageInformation(packageInformationID, packageName, "gomod", version))
		if err != nil {
			return "", err
		}

		e.packageInformationIDs[packageName] = packageInformationID
	}

	return packageInformationID, nil
}

// addMonikers outputs a "gomod" moniker vertex, attaches the given package vertex
// identifier to it, and attaches the new moniker to the source moniker vertex.
func (e *indexer) addMonikers(kind string, identifier string, sourceID, packageID string) error {
	monikerID := e.nextID()
	err := e.emit(protocol.NewMoniker(monikerID, kind, "gomod", identifier))
	if err != nil {
		return err
	}

	err = e.emit(protocol.NewPackageInformationEdge(e.nextID(), monikerID, packageID))
	if err != nil {
		return err
	}

	err = e.emit(protocol.NewMonikerEdge(e.nextID(), sourceID, monikerID))
	if err != nil {
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
