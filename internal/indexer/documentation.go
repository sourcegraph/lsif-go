package indexer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	doc "github.com/slimsag/godocmd"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer"
	"golang.org/x/tools/go/packages"
)

// This file handles indexing in accordance with the Sourcegraph documentation LSIF extension:
//
// https://github.com/sourcegraph/sourcegraph/pull/20108
//
// The aim of this extension is to enable LSIF indexers to produce API documentation
// that is competitive with the API documentation offered in some languages today
// through various websites such as:
//
// * (Go) https://pkg.go.dev
// * (Rust) https://docs.rs
// * (Java) https://javadoc.io
// * (Zig) https://ziglang.org/documentation/master/std/#builtin
//

// A mapping of types -> documentationResult vertex ID
type emittedDocumentationResults map[NoahObject]uint64

func (e emittedDocumentationResults) addAll(other emittedDocumentationResults) map[NoahObject]uint64 {
	for associatedType, documentationResultID := range other {
		e[associatedType] = documentationResultID
	}
	return e
}

// indexDocumentation indexes all packages in the project.
func (i *Indexer) indexDocumentation() error {
	var (
		d                     = &docsIndexer{i: i}
		mu                    sync.Mutex
		docsPackages          []docsPackage
		emitted               = make(emittedDocumentationResults, 4096)
		emittedPackagesByPath = make(map[string]uint64, 32)
		errs                  error
	)
	i.visitEachPackage("Indexing documentation", func(p *packages.Package) {
		// Index the package without the lock, for parallelism.
		docsPkg, err := d.indexPackage(p)

		// Acquire the lock; note that multierror.Append could also be racy and hence we hold the
		// lock even for the error check. In practice, this is not where most of the work is done
		// (indexPackage is) so this is fine.
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = multierror.Append(errs, errors.Wrap(err, "package "+p.Name))
			return
		}
		emitted.addAll(docsPkg.emitted)
		docsPackages = append(docsPackages, docsPkg)
		emittedPackagesByPath[docsPkg.Path] = docsPkg.ID
	})

	// Find the root package path (e.g. "github.com/sourcegraph/sourcegraph").
	rootPkgPath := d.rootPkgPath()

	// Build an understanding of all pages in the workspace.
	type page struct {
		id       uint64   // the page itself
		children []uint64 // the children pages of this one
	}
	pagesByPath := map[string]*page{}
	for _, docsPkg := range docsPackages {
		relPackagePath := d.relPackagePath(docsPkg.Path, rootPkgPath)
		if _, exists := pagesByPath[relPackagePath]; exists {
			panic("invariant: no duplicate paths")
		}
		pagesByPath[relPackagePath] = &page{id: docsPkg.ID}
	}

	// Emit the root documentationResult which will link all packages in this project to the
	// project itself. If the root of the workspace is a Go package, this may already exist
	// and would be that Go package's documentation.
	if rootPage, ok := pagesByPath[""]; ok {
		_ = i.emitter.EmitDocumentationResultEdge(rootPage.id, i.projectID)
	} else {
		// Emit a blank index page.
		rootDocumentationID := (&documentationResult{
			Documentation: protocol.Documentation{
				Identifier: "",
				SearchKey:  "",
				NewPage:    true,
				Tags:       []protocol.Tag{},
			},
			Label:  protocol.NewMarkupContent("", protocol.PlainText),
			Detail: protocol.NewMarkupContent("", protocol.PlainText),
		}).emit(i.emitter)
		_ = i.emitter.EmitDocumentationResultEdge(rootDocumentationID, i.projectID)
		pagesByPath[""] = &page{id: rootDocumentationID}
	}

	// What we have now is pages for each package in the workspace, e.g.:
	//
	// 	/                      (root index page)
	// 	/internal/lib/protocol (package page)
	// 	/internal/lib/util     (package page)
	// 	/router/mux            (package page)
	//
	// What we want ot add in is index pages (blank pages) for each parent path so we end up with:
	//
	// 	/                      (root index page)
	// 	/internal              (index page)
	// 	/internal/lib          (index page)
	// 	/internal/lib/protocol (package page)
	// 	/internal/lib/util     (package page)
	// 	/router                (index page)
	// 	/router/mux            (package page)
	//
	// Note: the actual paths do not have a leading slash.
	sort.Slice(docsPackages, func(i, j int) bool {
		return docsPackages[i].Path < docsPackages[j].Path
	})
	for _, docsPkg := range docsPackages {
		relPackagePath := d.relPackagePath(docsPkg.Path, rootPkgPath)
		pkgPathElements := strings.Split(relPackagePath, "/") // ["internal", "lib", "protocol"]

		// Walk over each path: "internal", "internal/lib", "internal/lib/protocol" and emit an
		// index page for each that does not have it.
		currentPath := ""
		for _, element := range pkgPathElements {
			currentPath = path.Join(currentPath, element)
			_, ok := pagesByPath[currentPath]
			if ok {
				continue
			}
			currentPathElements := strings.Split(currentPath, "/")
			parentPath := path.Join(currentPathElements[:len(currentPathElements)-1]...)

			// Emit an index page at this path since one does not exist.
			pageID := (&documentationResult{
				Documentation: protocol.Documentation{
					Identifier: element,
					SearchKey:  "", // don't index for search
					NewPage:    true,
					Tags:       []protocol.Tag{},
				},
				Label:  protocol.NewMarkupContent("", protocol.PlainText),
				Detail: protocol.NewMarkupContent("", protocol.PlainText),
			}).emit(i.emitter)
			parentPage, ok := pagesByPath[parentPath]
			if !ok {
				panic("invariant: parentPage should always exist(1)")
			}
			parentPage.children = append(parentPage.children, pageID)
			pagesByPath[currentPath] = &page{id: pageID}
		}
	}

	// Finalize children of pages.
	for _, docsPkg := range docsPackages {
		relPackagePath := d.relPackagePath(docsPkg.Path, rootPkgPath)

		// Attach the children sections of the page (consts/vars/etc) as children of the page itself.
		page, ok := pagesByPath[relPackagePath]
		if !ok {
			panic("invariant: page should always exist")
		}
		page.children = append(page.children, docsPkg.children...)

		// Attach package documentation pages as children of their parent (either another package
		// documentation page, or a blank index page.)
		if relPackagePath == "" {
			// root is not a child of anything.
			continue
		}
		pkgPathElements := strings.Split(relPackagePath, "/") // ["internal", "lib", "protocol"]
		parentPath := path.Join(pkgPathElements[:len(pkgPathElements)-1]...)
		parentPage, ok := pagesByPath[parentPath]
		if !ok {
			panic("invariant: parentPage should always exist(2)")
		}
		parentPage.children = append(parentPage.children, docsPkg.ID)
	}

	// Emit children edges of all pages.
	for _, page := range pagesByPath {
		_ = i.emitter.EmitDocumentationChildrenEdge(page.children, page.id)
	}

	i.emittedDocumentationResults = emitted
	i.emittedDocumentationResultsByPackagePath = emittedPackagesByPath
	return errs
}

