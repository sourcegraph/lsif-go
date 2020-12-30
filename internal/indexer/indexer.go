package indexer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"go/types"
	"log"
	"strings"
	"sync"

	"github.com/pkg/errors"
	protocol "github.com/sourcegraph/lsif-protocol"
	"github.com/sourcegraph/lsif-protocol/writer"
	"golang.org/x/tools/go/packages"
)

type Indexer struct {
	repositoryRoot string            // path to repository
	projectRoot    string            // path to package
	toolInfo       protocol.ToolInfo // metadata vertex payload
	moduleName     string            // name of this module
	moduleVersion  string            // version of this module
	dependencies   map[string]string // parsed module data
	emitter        *writer.Emitter   // LSIF data emitter
	outputOptions  OutputOptions     // What to print to stdout/stderr

	// Definition type cache
	consts  map[interface{}]*DefinitionInfo // position -> info
	funcs   map[interface{}]*DefinitionInfo // name -> info
	imports map[interface{}]*DefinitionInfo // position -> info
	labels  map[interface{}]*DefinitionInfo // position -> info
	types   map[interface{}]*DefinitionInfo // name -> info
	vars    map[interface{}]*DefinitionInfo // position -> info

	// LSIF data cache
	documents             map[string]*DocumentInfo  // filename -> info
	ranges                map[string]map[int]uint64 // filename -> offset -> rangeID
	hoverResultCache      map[string]uint64         // cache key -> hoverResultID
	packageInformationIDs map[string]uint64         // name -> packageInformationID
	packageDataCache      *PackageDataCache         // hover text and moniker path cache
	packages              []*packages.Package       // index target packages
	projectID             uint64                    // project vertex identifier
	packagesByFile        map[string][]*packages.Package

	constsMutex                sync.Mutex
	funcsMutex                 sync.Mutex
	importsMutex               sync.Mutex
	labelsMutex                sync.Mutex
	typesMutex                 sync.Mutex
	varsMutex                  sync.Mutex
	stripedMutex               *StripedMutex
	hoverResultCacheMutex      sync.RWMutex
	packageInformationIDsMutex sync.RWMutex
}

func New(
	repositoryRoot string,
	projectRoot string,
	toolInfo protocol.ToolInfo,
	moduleName string,
	moduleVersion string,
	dependencies map[string]string,
	jsonWriter writer.JSONWriter,
	packageDataCache *PackageDataCache,
	outputOptions OutputOptions,
) *Indexer {
	return &Indexer{
		repositoryRoot:        repositoryRoot,
		projectRoot:           projectRoot,
		toolInfo:              toolInfo,
		moduleName:            moduleName,
		moduleVersion:         moduleVersion,
		dependencies:          dependencies,
		emitter:               writer.NewEmitter(jsonWriter),
		outputOptions:         outputOptions,
		consts:                map[interface{}]*DefinitionInfo{},
		funcs:                 map[interface{}]*DefinitionInfo{},
		imports:               map[interface{}]*DefinitionInfo{},
		labels:                map[interface{}]*DefinitionInfo{},
		types:                 map[interface{}]*DefinitionInfo{},
		vars:                  map[interface{}]*DefinitionInfo{},
		documents:             map[string]*DocumentInfo{},
		ranges:                map[string]map[int]uint64{},
		hoverResultCache:      map[string]uint64{},
		packageInformationIDs: map[string]uint64{},
		packageDataCache:      packageDataCache,
		stripedMutex:          newStripedMutex(),
	}
}

// Index generates an LSIF dump from a workspace by traversing through source files
// and writing the LSIF equivalent to the output source that implements io.Writer.
// It is caller's responsibility to close the output source if applicable.
func (i *Indexer) Index() error {
	if err := i.loadPackages(); err != nil {
		return errors.Wrap(err, "loadPackages")
	}

	i.emitMetadataAndProjectVertex()
	i.emitDocuments()
	i.addImports()
	i.indexSymbols() // first because symbol ranges need to be emitted w/"tag" properties
	i.indexDefinitions()
	i.indexReferences()
	i.linkReferenceResultsToRanges()
	i.emitContains()

	if err := i.emitter.Flush(); err != nil {
		return errors.Wrap(err, "emitter.Flush")
	}

	return nil
}

var loadMode = packages.NeedDeps | packages.NeedFiles | packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedName

