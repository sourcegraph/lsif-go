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
	"log"
	"path/filepath"
	"strconv"

	doc "github.com/slimsag/godocmd"
	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/packages"
)

// Export generates an LSIF dump for a workspace by traversing through source files
// and storing LSP responses to output source that implements io.Writer. It is
// caller's responsibility to close the output source if applicable.
func Export(workspace string, w io.Writer, toolInfo protocol.ToolInfo) error {
	projectRoot, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("get abspath of project root: %v", err)
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles |
			packages.NeedImports | packages.NeedDeps |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir: projectRoot,
	}, "./...")
	if err != nil {
		return fmt.Errorf("load packages: %v", err)
	}

	return (&exporter{
		projectRoot: projectRoot,
		w:           w,

		pkgs:  pkgs,
		files: make(map[string]*fileInfo),
		funcs: make(map[string]*defInfo),
		vars:  make(map[token.Pos]*defInfo),
		types: make(map[string]*defInfo),
		refs:  make(map[string]*refResultInfo),
	}).export(toolInfo)
}

// exporter keeps track of all information needed to generate a LSIF dump.
type exporter struct {
	projectRoot string
	w           io.Writer

	id    int // The ID counter of the last element emitted
	pkgs  []*packages.Package
	files map[string]*fileInfo      // Keys: filename
	funcs map[string]*defInfo       // Keys: full name (with receiver for methods)
	vars  map[token.Pos]*defInfo    // Keys: definition position
	types map[string]*defInfo       // Keys: type name
	refs  map[string]*refResultInfo // Keys: definition range ID
}