// The Go standard library at github.com/golang/go/src has a Go module name of "std"
// but we do not want stdlib package paths to be prefixed with "std/".
func pkgPathStdStrip(path string) string {
	return strings.TrimPrefix(path, "std/")
}

type docsIndexer struct {
	i *Indexer
}

func (d *docsIndexer) relPackagePath(pkgPath, rootPkgPath string) string {
	v := strings.TrimPrefix(pkgPath, rootPkgPath) // e.g. "/internal/lib/protocol"
	v = strings.TrimPrefix(v, "/")
	return v
}

func (d *docsIndexer) rootPkgPath() string {
	root := ""
	for _, pkg := range d.i.packages {
		if strings.HasPrefix(pkg.PkgPath, "std/") {
			return "" // Go stdlib
		}
		if root == "" || len(pkg.PkgPath) < len(root) {
			root = pkg.PkgPath
		}
	}
	return pkgPathStdStrip(root)
}

// docsPackage is the result of indexing documentation for a single Go package.
type docsPackage struct {
	// ID is the ID of the "documentationResult" vertex which describes this package.
	ID uint64

	// Path is the actual package path.
	Path string

	// A mapping of types -> documentationResult vertex ID
	emitted emittedDocumentationResults

	// children of the page to be attached later.
	children []uint64
}