// packages populates the packages field containing an AST for each package within the configured
// project root.
func (i *Indexer) loadPackages() error {
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(errs)

		config := &packages.Config{
			Mode:  loadMode,
			Dir:   i.projectRoot,
			Tests: true,
			Logf:  i.packagesLoadLogger,
		}

		pkgs, err := packages.Load(config, "./...")
		if err != nil {
			errs <- errors.Wrap(err, "packages.Load")
			return
		}

		i.packages = pkgs
		i.packagesByFile = map[string][]*packages.Package{}

		for _, p := range i.packages {
			for _, f := range p.Syntax {
				filename := p.Fset.Position(f.Package).Filename
				i.packagesByFile[filename] = append(i.packagesByFile[filename], p)
			}
		}
	}()

	withProgress(&wg, "Loading packages", i.outputOptions, nil, 0)
	return <-errs
}

// packagesLoadLogger logs the debug messages from the packages.Load function.
//
// We only care about one message, which contains the output of the `go list`
// command. In order to determine what relevant data we should print, we try to
// unmarshal the fourth log argument value (a *bytes.Buffer of go list stdout)
// as a stream of JSON objects representing loaded (or candidate) packages.
func (i *Indexer) packagesLoadLogger(format string, args ...interface{}) {
	if i.outputOptions.Verbosity < VeryVeryVerboseOutput || len(args) < 4 {
		return
	}
	stdoutBuf, ok := args[3].(*bytes.Buffer)
	if !ok {
		return
	}

	var payload struct {
		ImportPath string `json:"ImportPath"`
	}

	for decoder := json.NewDecoder(strings.NewReader(stdoutBuf.String())); decoder.Decode(&payload) == nil; {
		log.Printf("\tPackage %s", payload.ImportPath)
	}
}

// emitMetadata emits a metadata and project vertex. This method returns the identifier of the project
// vertex, which is needed to construct the project/document contains relation later.
func (i *Indexer) emitMetadataAndProjectVertex() {
	i.emitter.EmitMetaData("file://"+i.repositoryRoot, i.toolInfo)
	i.projectID = i.emitter.EmitProject(languageGo)
}

// emitDocuments emits a document vertex for each file for the loaded packages and a contains relation
// to the project vertex emitted by emitMetadataAndProjectVertex. This also adds entries into the files
// and ranges map for each document. Methods should skip any document that does not have a file entry as
// it may fall outside of the project root (and is thus not properly indexable).
func (i *Indexer) emitDocuments() {
	i.visitEachRawFile("Emitting documents", i.emitDocument)
}

// emitDocument emits a document vertex and a contains relation to the enclosing project. This method
// also prepares the documents and ranges maps (alternatively: this method must be called before any
// other method that requires the filename key be present in either map).
func (i *Indexer) emitDocument(filename string) {
	// Emit each document only once
	if _, ok := i.documents[filename]; ok {
		return
	}

	// Indexing test files means that we're also indexing the code _generated_ by go test;
	// e.g. file://Users/efritz/Library/Caches/go-build/07/{64-character identifier}-d. Skip
	// These files as they won't be navigable outside of the machine that indexed the project.
	if !strings.HasPrefix(filename, i.projectRoot) {
		return
	}

	documentID := i.emitter.EmitDocument(languageGo, filename)
	i.documents[filename] = &DocumentInfo{DocumentID: documentID}
	i.ranges[filename] = map[int]uint64{}
}

// addImports modifies the definitions map of each file to include entries for import statements so
// they can be indexed uniformly in subsequent steps.
func (i *Indexer) addImports() {
	i.visitEachPackage("Adding import definitions", i.addImportsToPackage)
}

// addImportsToFile modifies the definitions map of the given file to include entries for import
// statements so they can be indexed uniformly in subsequent steps.
func (i *Indexer) addImportsToPackage(p *packages.Package) {
	for _, f := range p.Syntax {
		for _, spec := range f.Imports {
			pkg := p.Imports[strings.Trim(spec.Path.Value, `"`)]
			if pkg == nil {
				continue
			}

			name := importSpecName(spec)
			ident := &ast.Ident{NamePos: spec.Pos(), Name: name, Obj: ast.NewObj(ast.Pkg, name)}
			p.TypesInfo.Defs[ident] = types.NewPkgName(spec.Pos(), p.Types, name, pkg.Types)
		}
	}
}

