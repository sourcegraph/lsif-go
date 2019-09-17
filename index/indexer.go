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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sourcegraph/lsif-go/log"
	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/packages"
)

// Index generates an LSIF dump for a workspace by traversing through source files
// and storing LSP responses to output source that implements io.Writer. It is
// caller's responsibility to close the output source if applicable.
func Index(workspace string, excludeContent bool, w io.Writer, toolInfo protocol.ToolInfo) (*Stats, error) {
	projectRoot, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("get abspath of project root: %v", err)
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles |
			packages.NeedImports | packages.NeedDeps |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir: projectRoot,
	}, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %v", err)
	}

	e := &indexer{
		projectRoot:    projectRoot,
		excludeContent: excludeContent,
		w:              w,

		pkgs:    pkgs,
		files:   make(map[string]*fileInfo),
		imports: make(map[token.Pos]*defInfo),
		funcs:   make(map[string]*defInfo),
		consts:  make(map[token.Pos]*defInfo),
		vars:    make(map[token.Pos]*defInfo),
		types:   make(map[string]*defInfo),
		labels:  make(map[token.Pos]*defInfo),
		refs:    make(map[string]*refResultInfo),
	}
	return e.index(toolInfo)
}

// indexer keeps track of all information needed to generate a LSIF dump.
type indexer struct {
	projectRoot    string
	excludeContent bool
	w              io.Writer

	id      int // The ID counter of the last element emitted
	pkgs    []*packages.Package
	files   map[string]*fileInfo      // Keys: filename
	imports map[token.Pos]*defInfo    // Keys: definition position
	funcs   map[string]*defInfo       // Keys: full name (with receiver for methods)
	consts  map[token.Pos]*defInfo    // Keys: definition position
	vars    map[token.Pos]*defInfo    // Keys: definition position
	types   map[string]*defInfo       // Keys: type name
	labels  map[token.Pos]*defInfo    // Keys: definition position
	refs    map[string]*refResultInfo // Keys: definition range ID
}

// Stats contains statistics of data processed during index.
type Stats struct {
	NumPkgs     int
	NumFiles    int
	NumDefs     int
	NumElements int
}

func (e *indexer) index(info protocol.ToolInfo) (*Stats, error) {
	_, err := e.emitMetaData("file://"+e.projectRoot, info)
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

	for _, p := range e.pkgs {
		if err := e.indexPkg(p, proID); err != nil {
			return nil, fmt.Errorf("index package %q: %v", p.Name, err)
		}
	}

	for _, f := range e.files {
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
			_, err = e.emitContains(f.docID, append(f.defRangeIDs, f.useRangeIDs...))
			if err != nil {
				return nil, fmt.Errorf(`emit "contains": %v`, err)
			}
		}
	}

	// Close all documents. This must be done as a last step as we need
	// to emit everything about a document before sending the end event.

	// TODO(efritz) - see if we can rearrange the outputs so that
	// all of the output for a document is contained in one segment
	// that does not interfere with emission of other document
	// properties.

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
		NumPkgs:     len(e.pkgs),
		NumFiles:    len(e.files),
		NumDefs:     len(e.imports) + len(e.funcs) + len(e.consts) + len(e.vars) + len(e.types) + len(e.labels),
		NumElements: e.id,
	}, nil
}

func (e *indexer) indexPkg(p *packages.Package, proID string) (err error) {
	log.Infoln("Package:", p.Name)
	defer log.Infoln()

	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		log.Infoln("\tFile:", fpos.Filename)

		fi, ok := e.files[fpos.Filename]
		if !ok {
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

			fi = &fileInfo{docID: docID}
			e.files[fpos.Filename] = fi
		}

		if err = e.addImports(p, f, fi); err != nil {
			return fmt.Errorf("error indexing imports of %q: %v", p.PkgPath, err)
		}

		if err = e.indexDefs(p, f, fi, proID, fpos.Filename); err != nil {
			return fmt.Errorf("error indexing definitions of %q: %v", p.PkgPath, err)
		}
	}

	// NOTE: As long as the flag `packages.NeedDeps` is passed to the load config,
	// `go/packages` does a post-order traversal of packages. That means
	// dependencies of a package are visited before the package itself, which
	// guarantees all defs that would be referenced in this package have been
	// added by the time this package is visited.
	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		if err := e.indexUses(p, e.files[fpos.Filename], fpos.Filename); err != nil {
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

		// TODO(jchen): emit other documents on the fly, thus we do not need to iterate
		//  over this map once for every file.
		// Only emit if the object belongs to current file
		ipos := p.Fset.Position(ident.Pos())
		if ipos.Filename != filename {
			continue
		}

		rangeID, err := e.emitRange(lspRange(ipos, ident.Name))
		if err != nil {
			return fmt.Errorf(`emit "range": %v`, err)
		}

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

		defInfo := &defInfo{
			docID:       fi.docID,
			rangeID:     rangeID,
			resultSetID: refResult.resultSetID,
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

			err := e.emitMoniker("import", refResult.resultSetID, strings.Trim(ident.String(), `"`))
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
			err := e.emitMoniker("export", refResult.resultSetID, fmt.Sprintf("%s.%s", p.String(), ident.String()))
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

		if def == nil {
			continue
		}

		rangeID, err := e.emitRange(lspRange(ipos, ident.Name))
		if err != nil {
			return fmt.Errorf(`emit "range": %v`, err)
		}
		rangeIDs = append(rangeIDs, rangeID)

		_, err = e.emitNext(rangeID, def.resultSetID)
		if err != nil {
			return fmt.Errorf(`emit "next": %v`, err)
		}

		// If this is the first use for this definition, we need to create
		// some extra vertices. Caching this on the definition lets us share
		// the vertices between uses. We do this lazily so that we don't have
		// an unreachable set of vertices.

		if def.defResultID == "" {
			defResultID, err := e.emitDefinitionResult()
			if err != nil {
				return fmt.Errorf(`emit "definitionResult": %v`, err)
			}

			_, err = e.emitTextDocumentDefinition(def.resultSetID, defResultID)
			if err != nil {
				return fmt.Errorf(`emit "textDocument/definition": %v`, err)
			}

			def.defResultID = defResultID

			_, err = e.emitItem(def.defResultID, []string{def.rangeID}, def.docID)
			if err != nil {
				return fmt.Errorf(`emit "item": %v`, err)
			}
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

func (e *indexer) emitMoniker(kind, sourceID, identifier string) error {
	monikerID := e.nextID()
	err := e.emit(protocol.NewMoniker(monikerID, kind, protocol.LanguageID, identifier))
	if err != nil {
		return err
	}

	return e.emit(protocol.NewMonikerEdge(e.nextID(), sourceID, monikerID))
}