// indexPackage indexes a single Go package.
func (d *docsIndexer) indexPackage(p *packages.Package) (docsPackage, error) {
	var (
		pkgDocsMarkdown string
		consts          []constVarDocs
		vars            []constVarDocs
		types           []typeDocs
		funcs           []funcDocs
		emitted         = make(emittedDocumentationResults, 64)
	)
	for _, file := range p.Syntax {
		filename := p.Fset.Position(file.Pos()).Filename
		if !strings.HasPrefix(filename, d.i.projectRoot) {
			// Omit files (such as those generated by `go test`) that aren't in the project root
			// because those are not externally accessible under any circumstance.
			continue
		}
		fileDocs, err := d.indexFile(p, file, filepath.Base(filename), strings.HasSuffix(filename, "_test.go"))
		if err != nil {
			return docsPackage{}, errors.Wrap(err, "file "+filename)
		}
		pkgDocsMarkdown += fileDocs.pkgDocsMarkdown
		for _, c := range fileDocs.consts {
			consts = append(consts, c)
			emitted[c.def] = c.ID
		}
		for _, v := range fileDocs.vars {
			vars = append(vars, v)
			emitted[v.def] = v.ID
		}
		for _, t := range fileDocs.types {
			types = append(types, t)
			emitted[t.def] = t.ID
		}
		for _, f := range fileDocs.funcs {
			funcs = append(funcs, f)
			emitted[f.def] = f.ID
		}
	}

	rootPkgPath := d.rootPkgPath()
	shortestUniquePkgPath := strings.TrimPrefix(strings.TrimPrefix(pkgPathStdStrip(p.PkgPath), rootPkgPath), "/")

	visibilityTags := []protocol.Tag{}
	if strings.Contains(p.PkgPath, "/internal/") || strings.HasSuffix(p.Name, "_test") {
		visibilityTags = append(visibilityTags, protocol.TagPrivate)
	}
	if isDeprecated(pkgDocsMarkdown) {
		visibilityTags = append(visibilityTags, protocol.TagDeprecated)
	}
	pkgTags := make([]protocol.Tag, len(visibilityTags))
	copy(pkgTags, visibilityTags)
	pkgTags = append(pkgTags, protocol.TagPackage)

	pkgPathElements := strings.Split(pkgPathStdStrip(p.PkgPath), "/")
	packageDocsID := (&documentationResult{
		Documentation: protocol.Documentation{
			Identifier: pkgPathElements[len(pkgPathElements)-1],
			SearchKey:  shortestUniquePkgPath,
			NewPage:    true,
			Tags:       pkgTags,
		},
		Label:  protocol.NewMarkupContent("Package "+p.Name, protocol.PlainText),
		Detail: protocol.NewMarkupContent(pkgDocsMarkdown, protocol.Markdown),
	}).emit(d.i.emitter)

	newSection := func(label, identifier string, children []uint64) uint64 {
		sectionID := (&documentationResult{
			Documentation: protocol.Documentation{
				Identifier: identifier,
				SearchKey:  "", // don't index sections of documentation for search
				NewPage:    false,
				Tags:       visibilityTags,
			},
			Label:  protocol.NewMarkupContent(label, protocol.PlainText),
			Detail: protocol.NewMarkupContent("", protocol.PlainText),
		}).emit(d.i.emitter)
		_ = d.i.emitter.EmitDocumentationChildrenEdge(children, sectionID)
		return sectionID
	}

	var sections []uint64
	// Emit a "Constants" section
	sort.Slice(consts, func(i, j int) bool {
		return sortName("", "", consts[i].name, consts[j].name)
	})
	if len(consts) > 0 {
		var children []uint64
		for _, constDocs := range consts {
			children = append(children, constDocs.ID)
		}
		sections = append(sections, newSection("Constants", "const", children))
	}

	// Emit a "Variables" section
	sort.Slice(vars, func(i, j int) bool {
		return sortName("", "", vars[i].name, vars[j].name)
	})
	if len(vars) > 0 {
		var children []uint64
		for _, varDocs := range vars {
			children = append(children, varDocs.ID)
		}
		sections = append(sections, newSection("Variables", "var", children))
	}

	// Emit methods as children of their receiver types, functions as children of the type they
	// produce.
	sort.Slice(types, func(i, j int) bool {
		return sortName("", "", types[i].name, types[j].name)
	})
	sort.Slice(funcs, func(i, j int) bool {
		return sortName(funcs[i].recvTypeName, funcs[j].recvTypeName, funcs[i].name, funcs[j].name)
	})
	emittedMethods := map[uint64]struct{}{}
	for _, typeDocs := range types {
		var children []uint64
		for _, funcDocs := range funcs {
			if _, emitted := emittedMethods[funcDocs.ID]; emitted {
				continue
			}
			if funcDocs.recvType == nil {
				var matches int
				for _, resultTypeExpr := range funcDocs.resultTypes {
					resultType := p.TypesInfo.TypeOf(resultTypeExpr)
					if dereference(resultType) == dereference(typeDocs.typ) {
						matches++
					}
				}
				if matches == 1 {
					// The function is only a child of the type it produces if there was one match.
					// If it returned multiple types, better off keeping it separate from both.
					emittedMethods[funcDocs.ID] = struct{}{}
					children = append(children, funcDocs.ID)
				}
			}
		}
		for _, funcDocs := range funcs {
			if _, emitted := emittedMethods[funcDocs.ID]; emitted {
				continue
			}
			if funcDocs.recvType != nil {
				recvType := p.TypesInfo.TypeOf(funcDocs.recvType)
				if dereference(recvType) == dereference(typeDocs.typ) {
					emittedMethods[funcDocs.ID] = struct{}{}
					children = append(children, funcDocs.ID)
					continue
				}
			}
		}
		if len(children) > 0 {
			_ = d.i.emitter.EmitDocumentationChildrenEdge(children, typeDocs.ID)
		}
	}

	// Emit a "Types" section
	if len(types) > 0 {
		var children []uint64
		for _, typeDocs := range types {
			children = append(children, typeDocs.ID)
		}
		sections = append(sections, newSection("Types", "type", children))
	}

	// Emit a "Functions" section
	if len(funcs) > 0 {
		var children []uint64
		for _, funcDocs := range funcs {
			if _, emitted := emittedMethods[funcDocs.ID]; emitted {
				continue
			}
			children = append(children, funcDocs.ID)
		}
		if len(children) > 0 {
			sections = append(sections, newSection("Functions", "func", children))
		}
	}

	return docsPackage{
		ID:       packageDocsID,
		Path:     pkgPathStdStrip(p.PkgPath),
		emitted:  emitted,
		children: sections,
	}, nil
}