// importSpecName extracts the name from the given import spec.
func importSpecName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.String()
	}

	return spec.Path.Value
}

// getAllReferencedPackages returns a slice of packages containing the index target packages
// as well as each directly imported package (but no transitively imported packages). The
// resulting slice contains no duplicates.
func getAllReferencedPackages(pkgs []*packages.Package) (flattened []*packages.Package) {
	allPackages := map[*packages.Package]struct{}{}
	for _, p := range pkgs {
		allPackages[p] = struct{}{}

		for _, i := range p.Imports {
			allPackages[i] = struct{}{}
		}
	}

	for pkg := range allPackages {
		flattened = append(flattened, pkg)
	}

	return flattened
}

// indexSymbols indexes data for each symbol in the project.
func (i *Indexer) indexSymbols() {
	i.visitEachPackage("Indexing symbols", i.indexSymbolsForPackage)
}

// indexSymbolsForPackage indexes data for each symbol in the given package.
func (i *Indexer) indexSymbolsForPackage(p *packages.Package) {
	// TODO(sqs): HACK, the loader returns 4 packages because (loader.Config).Tests==true and we
	// want to avoid duplication.
	if !strings.HasSuffix(p.Name, "_test") && strings.HasSuffix(p.ID, ".test]") {
		return
	}

	// Omit files (such as those generated by `go test`) that aren't in the project root because
	// those are not externally accessible.
	files := make([]*ast.File, 0, len(p.Syntax))
	for _, file := range p.Syntax {
		path := p.Fset.Position(file.Pos()).Filename
		if strings.HasPrefix(path, i.projectRoot) {
			files = append(files, file)
		}
	}

	packageSymbolID := i.emitter.EmitSymbol(
		protocol.SymbolData{
			Text:   p.Name,
			Detail: p.PkgPath,
			Kind:   protocol.Package,
		},
		nil, // TODO(sqs): include all package files?
	)
	i.emitExportMoniker(packageSymbolID, p, types.NewPkgName(0, p.Types, p.PkgPath, p.Types))
	_ = i.emitter.EmitWorkspaceSymbolEdge(i.projectID, []uint64{packageSymbolID})

	docpkg, err := doc.NewFromFiles(p.Fset, files, p.PkgPath /* TODO(sqs): doc.AllDecls|*/, doc.PreserveAST)
	if err != nil {
		panic(err)
	}

	symbolsByDocument := make(map[*DocumentInfo][]protocol.RangeBasedDocumentSymbol, len(files))
	emitAndRecordSymbol := func(symbol protocol.RangeSymbolTag, nameNode ast.Node, parent uint64, children []uint64) uint64 {
		// TODO(sqs): can rearrange locks to spend less time holding lock
		pos := p.Fset.Position(nameNode.Pos())
		i.stripedMutex.LockKey(pos.Filename)
		defer i.stripedMutex.UnlockKey(pos.Filename)

		if _, ok := i.ranges[pos.Filename][pos.Offset]; ok {
			panic(fmt.Sprintf("range already exists: %+v", symbol))
		}

		rng := rangeForNode(p.Fset, nameNode)
		rangeID := i.emitter.EmitRangeWithTag(rng.Start, rng.End, &symbol)

		i.ranges[pos.Filename][pos.Offset] = rangeID

		d, ok := i.documents[pos.Filename]
		if !ok {
			panic("filename not found: " + pos.Filename) // TODO(sqs)
		}

		d.m.Lock()
		d.SymbolRangeIDs = append(d.SymbolRangeIDs, rangeID)
		d.m.Unlock()

		if parent == packageSymbolID {
			ds := protocol.RangeBasedDocumentSymbol{
				ID: rangeID,
			}
			ds.Children = make([]protocol.RangeBasedDocumentSymbol, len(children))
			for i, childID := range children {
				ds.Children[i].ID = childID
			}
			symbolsByDocument[d] = append(symbolsByDocument[d], ds)
		}

		if parent != 0 {
			_ = i.emitter.EmitMember(parent, []uint64{rangeID})
		}

		return rangeID
	}

	newConstSymbol := func(o *doc.Value) (protocol.RangeSymbolTag, ast.Node) {
		// TODO(sqs): narrow ranges in GenDecl (multi-const decl)
		fullRange := rangeForNode(p.Fset, o.Decl)
		return protocol.RangeSymbolTag{
			Type: "definition",
			SymbolData: protocol.SymbolData{
				Text: o.Names[0], // TODO(sqs): emit all names
				Kind: protocol.Constant,
			},
			FullRange: &fullRange,
		}, o.Decl.Specs[0].(*ast.ValueSpec).Names[0]
	}
	newFuncSymbol := func(o *doc.Func) (protocol.RangeSymbolTag, ast.Node) {
		var kind protocol.SymbolKind
		if o.Recv == "" {
			kind = protocol.Function
		} else {
			kind = protocol.Method
		}
		fullRange := rangeForNode(p.Fset, o.Decl)
		return protocol.RangeSymbolTag{
			Type: "definition",
			SymbolData: protocol.SymbolData{
				Text: o.Name,
				Kind: kind,
			},
			FullRange: &fullRange,
		}, o.Decl.Name
	}
	newTypeSymbol := func(o *doc.Type) (protocol.RangeSymbolTag, ast.Node) {
		// TODO(sqs): narrow down type ranges
		fullRange := rangeForNode(p.Fset, o.Decl)
		return protocol.RangeSymbolTag{
			Type: "definition",
			SymbolData: protocol.SymbolData{
				Text: o.Name,
				Kind: protocol.Interface, // TODO(sqs): differentiate between interface/struct/etc.
			},
			FullRange: &fullRange,
		}, o.Decl.Specs[0].(*ast.TypeSpec).Name
	}
	newVarSymbol := func(o *doc.Value) (protocol.RangeSymbolTag, ast.Node) {
		// TODO(sqs): narrow down ranges
		fullRange := rangeForNode(p.Fset, o.Decl)
		return protocol.RangeSymbolTag{
			Type: "definition",
			SymbolData: protocol.SymbolData{
				Text: o.Names[0], // TODO(sqs): emit all names
				Kind: protocol.Variable,
			},
			FullRange: &fullRange,
		}, o.Decl.Specs[0].(*ast.ValueSpec).Names[0]
	}

	for _, o := range docpkg.Consts {
		symbol, node := newConstSymbol(o)
		emitAndRecordSymbol(symbol, node, packageSymbolID, nil)
	}

	for _, o := range docpkg.Funcs {
		symbol, node := newFuncSymbol(o)
		emitAndRecordSymbol(symbol, node, packageSymbolID, nil)
	}

	for _, o := range docpkg.Types {
		symbol, node := newTypeSymbol(o)

		var children []uint64
		for _, c := range o.Consts {
			childSymbol, node := newConstSymbol(c)
			children = append(children, emitAndRecordSymbol(childSymbol, node, 0, nil))
		}
		for _, c := range o.Funcs {
			childSymbol, node := newFuncSymbol(c)
			children = append(children, emitAndRecordSymbol(childSymbol, node, 0, nil))
		}
		for _, c := range o.Methods {
			childSymbol, node := newFuncSymbol(c)
			children = append(children, emitAndRecordSymbol(childSymbol, node, 0, nil))
		}
		for _, c := range o.Vars {
			childSymbol, node := newVarSymbol(c)
			children = append(children, emitAndRecordSymbol(childSymbol, node, 0, nil))
		}

		id := emitAndRecordSymbol(symbol, node, packageSymbolID, children)
		_ = i.emitter.EmitMember(id, children)
	}

	for _, o := range docpkg.Vars {
		symbol, node := newVarSymbol(o)
		emitAndRecordSymbol(symbol, node, packageSymbolID, nil)
	}

	for d, symbols := range symbolsByDocument {
		resultID := i.emitter.EmitDocumentSymbolResult(symbols)
		_ = i.emitter.EmitTextDocumentDocumentSymbolEdge(d.DocumentID, resultID)
	}
}

