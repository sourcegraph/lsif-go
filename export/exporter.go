// Package export is used to generate an LSIF dump for a workspace.
package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"path/filepath"
	"strconv"

	doc "github.com/slimsag/godocmd"
	"github.com/sourcegraph/lsif-go/log"
	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/packages"
)

// Export generates an LSIF dump for a workspace by traversing through source files
// and storing LSP responses to output source that implements io.Writer. It is
// caller's responsibility to close the output source if applicable.
func Export(workspace string, excludeContent bool, w io.Writer, toolInfo protocol.ToolInfo) (*Stats, error) {
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

	e := &exporter{
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
	return e.export(toolInfo)
}

// exporter keeps track of all information needed to generate a LSIF dump.
type exporter struct {
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

// Stats contains statistics of data processed during export.
type Stats struct {
	NumPkgs     int
	NumFiles    int
	NumDefs     int
	NumElements int
}

func (e *exporter) export(info protocol.ToolInfo) (*Stats, error) {
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
		if err := e.exportPkg(p, proID); err != nil {
			return nil, fmt.Errorf("export package %q: %v", p.Name, err)
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

			_, err = e.emitItemOfDefinitions(refResultID, e.refs[rangeID].defRangeIDs, f.docID)
			if err != nil {
				return nil, fmt.Errorf(`emit "item": %v`, err)
			}

			if len(e.refs[rangeID].refRangeIDs) > 0 {
				_, err = e.emitItemOfReferences(refResultID, e.refs[rangeID].refRangeIDs, f.docID)
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

func (e *exporter) exportPkg(p *packages.Package, proID string) (err error) {
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

		if err = e.exportDefs(p, f, fi, proID, fpos.Filename); err != nil {
			return fmt.Errorf("error exporting definitions of %q: %v", p.PkgPath, err)
		}
	}

	// NOTE: since we currently only support package-level references, it is OK to export usages
	// at the end of each package. When repository-level references are implemented, usages must
	// be exported after all files are processed.
	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		if err := e.exportUses(p, e.files[fpos.Filename], fpos.Filename); err != nil {
			return fmt.Errorf("error exporting uses of %q: %v", p.PkgPath, err)
		}
	}

	return nil
}

func (e *exporter) exportDefs(p *packages.Package, f *ast.File, fi *fileInfo, proID, filename string) (err error) {
	var rangeIDs []string
	for ident, obj := range p.TypesInfo.Defs {
		// Object is nil when not denote an object
		if obj == nil {
			continue
		}

		// Only emit if the object belongs to current file
		// TODO(jchen): maybe emit other documents on the fly
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
			refResult = &refResultInfo{resultSetID: e.nextID()}
			e.refs[rangeID] = refResult
		}

		refResult.defRangeIDs = append(refResult.defRangeIDs, rangeID)

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

		qf := func(*types.Package) string { return "" }
		var s string
		var extra string
		if f, ok := obj.(*types.Var); ok && f.IsField() {
			// TODO(jchen): make this be like (T).F not "struct field F string".
			s = "struct " + obj.String()
		} else {
			if obj, ok := obj.(*types.TypeName); ok {
				typ := obj.Type().Underlying()
				if _, ok := typ.(*types.Struct); ok {
					s = "type " + obj.Name() + " struct"
					extra = prettyPrintTypesString(types.TypeString(typ, qf))
				}
				if _, ok := typ.(*types.Interface); ok {
					s = "type " + obj.Name() + " interface"
					extra = prettyPrintTypesString(types.TypeString(typ, qf))
				}
			}
			if s == "" {
				s = types.ObjectString(obj, qf)
			}
		}

		contents := []protocol.MarkedString{
			protocol.NewMarkedString(s),
		}
		comments, err := findComments(f, obj)
		if err != nil {
			return fmt.Errorf("find comments: %v", err)
		}
		if comments != "" {
			var b bytes.Buffer
			doc.ToMarkdown(&b, comments, nil)
			contents = append(contents, protocol.RawMarkedString(b.String()))
		}
		if extra != "" {
			contents = append(contents, protocol.NewMarkedString(extra))
		}

		switch v := obj.(type) {
		case *types.Func:
			log.Debugf("func", "---> %T\n", obj)
			log.Debugln("func", "Def:", ident.Name)
			log.Debugln("func", "FullName:", v.FullName())
			log.Debugln("func", "iPos:", ipos)
			log.Debugln("func", "vPos:", p.Fset.Position(v.Pos()))
			e.funcs[v.FullName()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		case *types.Const:
			log.Debugf("const", "---> %T\n", obj)
			log.Debugln("const", "Def:", ident.Name)
			log.Debugln("const", "iPos:", ipos)
			e.consts[ident.Pos()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		case *types.Var:
			log.Debugf("var", "---> %T\n", obj)
			log.Debugln("var", "Def:", ident.Name)
			log.Debugln("var", "iPos:", ipos)
			e.vars[ident.Pos()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		case *types.TypeName:
			log.Debugf("typename", "Def:", ident.Name)
			log.Debugln("typename", "Type:", obj.Type())
			log.Debugln("typename", "iPos:", ipos)
			e.types[obj.Type().String()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		case *types.Label:
			log.Debugf("label", "---> %T\n", obj)
			log.Debugln("label", "Def:", ident.Name)
			log.Debugln("label", "iPos:", ipos)
			e.labels[ident.Pos()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		case *types.PkgName:
			// TODO: support import paths are not renamed
			log.Debugf("pkgname", "---> %T\n", obj)
			log.Debugln("pkgname", "Use:", ident)
			log.Debugln("pkgname", "iPos:", ipos)
			e.imports[ident.Pos()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		default:
			log.Debugf("default", "---> %T\n", obj)
			log.Debugln("default", "(default)")
			log.Debugln("default", "Def:", ident)
			log.Debugln("default", "iPos:", ipos)
			continue
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

func (e *exporter) exportUses(p *packages.Package, fi *fileInfo, filename string) error {
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
			log.Debugf("func", "---> %T\n", obj)
			log.Debugln("func", "Use:", ident.Name)
			log.Debugln("func", "FullName:", v.FullName())
			log.Debugln("func", "iPos:", ipos)
			def = e.funcs[v.FullName()]

		case *types.Const:
			log.Debugf("const", "---> %T\n", obj)
			log.Debugln("const", "Use:", ident)
			log.Debugln("const", "iPos:", ipos)
			log.Debugln("const", "vPos:", p.Fset.Position(v.Pos()))
			def = e.consts[v.Pos()]

		case *types.Var:
			log.Debugf("var", "---> %T\n", obj)
			log.Debugln("var", "Use:", ident)
			log.Debugln("var", "iPos:", ipos)
			log.Debugln("var", "vPos:", p.Fset.Position(v.Pos()))
			def = e.vars[v.Pos()]

		case *types.TypeName:
			log.Debugf("typename", "---> %T\n", obj)
			log.Debugln("typename", "Use:", ident.Name)
			log.Debugln("typename", "Type:", obj.Type())
			log.Debugln("typename", "iPos:", ipos)
			def = e.types[obj.Type().String()]

		case *types.Label:
			log.Debugf("label", "---> %T\n", obj)
			log.Debugln("label", "Use:", ident.Name)
			log.Debugln("label", "iPos:", ipos)
			log.Debugln("label", "vPos:", p.Fset.Position(v.Pos()))
			def = e.labels[v.Pos()]

		case *types.PkgName:
			log.Debugf("pkgname", "---> %T\n", obj)
			log.Debugln("pkgname", "Use:", ident)
			log.Debugln("pkgname", "iPos:", ipos)
			log.Debugln("pkgname", "vPos:", p.Fset.Position(v.Pos()))
			def = e.imports[v.Pos()]

		// TODO(jchen): case *types.Builtin:

		// TODO(jchen): case *types.Nil:

		default:
			log.Debugf("default", "---> %T\n", obj)
			log.Debugln("default", "(default)")
			log.Debugln("default", "Use:", ident)
			log.Debugln("default", "iPos:", ipos)
			log.Debugln("default", "vPos:", p.Fset.Position(v.Pos()))
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
		}

		_, err = e.emitItem(def.defResultID, []string{def.rangeID}, fi.docID)
		if err != nil {
			return fmt.Errorf(`emit "item": %v`, err)
		}

		refResult := e.refs[def.rangeID]
		if refResult != nil {
			refResult.refRangeIDs = append(refResult.refRangeIDs, rangeID)
		}
	}

	fi.useRangeIDs = append(fi.useRangeIDs, rangeIDs...)

	return nil
}

func (e *exporter) writeNewLine() error {
	_, err := e.w.Write([]byte("\n"))
	return err
}

func (e *exporter) nextID() string {
	e.id++
	return strconv.Itoa(e.id)
}

func (e *exporter) emit(v interface{}) error {
	return json.NewEncoder(e.w).Encode(v)
}

func (e *exporter) emitMetaData(root string, info protocol.ToolInfo) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewMetaData(id, root, info))
}

func (e *exporter) emitBeginEvent(scope string, data string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewEvent(id, "begin", scope, data))
}

func (e *exporter) emitEndEvent(scope string, data string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewEvent(id, "end", scope, data))
}

func (e *exporter) emitProject() (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewProject(id))
}

func (e *exporter) emitDocument(path string) (string, error) {
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

func (e *exporter) emitContains(outV string, inVs []string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewContains(id, outV, inVs))
}

func (e *exporter) emitResultSet() (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewResultSet(id))
}

func (e *exporter) emitRange(start, end protocol.Pos) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewRange(id, start, end))
}

func (e *exporter) emitNext(outV, inV string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewNext(id, outV, inV))
}

func (e *exporter) emitDefinitionResult() (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewDefinitionResult(id))
}

func (e *exporter) emitTextDocumentDefinition(outV, inV string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentDefinition(id, outV, inV))
}

func (e *exporter) emitHoverResult(contents []protocol.MarkedString) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewHoverResult(id, contents))
}

func (e *exporter) emitTextDocumentHover(outV, inV string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentHover(id, outV, inV))
}

func (e *exporter) emitReferenceResult() (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewReferenceResult(id))
}

func (e *exporter) emitTextDocumentReferences(outV, inV string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentReferences(id, outV, inV))
}

func (e *exporter) emitItem(outV string, inVs []string, docID string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItem(id, outV, inVs, docID))
}

func (e *exporter) emitItemOfDefinitions(outV string, inVs []string, docID string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItemOfDefinitions(id, outV, inVs, docID))
}

func (e *exporter) emitItemOfReferences(outV string, inVs []string, docID string) (string, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItemOfReferences(id, outV, inVs, docID))
}