type fileDocs struct {
	// pkgDocsMarkdown describes package-level documentation found in the file.
	pkgDocsMarkdown string

	// Constants
	consts []constVarDocs

	// Variables
	vars []constVarDocs

	// Type documentation that was emitted.
	types []typeDocs

	// Function/method documentation that was emitted.
	funcs []funcDocs
}

// indexFile returns the documentation corresponding to the given file.
func (d *docsIndexer) indexFile(p *packages.Package, f *ast.File, fileName string, isTestFile bool) (fileDocs, error) {
	var result fileDocs
	result.pkgDocsMarkdown = godocToMarkdown(f.Doc.Text())

	// Collect each top-level declaration.
	var initIndex int = 1
	var mainIndex int = 1
	for _, decl := range f.Decls {
		switch node := decl.(type) {
		case *ast.GenDecl:
			genDeclDocs := d.indexGenDecl(p, f, node, isTestFile)
			result.consts = append(result.consts, genDeclDocs.consts...)
			result.vars = append(result.vars, genDeclDocs.vars...)
			result.types = append(result.types, genDeclDocs.types...)
		case *ast.FuncDecl:
			// Functions, methods
			if node.Name.Name == "_" {
				// Not only is it not exported, it cannot be referenced outside this package at all.
				continue
			}
			result.funcs = append(result.funcs, d.indexFuncDecl(p.Fset, p, node, fileName, &initIndex, &mainIndex, isTestFile))
		}
	}

	// Emit documentation for all constants.
	for i, constDocs := range result.consts {
		emittedID := constDocs.result().emit(d.i.emitter)
		constDocs.ID = emittedID
		result.consts[i] = constDocs
	}

	// Emit documentation for all variables.
	for i, varDocs := range result.vars {
		emittedID := varDocs.result().emit(d.i.emitter)
		varDocs.ID = emittedID
		result.vars[i] = varDocs
	}

	// Emit documentation for all types (struct/interface/other type definitions)
	for i, typeDocs := range result.types {
		emittedID := typeDocs.result().emit(d.i.emitter)
		typeDocs.ID = emittedID
		result.types[i] = typeDocs
	}

	// Emit documentation for all funcs/methods.
	for i, funcDocs := range result.funcs {
		emittedID := funcDocs.result().emit(d.i.emitter)
		funcDocs.ID = emittedID
		result.funcs[i] = funcDocs
	}
	return result, nil
}

type genDeclDocs struct {
	consts []constVarDocs
	vars   []constVarDocs
	types  []typeDocs
}

func (d *docsIndexer) indexGenDecl(p *packages.Package, f *ast.File, node *ast.GenDecl, isTestFile bool) genDeclDocs {
	var result genDeclDocs
	blockDocsMarkdown := godocToMarkdown(node.Doc.Text())

	// Each *ast.GenDecl node may contain multiple specs, e.g. one per variable in a var( ... )
	// block. const/type block. etc.
	for _, spec := range node.Specs {
		switch t := spec.(type) {
		case *ast.ValueSpec:
			// Variable or constant, potentially of the form `var x, y = 1, 2` - we emit each
			// separately.
			for i, name := range t.Names {
				if name.Name == "_" {
					// Not only is it not exported, it cannot be referenced outside this package at all.
					continue
				}
				switch node.Tok {
				case token.CONST:
					constDocs, ok := d.indexConstVar(p, t, i, "const", isTestFile)
					if ok {
						constDocs.docsMarkdown = blockDocsMarkdown + constDocs.docsMarkdown
						result.consts = append(result.consts, constDocs)
					}
				case token.VAR:
					varDocs, ok := d.indexConstVar(p, t, i, "var", isTestFile)
					if ok {
						varDocs.docsMarkdown = blockDocsMarkdown + varDocs.docsMarkdown
						result.vars = append(result.vars, varDocs)
					}
				}
			}
		case *ast.TypeSpec:
			if t.Name.Name == "_" {
				// Not only is it not exported, it cannot be referenced outside this package at all.
				continue
			}
			typeDocs, ok := d.indexTypeSpec(p, t, isTestFile)
			if ok {
				typeDocs.docsMarkdown = blockDocsMarkdown + typeDocs.docsMarkdown
				result.types = append(result.types, typeDocs)
			}
		}
	}
	return result
}

type constVarDocs struct {
	// The emitted "documentationResult" vertex ID.
	ID uint64

	// The best one-line label for this type we could come up with, e.g. `var x` omitting
	// the assignment.
	label string

	// The name of the const/var.
	name string

	// The search key for this const/var (see protocol.Documentation.SearchKey.)
	searchKey string

	// The full type signature, with docstrings on e.g. struct fields.
	signature string

	// Documentation strings in Markdown.
	docsMarkdown string

	// The definition object.
	def NoahObject

	// Associated tags
	tags []protocol.Tag
}