// indexDefinitions emits data for each definition in an index target package. This will emit
// a result set, a definition result, a hover result, and export monikers attached to each range.
// This method will also populate each document's definition range identifier slice.
func (i *Indexer) indexDefinitions() {
	i.visitEachPackage("Indexing definitions", i.indexDefinitionsForPackage)
}

// indexDefinitionsForPackage emits data for each definition within the given package.
func (i *Indexer) indexDefinitionsForPackage(p *packages.Package) {
	// Create a map of implicit case clause objects by their position. Note that there is an
	// implicit object for each case clause of a type switch (including default), and they all
	// share the same position. This creates a map with one arbitrarily chosen argument for
	// each distinct type switch.
	caseClauses := map[token.Pos]types.Object{}
	for node, obj := range p.TypesInfo.Implicits {
		if _, ok := node.(*ast.CaseClause); ok {
			caseClauses[obj.Pos()] = obj
		}
	}

	for ident, obj := range p.TypesInfo.Defs {
		typeSwitchHeader := false
		if obj == nil {
			// The definitions map contains nil objects for symbolic variables t in t := x.(type)
			// of type switch headers. In these cases we select an arbitrary case clause for the
			// same type switch to index the definition. We mark this object as a typeSwitchHeader
			// so that it can distinguished from other definitions with non-nil objects.
			caseClause, ok := caseClauses[ident.Pos()]
			if !ok {
				continue
			}

			obj = caseClause
			typeSwitchHeader = true
		}

		pos, d, ok := i.positionAndDocument(p, obj.Pos())
		if !ok {
			continue
		}

		rangeID := i.indexDefinition(p, pos.Filename, d, pos, obj, typeSwitchHeader)

		d.m.Lock()
		d.DefinitionRangeIDs = append(d.DefinitionRangeIDs, rangeID)
		d.m.Unlock()
	}
}