func (e *exporter) export(info protocol.ToolInfo) error {
	_, err := e.emitMetaData("file://"+e.projectRoot, info)
	if err != nil {
		return fmt.Errorf(`emit "metadata": %v`, err)
	}
	proID, err := e.emitProject()
	if err != nil {
		return fmt.Errorf(`emit "project": %v`, err)
	}

	_, err = e.emitBeginEvent("project", proID)
	if err != nil {
		return fmt.Errorf(`emit "begin": %v`, err)
	}

	for _, p := range e.pkgs {
		if err := e.exportPkg(p, proID); err != nil {
			return fmt.Errorf("export package %q: %v", p.Name, err)
		}
	}

	for _, f := range e.files {
		for _, rangeID := range f.defRangeIDs {
			refResultID, err := e.emitReferenceResult()
			if err != nil {
				return fmt.Errorf(`emit "referenceResult": %v`, err)
			}

			_, err = e.emitTextDocumentReferences(e.refs[rangeID].resultSetID, refResultID)
			if err != nil {
				return fmt.Errorf(`emit "textDocument/references": %v`, err)
			}

			_, err = e.emitItemOfDefinitions(refResultID, e.refs[rangeID].defRangeIDs, f.docID)
			if err != nil {
				return fmt.Errorf(`emit "item": %v`, err)
			}

			if len(e.refs[rangeID].refRangeIDs) > 0 {
				_, err = e.emitItemOfReferences(refResultID, e.refs[rangeID].refRangeIDs, f.docID)
				if err != nil {
					return fmt.Errorf(`emit "item": %v`, err)
				}
			}
		}

		if len(f.defRangeIDs) > 0 || len(f.useRangeIDs) > 0 {
			_, err = e.emitContains(f.docID, append(f.defRangeIDs, f.useRangeIDs...))
			if err != nil {
				return fmt.Errorf(`emit "contains": %v`, err)
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
			return fmt.Errorf(`emit "end": %v`, err)
		}
	}

	_, err = e.emitEndEvent("project", proID)
	if err != nil {
		return fmt.Errorf(`emit "end": %v`, err)
	}

	return nil
}

func (e *exporter) exportPkg(p *packages.Package, proID string) (err error) {
	// TODO(jchen): support "-verbose" flag
	log.Println("Package:", p.Name)
	defer log.Println()

	for _, f := range p.Syntax {
		fpos := p.Fset.Position(f.Package)
		log.Println("\tFile:", fpos.Filename)

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

		if err := e.exportUses(p, fi, fpos.Filename); err != nil {
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
			// TODO(jchen): support "-verbose" flag
			//fmt.Printf("---> %T\n", obj)
			//fmt.Println("Def:", ident.Name)
			//fmt.Println("FullName:", v.FullName())
			//fmt.Println("iPos:", ipos)
			//fmt.Println("vPos:", p.Fset.Position(v.Pos()))
			e.funcs[v.FullName()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		// TODO(jchen): case *types.Const:

		case *types.Var:
			// TODO(jchen): support "-verbose" flag
			//fmt.Printf("---> %T\n", obj)
			//fmt.Println("Def:", ident.Name)
			//fmt.Println("iPos:", ipos)
			e.vars[ident.Pos()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		case *types.TypeName:
			// TODO(jchen): support "-verbose" flag
			//fmt.Println("Def:", ident.Name)
			//fmt.Println("Type:", obj.Type())
			//fmt.Println("Pos:", ipos)
			e.types[obj.Type().String()] = &defInfo{
				rangeID:     rangeID,
				resultSetID: refResult.resultSetID,
				contents:    contents,
			}

		// TODO(jchen): case *types.Label:

		// TODO(jchen): case *types.PkgName:

		// TODO(jchen): case *types.Builtin:

		// TODO(jchen): case *types.Nil:

		default:
			// TODO(jchen): remove this case-branch
			//fmt.Printf("---> %T\n", obj)
			//fmt.Println("(default)")
			//fmt.Println("Def:", ident)
			//fmt.Println("Pos:", ipos)
			//spew.Dump(obj)
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
			// TODO(jchen): support "-verbose" flag
			//fmt.Printf("---> %T\n", obj)
			//fmt.Println("Use:", ident.Name)
			//fmt.Println("FullName:", v.FullName())
			//fmt.Println("Pos:", ipos)
			//fmt.Println("Scope.Parent.Pos:", p.Fset.Position(v.Scope().Parent().Pos()))
			//fmt.Println("Scope.Pos:", p.Fset.Position(v.Scope().Pos()))
			def = e.funcs[v.FullName()]

		// TODO(jchen): case *types.Const:

		case *types.Var:
			// TODO(jchen): support "-verbose" flag
			//fmt.Printf("---> %T\n", obj)
			//fmt.Println("Use:", ident)
			//fmt.Println("iPos:", ipos)
			//fmt.Println("vPos:", p.Fset.Position(v.Pos()))
			def = e.vars[v.Pos()]

		// TODO(jchen): case *types.PkgName:
		//fmt.Println("Use:", ident)
		//fmt.Println("Pos:", ipos)
		//def = e.imports[ident.Name]

		case *types.TypeName:
			// TODO(jchen): support "-verbose" flag
			//fmt.Printf("---> %T\n", obj)
			//fmt.Println("Use:", ident.Name)
			//fmt.Println("Type:", obj.Type())
			//fmt.Println("Pos:", ipos)
			def = e.types[obj.Type().String()]

		// TODO(jchen): case *types.Label:

		// TODO(jchen): case *types.PkgName:

		// TODO(jchen): case *types.Builtin:

		// TODO(jchen): case *types.Nil:

		default:
			// TODO(jchen): remove this case-branch
			//fmt.Printf("---> %T\n", obj)
			//fmt.Println("(default)")
			//fmt.Println("Use:", ident)
			//fmt.Println("iPos:", ipos)
			//fmt.Println("vPos:", p.Fset.Position(v.Pos()))
			//spew.Dump(obj)
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

		defResultID, err := e.emitDefinitionResult()
		if err != nil {
			return fmt.Errorf(`emit "definitionResult": %v`, err)
		}

		_, err = e.emitTextDocumentDefinition(def.resultSetID, defResultID)
		if err != nil {
			return fmt.Errorf(`emit "textDocument/definition": %v`, err)
		}

		_, err = e.emitItem(defResultID, []string{def.rangeID}, fi.docID)
		if err != nil {
			return fmt.Errorf(`emit "item": %v`, err)
		}

		rangeIDs = append(rangeIDs, rangeID)

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
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %v", err)
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