func (t constVarDocs) result() *documentationResult {
	// Include the full type signature
	var detail bytes.Buffer
	fmt.Fprintf(&detail, "```Go\n")
	fmt.Fprintf(&detail, "%s\n", t.signature)
	fmt.Fprintf(&detail, "```\n\n")
	fmt.Fprintf(&detail, "%s", t.docsMarkdown)

	return &documentationResult{
		Documentation: protocol.Documentation{
			Identifier: t.name,
			SearchKey:  t.searchKey,
			NewPage:    false,
			Tags:       t.tags,
		},
		Label:  protocol.NewMarkupContent(t.label, protocol.PlainText),
		Detail: protocol.NewMarkupContent(detail.String(), protocol.Markdown),
	}
}

func (d *docsIndexer) indexConstVar(p *packages.Package, in *ast.ValueSpec, nameIndex int, typ string, isTestFile bool) (constVarDocs, bool) {
	var result constVarDocs
	name := in.Names[nameIndex]
	result.label = fmt.Sprintf("%s %s", typ, name.String())
	result.name = name.String()
	result.searchKey = p.Name + "." + name.String()
	result.def = p.TypesInfo.Defs[name]
	if result.def == nil {
		// TODO(slimsag): some packages have illegally conflicting symbols, like:
		//
		// * testdata/conflicting_test_symbols/sandbox_unsupported.go:ErrNotImplemented
		// * testdata/conflicting_test_symbols/sandbox_unsupported_test.go:ErrNotImplemented
		//
		// These examples were pulled from the moby/moby project in the wild (so yes, it really
		// happens.) In such cases, p.TypesInfo.Defs here would be `nil`. One could argue this is
		// a bug in go/packages in which, because both files are analyzed, one identifier "wins"
		// (say, the test file's) while the other becomes `nil` - but I would argue it's not
		// go/packages responsibility necessarily to handle analyzing illegal Go code aside from in
		// a best-effort way, so we should handle this in a best-effort way, too: drop the symbol.
		return constVarDocs{}, false
	}

	result.tags = []protocol.Tag{}
	if typ == "const" {
		result.tags = append(result.tags, protocol.TagConstant)
	} else {
		result.tags = append(result.tags, protocol.TagVariable)
	}
	result.tags = append(result.tags, tagsForType(result.def.Type())...)
	if !ast.IsExported(name.String()) || isTestFile {
		result.tags = append(result.tags, protocol.TagPrivate)
	}
	if isDeprecated(in.Doc.Text()) {
		result.tags = append(result.tags, protocol.TagDeprecated)
	}

	// Produce the full type signature with docs on e.g. methods and struct fields, but not on the
	// type itself (we'll produce those as Markdown below.)
	cpy := *in
	cpy.Doc = nil
	result.signature = typ + " " + formatNode(p.Fset, &cpy)

	// TODO(slimsag): future: this is a HACK because some variables/constants are ultra long table
	// initializers, including those is not helpful to the user so we fallback in this case to
	// something much briefer.
	if len(result.signature) > 100 {
		cpy.Values = nil
		result.signature = typ + " " + formatNode(p.Fset, &cpy) + " = ..."
	}

	result.docsMarkdown = godocToMarkdown(in.Doc.Text())
	return result, true
}

type typeDocs struct {
	// The emitted "documentationResult" vertex ID.
	ID uint64

	// The best one-line label for this type we could come up with, e.g. `type foo struct` omitting
	// field names.
	label string

	// The name of the type.
	name string

	// The search key for this const/var (see protocol.Documentation.SearchKey.)
	searchKey string

	// The full type signature, with docstrings on e.g. methods and struct fields.
	signature string

	// Documentation strings in Markdown.
	docsMarkdown string

	// The type itself.
	typ types.Type

	// The definition object.
	def NoahObject

	// Associated tags
	tags []protocol.Tag
}

func (t typeDocs) result() *documentationResult {
	// Include the full type signature
	var detail bytes.Buffer
	fmt.Fprintf(&detail, "```Go\n")
	fmt.Fprintf(&detail, "%s\n", t.signature)
	fmt.Fprintf(&detail, "```\n\n")
	fmt.Fprintf(&detail, "%s", t.docsMarkdown)

	return &documentationResult{
		Documentation: protocol.Documentation{
			Identifier: t.name,
			SearchKey:  t.searchKey,
			NewPage:    false,
			Tags:       t.tags,
		},
		Label:  protocol.NewMarkupContent(t.label, protocol.PlainText),
		Detail: protocol.NewMarkupContent(detail.String(), protocol.Markdown),
	}
}