// positionAndDocument returns the position of the given object and the document info object
// that contains it. If the given package is not the canonical package for the containing file
// in the packagesByFile map, this method returns false.
func (i *Indexer) positionAndDocument(p *packages.Package, pos token.Pos) (token.Position, *DocumentInfo, bool) {
	position := p.Fset.Position(pos)

	if packages := i.packagesByFile[position.Filename]; len(packages) == 0 || packages[0] != p {
		return token.Position{}, nil, false
	}

	d, hasDocument := i.documents[position.Filename]
	if !hasDocument {
		return token.Position{}, nil, false
	}

	return position, d, true
}

// indexDefinition emits data for the given definition object.
func (i *Indexer) indexDefinition(p *packages.Package, filename string, document *DocumentInfo, pos token.Position, obj types.Object, typeSwitchHeader bool) uint64 {
	rangeID := i.ensureRangeFor(pos, obj)
	resultSetID := i.emitter.EmitResultSet()
	defResultID := i.emitter.EmitDefinitionResult()

	_ = i.emitter.EmitNext(rangeID, resultSetID)
	_ = i.emitter.EmitTextDocumentDefinition(resultSetID, defResultID)
	_ = i.emitter.EmitItem(defResultID, []uint64{rangeID}, document.DocumentID)

	if typeSwitchHeader {
		// TODO(efritz) - not sure how to document a type switch header symbolic variable
		// I'd like to somehow keep the type string of the RHS, but I'm not yet sure how
		// to resolve that data given what we have.
	} else {
		// Create a hover result vertex and cache the result identifier keyed by the definition location.
		// Caching this gives us a big win for package documentation, which is likely to be large and is
		// repeated at each import and selector within referenced files.
		_ = i.emitter.EmitTextDocumentHover(resultSetID, i.makeCachedHoverResult(nil, obj, func() []protocol.MarkedString {
			return findHoverContents(i.packageDataCache, i.packages, p, obj)
		}))
	}

	if _, ok := obj.(*types.PkgName); ok {
		i.emitImportMoniker(resultSetID, p, obj)
	}

	if obj.Exported() {
		i.emitExportMoniker(resultSetID, p, obj)
	}

	i.setDefinitionInfo(obj, &DefinitionInfo{
		DocumentID:        document.DocumentID,
		RangeID:           rangeID,
		ResultSetID:       resultSetID,
		ReferenceRangeIDs: map[uint64][]uint64{},
		TypeSwitchHeader:  typeSwitchHeader,
	})

	return rangeID
}

// setDefinitionInfo stashes the given definition info indexed by the given object type and name.
// This definition info will be accessible by invoking getDefinitionInfo with the same type and
// name values (but not necessarily the same object).
func (i *Indexer) setDefinitionInfo(obj types.Object, d *DefinitionInfo) {
	switch v := obj.(type) {
	case *types.Const:
		i.constsMutex.Lock()
		i.consts[obj.Pos()] = d
		i.constsMutex.Unlock()

	case *types.Func:
		i.funcsMutex.Lock()
		i.funcs[v.FullName()] = d
		i.funcsMutex.Unlock()

	case *types.Label:
		i.labelsMutex.Lock()
		i.labels[obj.Pos()] = d
		i.labelsMutex.Unlock()

	case *types.PkgName:
		i.importsMutex.Lock()
		i.imports[obj.Pos()] = d
		i.importsMutex.Unlock()

	case *types.TypeName:
		i.typesMutex.Lock()
		i.types[obj.Type().String()] = d
		i.typesMutex.Unlock()

	case *types.Var:
		i.varsMutex.Lock()
		i.vars[obj.Pos()] = d
		i.varsMutex.Unlock()
	}
}

// indexReferences emits data for each reference in an index target package. This will attach
// the range to a local definition (if one exists), or will emit a result set, a reference result,
// a hover result, and import monikers (for external definitions). This method will also populate
// each document's reference range identifier slice.
func (i *Indexer) indexReferences() {
	i.visitEachPackage("Indexing references", i.indexReferencesForPackage)
}

// indexReferencesForPackage emits data for each reference within the given package.
func (i *Indexer) indexReferencesForPackage(p *packages.Package) {
	for ident, definitionObj := range p.TypesInfo.Uses {
		if definitionObj == nil {
			continue
		}

		pos, d, ok := i.positionAndDocument(p, ident.Pos())
		if !ok {
			continue
		}

		rangeID, ok := i.indexReference(p, d, pos, definitionObj)
		if !ok {
			continue
		}

		d.m.Lock()
		d.ReferenceRangeIDs = append(d.ReferenceRangeIDs, rangeID)
		d.m.Unlock()
	}
}

// indexReference emits data for the given reference object.
func (i *Indexer) indexReference(p *packages.Package, document *DocumentInfo, pos token.Position, definitionObj types.Object) (uint64, bool) {
	if def := i.getDefinitionInfo(definitionObj); def != nil {
		return i.indexReferenceToDefinition(p, document, pos, definitionObj, def)
	}

	return i.indexReferenceToExternalDefinition(p, document, pos, definitionObj)
}

// getDefinitionInfo returns the definition info object for the given object. This requires that
// setDefinitionInfo was previously called an object that can be resolved in the same way. This
// will only return definitions which are defined in an index target (not a dependency).
func (i *Indexer) getDefinitionInfo(obj types.Object) *DefinitionInfo {
	switch v := obj.(type) {
	case *types.Const:
		return i.consts[v.Pos()]
	case *types.Func:
		return i.funcs[v.FullName()]
	case *types.Label:
		return i.labels[v.Pos()]
	case *types.PkgName:
		return i.imports[v.Pos()]
	case *types.TypeName:
		return i.types[obj.Type().String()]
	case *types.Var:
		return i.vars[v.Pos()]
	}

	return nil
}

// indexReferenceToDefinition emits data for the given reference object that is defined within
// an index target package.
func (i *Indexer) indexReferenceToDefinition(p *packages.Package, document *DocumentInfo, pos token.Position, definitionObj types.Object, d *DefinitionInfo) (uint64, bool) {
	rangeID := i.ensureRangeFor(pos, definitionObj)
	_ = i.emitter.EmitNext(rangeID, d.ResultSetID)

	d.m.Lock()
	d.ReferenceRangeIDs[document.DocumentID] = append(d.ReferenceRangeIDs[document.DocumentID], rangeID)
	d.m.Unlock()

	if d.TypeSwitchHeader {
		// Attache a hover text result _directly_ to the given range so that it "overwrites" the
		// hover result of the type switch header for this use. Each reference of such a variable
		// will need a more specific hover text, as the type of the variable is refined in the body
		// of case clauses of the type switch.
		_ = i.emitter.EmitTextDocumentHover(rangeID, i.makeCachedHoverResult(nil, definitionObj, func() []protocol.MarkedString {
			return findHoverContents(i.packageDataCache, i.packages, p, definitionObj)
		}))
	}

	return rangeID, true
}