func (d *docsIndexer) indexTypeSpec(p *packages.Package, in *ast.TypeSpec, isTestFile bool) (typeDocs, bool) {
	if p.TypesInfo.TypeOf(in.Type) == nil || p.TypesInfo.ObjectOf(in.Name) == nil {
		// TODO(slimsag): It's unclear why, but exported types declared in a `package main` - such
		// as `type User struct` in minimal_main.go - do not have type information and would result
		// in a nil pointer deref below.
		//
		// Will need to investigate/fix this at a later point - it may be a bug in go/packages.
		// But, for now, at least don't panic - just discard it. This is a pretty rare occurrence
		// anyway, as an exported type from package main is also a linter error / non-idiomatic.
		return typeDocs{}, false
	}
	var result typeDocs
	result.label = fmt.Sprintf("type %s %s", in.Name.String(), formatTypeLabel(p.TypesInfo.TypeOf(in.Type)))
	result.name = in.Name.String()
	result.searchKey = p.Name + "." + in.Name.String()
	result.typ = p.TypesInfo.ObjectOf(in.Name).Type()
	result.def = p.TypesInfo.Defs[in.Name]

	result.tags = []protocol.Tag{}
	result.tags = append(result.tags, tagsForType(result.typ)...)
	if !ast.IsExported(in.Name.String()) || isTestFile {
		result.tags = append(result.tags, protocol.TagPrivate)
	}
	if isDeprecated(in.Doc.Text()) {
		result.tags = append(result.tags, protocol.TagDeprecated)
	}

	// Produce the full type signature with docs on e.g. methods and struct fields, but not on the
	// type itself (we'll produce those as Markdown below.)
	cpy := *in
	cpy.Doc = nil
	result.signature = "type " + formatNode(p.Fset, &cpy)

	result.docsMarkdown = godocToMarkdown(in.Doc.Text())
	return result, true
}

type funcDocs struct {
	// The emitted "documentationResult" vertex ID.
	ID uint64

	// The best one-line label for this type we could come up with, e.g. `func foo (e struct{...})`
	// omitting field names.
	label string

	// The full type signature, with docstrings on e.g. methods and struct fields.
	signature string

	// The name of the function.
	name string

	// The search key for this const/var (see protocol.Documentation.SearchKey.)
	searchKey string

	// Documentation strings in Markdown.
	docsMarkdown string

	// The type of the receiver, or nil.
	recvType ast.Expr

	// The name of the receiver type, or an empty string.
	recvTypeName string

	// The type of return values, or nil.
	resultTypes []ast.Expr

	// The definition object.
	def NoahObject

	// Associated tags
	tags []protocol.Tag
}

func (f funcDocs) result() *documentationResult {
	// Include the full type signature
	var detail strings.Builder
	detail.Grow(6 + len(f.signature) + len(f.docsMarkdown) + 6)
	detail.WriteString("```Go\n")
	detail.WriteString(f.signature)
	detail.WriteRune('\n')
	detail.WriteString("```\n\n")
	detail.WriteString(f.docsMarkdown)

	identifier := f.name
	if f.recvTypeName != "" {
		identifier = f.recvTypeName + "." + f.name
	}
	return &documentationResult{
		Documentation: protocol.Documentation{
			Identifier: identifier,
			SearchKey:  f.searchKey,
			NewPage:    false,
			Tags:       f.tags,
		},
		Label:  protocol.NewMarkupContent(f.label, protocol.PlainText),
		Detail: protocol.NewMarkupContent(detail.String(), protocol.Markdown),
	}
}