// indexReferenceToExternalDefinition emits data for the given reference object that is not defined
// within an index target package. This definition _may_ be resolvable by scanning dependencies, but
// it is not guaranteed.
func (i *Indexer) indexReferenceToExternalDefinition(p *packages.Package, document *DocumentInfo, pos token.Position, definitionObj types.Object) (uint64, bool) {
	definitionPkg := definitionObj.Pkg()
	if definitionPkg == nil {
		return 0, false
	}

	// Create a or retreive a hover result identifier keyed by the target object's identifier
	// (scoped ot the object's package name). Caching this gives us another big win as some
	// methods imported from other packages are likely to be used many times in a dependent
	// project (e.g., context.Context, http.Request, etc).
	hoverResultID := i.makeCachedHoverResult(definitionPkg, definitionObj, func() []protocol.MarkedString {
		return findExternalHoverContents(i.packageDataCache, i.packages, p, definitionObj)
	})

	rangeID := i.ensureRangeFor(pos, definitionObj)
	refResultID := i.emitter.EmitReferenceResult()
	_ = i.emitter.EmitTextDocumentReferences(rangeID, refResultID)
	_ = i.emitter.EmitItemOfReferences(refResultID, []uint64{rangeID}, document.DocumentID)

	if hoverResultID != 0 {
		_ = i.emitter.EmitTextDocumentHover(rangeID, hoverResultID)
	}

	i.emitImportMoniker(rangeID, p, definitionObj)
	return rangeID, true
}

// ensureRangeFor returns a range identifier for the given object. If a range for the object has
// not been emitted, a new vertex is created.
func (i *Indexer) ensureRangeFor(pos token.Position, obj types.Object) uint64 {
	i.stripedMutex.RLockKey(pos.Filename)
	rangeID, ok := i.ranges[pos.Filename][pos.Offset]
	i.stripedMutex.RUnlockKey(pos.Filename)
	if ok {
		return rangeID
	}

	// Note: we calculate this outside of the critical section
	start, end := rangeForObject(obj, pos)

	i.stripedMutex.LockKey(pos.Filename)
	defer i.stripedMutex.UnlockKey(pos.Filename)

	if rangeID, ok := i.ranges[pos.Filename][pos.Offset]; ok {
		return rangeID
	}

	rangeID = i.emitter.EmitRange(start, end)
	i.ranges[pos.Filename][pos.Offset] = rangeID
	return rangeID
}

// linkReferenceResultsToRanges emits item relations for each indexed definition result value.
func (i *Indexer) linkReferenceResultsToRanges() {
	i.visitEachDefinitionInfo("Linking items to definitions", i.linkItemsToDefinitions)
}

// linkItemsToDefinitions adds item relations between the given definition range and the ranges that
// define and reference it.
func (i *Indexer) linkItemsToDefinitions(d *DefinitionInfo) {
	refResultID := i.emitter.EmitReferenceResult()
	_ = i.emitter.EmitTextDocumentReferences(d.ResultSetID, refResultID)
	_ = i.emitter.EmitItemOfDefinitions(refResultID, []uint64{d.RangeID}, d.DocumentID)

	for documentID, rangeIDs := range d.ReferenceRangeIDs {
		_ = i.emitter.EmitItemOfReferences(refResultID, rangeIDs, documentID)
	}
}

// emitContains emits the contains relationship for all documents and the ranges that it contains.
func (i *Indexer) emitContains() {
	i.visitEachDocument("Emitting contains relations", i.emitContainsForDocument)

	// TODO(efritz) - think about printing a title here
	i.emitContainsForProject()
}

// emitContainsForProject emits a contains edge between a document and its ranges.
func (i *Indexer) emitContainsForDocument(d *DocumentInfo) {
	if len(d.SymbolRangeIDs) > 0 || len(d.DefinitionRangeIDs) > 0 || len(d.ReferenceRangeIDs) > 0 {
		_ = i.emitter.EmitContains(d.DocumentID, union(d.SymbolRangeIDs, d.DefinitionRangeIDs, d.ReferenceRangeIDs))
	}
}

// emitContainsForProject emits a contains edge between the target project and all indexed documents.
func (i *Indexer) emitContainsForProject() {
	documentIDs := make([]uint64, 0, len(i.documents))
	for _, info := range i.documents {
		documentIDs = append(documentIDs, info.DocumentID)
	}

	if len(documentIDs) > 0 {
		_ = i.emitter.EmitContains(i.projectID, documentIDs)
	}
}

// Stats returns an IndexerStats object with the number of packages, files, and elements analyzed/emitted.
func (i *Indexer) Stats() IndexerStats {
	return IndexerStats{
		NumPkgs:     uint(len(i.packages)),
		NumFiles:    uint(len(i.documents)),
		NumDefs:     uint(len(i.consts) + len(i.funcs) + len(i.imports) + len(i.labels) + len(i.types) + len(i.vars)),
		NumElements: i.emitter.NumElements(),
	}
}