// indexFuncDecl returns the documentation corresponding to the given function declaration.
func (d *docsIndexer) indexFuncDecl(fset *token.FileSet, p *packages.Package, in *ast.FuncDecl, fileName string, initIndex *int, mainIndex *int, isTestFile bool) funcDocs {
	var result funcDocs
	result.name = in.Name.String()
	if result.name == "init" {
		// In a single Go package (and even in a single file), there may be multiple `init` functions.
		// We suffix them with the filename and a unique identifier number if there are more than one
		// per file, e.g. `init.mux.go.2`
		if *initIndex == 1 {
			result.name = fmt.Sprintf("%s.%s", result.name, fileName)
		} else {
			result.name = fmt.Sprintf("%s.%s.%v", result.name, fileName, *initIndex)
		}
		*initIndex++
	}
	if result.name == "main" {
		// In some repositories, there exist Go files which have multiple `main` functions inside
		// the same file. This is illegal and rejected by the Go compiler, yet it exists. It's bad
		// code, and would result in an error if compiled or passed through most tools. However, we
		// must not emit a duplicate path ID in such cases. Specifically because this would prevent
		// indexing the Go standard library due to e.g. https://raw.githubusercontent.com/golang/go/master/test/mainsig.go
		if *mainIndex > 1 {
			result.name = fmt.Sprintf("%s.%s.%v", result.name, fileName, *mainIndex)
		}
		*mainIndex++
	}
	result.searchKey = p.Name + "." + in.Name.String()
	result.docsMarkdown = godocToMarkdown(in.Doc.Text())
	result.def = p.TypesInfo.Defs[in.Name]

	private := !ast.IsExported(in.Name.String()) || isTestFile

	// Create a brand new FuncDecl based on the parts of the input we care about,
	// ignoring other aspects (e.g. docs and the function body, which are not needed to
	// produce the function signature.)
	fn := &ast.FuncDecl{Name: in.Name}

	// Receivers (e.g. struct methods.)
	if in.Recv != nil {
		fn.Recv = &ast.FieldList{List: make([]*ast.Field, 0, len(in.Recv.List))}
		for _, field := range in.Recv.List {
			fn.Recv.List = append(fn.Recv.List, &ast.Field{
				Type:  field.Type,
				Names: field.Names,
			})
		}
		if len(fn.Recv.List) > 0 {
			// Guaranteed to be length 1 for all valid Go programs, see https://golang.org/ref/spec#Receiver
			result.recvType = fn.Recv.List[0].Type

			// Mark functions as unexported if they are an exported method of a type that is
			// unexported.
			if named, ok := dereference(p.TypesInfo.TypeOf(result.recvType)).(*types.Named); ok {
				result.recvTypeName = named.Obj().Name()
				result.searchKey = p.Name + "." + result.recvTypeName + "." + in.Name.String()
				if !named.Obj().Exported() {
					private = true
				}
			}
		}
	}
	result.tags = []protocol.Tag{}
	if result.recvType != nil {
		result.tags = append(result.tags, protocol.TagMethod)
	} else {
		result.tags = append(result.tags, protocol.TagFunction)
	}
	if private {
		result.tags = append(result.tags, protocol.TagPrivate)
	}
	if isDeprecated(in.Doc.Text()) {
		result.tags = append(result.tags, protocol.TagDeprecated)
	}
	if strings.HasPrefix(result.name, "Test") && isTestFile {
		result.tags = append(result.tags, protocol.TagTest)
	}
	if strings.HasPrefix(result.name, "Benchmark") && isTestFile {
		result.tags = append(result.tags, protocol.TagBenchmark)
	}

	// Parameters.
	fn.Type = &ast.FuncType{}
	fn.Type.Params = &ast.FieldList{List: make([]*ast.Field, 0, len(in.Type.Params.List))}
	for _, field := range in.Type.Params.List {
		fn.Type.Params.List = append(fn.Type.Params.List, &ast.Field{
			Type:  field.Type,
			Names: field.Names,
		})
	}

	// Results.
	if in.Type.Results != nil {
		fn.Type.Results = &ast.FieldList{List: make([]*ast.Field, 0, len(in.Type.Results.List))}
		for _, field := range in.Type.Results.List {
			fn.Type.Results.List = append(fn.Type.Results.List, &ast.Field{
				Type:  field.Type,
				Names: field.Names,
			})
			result.resultTypes = append(result.resultTypes, field.Type)
		}
	}
	// TODO(slimsag): future: this doesn't format types very appropriately, some could span
	// multiple lines!
	result.label = formatNode(fset, fn)
	if lines := strings.Split(result.label, "\n"); len(lines) > 1 {
		result.label = lines[0] + "..." // To be fixed another day!
	}

	// Produce the full type signature with docs on e.g. anonymous struct fields, multi-line parameters,
	// etc. but not on the function itself (we'll produce those as Markdown below.)
	cpy := *in
	cpy.Doc = nil
	cpy.Body = nil
	result.signature = formatNode(fset, &cpy)

	return result
}

func tagsForType(t types.Type) []protocol.Tag {
	return doTagsForType(nil, t)
}

func doTagsForType(visited []types.Type, t types.Type) []protocol.Tag {
	// Go types can be cyclic, make sure we don't get stuck in a loop by keeping track of what we've visited.
	for _, visited := range visited {
		if visited == t {
			return nil
		}
	}
	visited = append(visited, t)
	switch v := t.(type) {
	case *types.Array:
		return append([]protocol.Tag{protocol.TagArray}, doTagsForType(visited, v.Elem())...)
	case *types.Slice:
		// TODO(slimsag): add a "slice" tag?
		return append([]protocol.Tag{protocol.TagArray}, doTagsForType(visited, v.Elem())...)
	case *types.Chan:
		// TODO(slimsag): add a "channel" tag?
		return nil
	case *types.Interface:
		return []protocol.Tag{protocol.TagInterface}
	case *types.Map:
		// TODO(slimsag): is "object" really a good tag for "map" or "dictionary"?
		return []protocol.Tag{protocol.TagObject}
	case *types.Pointer:
		// TODO(slimsag): add a "pointer" tag?
		return doTagsForType(visited, v.Elem()) // "***int32" is a "number" I guess
	case *types.Struct:
		return []protocol.Tag{protocol.TagStruct}
	case *types.Named:
		return doTagsForType(visited, v.Underlying())
	case *types.Basic:
		switch v.Kind() {
		case types.Bool, types.UntypedBool:
			return []protocol.Tag{protocol.TagBoolean}
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Uintptr,
			types.Float32, types.Float64,
			types.Complex64, types.Complex128,
			types.UnsafePointer,
			types.UntypedInt, types.UntypedRune, types.UntypedFloat, types.UntypedComplex:
			return []protocol.Tag{protocol.TagNumber}
		case types.String, types.UntypedString:
			return []protocol.Tag{protocol.TagString}
		case types.UntypedNil:
			return []protocol.Tag{protocol.TagNull}
		case types.Invalid:
			return nil
		default:
			panic(fmt.Sprintf("invariant: unknown kind %v", v))
		}
	case *types.Signature:
		if v.Recv() != nil {
			return []protocol.Tag{protocol.TagMethod}
		}
		return []protocol.Tag{protocol.TagFunction}
	case *types.Tuple:
		return nil // parameter arguments / receiver tuples
	default:
		panic(fmt.Sprintf("invariant: unknown type %T", t))
	}
}

// formatNode turns an ast.Node into a string.
func formatNode(fset *token.FileSet, node interface{}) string {
	var buf bytes.Buffer
	err := format.Node(&buf, fset, node)
	if err != nil {
		panic(fmt.Sprintf("never here: %v", err))
	}
	return buf.String()
}

// formatTypeLabel formats the given type as a one-line label, omitting certain details like struct
// fields.
func formatTypeLabel(t types.Type) string {
	switch v := t.(type) {
	case *types.Array:
		return fmt.Sprintf("[%d]%s", v.Len(), formatTypeLabel(v.Elem()))
	case *types.Slice:
		return fmt.Sprintf("[]%s", formatTypeLabel(v.Elem()))
	case *types.Chan:
		var dir string
		switch v.Dir() {
		case types.SendRecv:
			dir = "chan"
		case types.SendOnly:
			dir = "chan<-"
		case types.RecvOnly:
			dir = "<-chan"
		default:
			panic("never here")
		}
		return fmt.Sprintf("%s %s", dir, formatTypeLabel(v.Elem()))
	case *types.Interface:
		if v.Empty() {
			return "interface{}"
		}
		return "interface"
	case *types.Map:
		return fmt.Sprintf("map[%s]%s", formatTypeLabel(v.Key()), formatTypeLabel(v.Elem()))
	case *types.Pointer:
		return fmt.Sprintf("*%s", formatTypeLabel(v.Elem()))
	case *types.Struct:
		if v.NumFields() == 0 {
			return "struct{}"
		}
		return "struct"
	case *types.Named:
		typeName := v.Obj()
		if typeName.Pkg() == nil {
			// e.g. builtin `error` interface.
			return typeName.Name()
		}
		return fmt.Sprintf("%s.%s", typeName.Pkg().Name(), typeName.Name())
	case *types.Basic, *types.Signature, *types.Tuple:
		return v.String()
	default:
		return v.String()
	}
}

// isDeprecated tells if the given docstring has any line beginning with "deprecated",
// "Deprecated", or "DEPRECATED".
func isDeprecated(docstring string) bool {
	for _, line := range strings.Split(docstring, "\n") {
		if strings.HasPrefix(strings.ToLower(line), "deprecated") {
			return true
		}
	}
	return false
}

func sortName(aRecv, bRecv, a, b string) bool {
	if aRecv != bRecv {
		return sortExportedNameFirst(aRecv, bRecv)
	}
	return sortExportedNameFirst(a, b)
}

func sortExportedNameFirst(a, b string) bool {
	aExported, bExported := false, false
	aUnderscore, bUnderscore := false, false
	if a != "" {
		aExported = !unicode.IsLower([]rune(a)[0])
		aUnderscore = a[0] == '_'
	}
	if b != "" {
		bExported = !unicode.IsLower([]rune(b)[0])
		bUnderscore = b[0] == '_'
	}
	if aUnderscore != bUnderscore {
		if !aUnderscore {
			return true // Names not starting with "_" have higher importance
		}
		return false
	}
	if aExported != bExported {
		if aExported {
			return true
		}
		return false
	}
	return a < b
}

func dereference(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return dereference(p.Elem())
	}
	return t
}

func godocToMarkdown(godoc string) string {
	var buf bytes.Buffer
	doc.ToMarkdown(&buf, godoc, nil)
	return buf.String()
}

// documentationResult is a simple emitter of complete documentationResults.
//
// Advanced usages should just emit the vertices/edges on their own instead
// of using this helper.
type documentationResult struct {
	Documentation protocol.Documentation
	Label, Detail protocol.MarkupContent
}

// emit emits:
//
// * The "documentationResult" vertex corresponding to d.Documentation
// * The "documentationString" vertex corresponding to d.Label
// * The "documentationString" vertex corresponding to d.Detail
// * The "documentationString" edge attaching d.Label to d.Documentation.
// * The "documentationString" edge attaching d.Detail to d.Documentation.
//
// Returned is the ID of the "documentationResult" vertex.
func (d *documentationResult) emit(emitter *writer.Emitter) uint64 {
	documentationResultID := emitter.EmitDocumentationResult(d.Documentation)
	labelID := emitter.EmitDocumentationString(d.Label)
	detailID := emitter.EmitDocumentationString(d.Detail)
	_ = emitter.EmitDocumentationStringEdge(labelID, documentationResultID, protocol.DocumentationStringKindLabel)
	_ = emitter.EmitDocumentationStringEdge(detailID, documentationResultID, protocol.DocumentationStringKindDetail)
	return documentationResultID
}
