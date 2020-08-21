package indexer

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sourcegraph/lsif-go/internal/writer"
	protocolwriter "github.com/sourcegraph/lsif-go/internal/writer"
	"github.com/sourcegraph/lsif-go/protocol"
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
	animate        bool              // Whether to animate output
	silent         bool              // Whether to suppress all output
	verbose        bool              // Whether to display elapsed time

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
	preloader             *Preloader                // hover text cache
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
	writer protocolwriter.JSONWriter,
	animate bool,
	silent bool,
	verbose bool,
) *Indexer {
	return &Indexer{
		repositoryRoot:        repositoryRoot,
		projectRoot:           projectRoot,
		toolInfo:              toolInfo,
		moduleName:            moduleName,
		moduleVersion:         moduleVersion,
		dependencies:          dependencies,
		emitter:               protocolwriter.NewEmitter(writer),
		animate:               animate,
		silent:                silent,
		verbose:               verbose,
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
		preloader:             newPreloader(),
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
	i.preload()
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

		pkgs, err := packages.Load(&packages.Config{Mode: loadMode, Dir: i.projectRoot, Tests: true}, "./...")
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

	withProgress(&wg, "Loading packages", i.animate, i.silent, i.verbose, nil, 0)
	return <-errs

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

// preload populates the preloader with hover text for every definition within the index target
// packages, as well as the definitions in all directly imported packages (but no transitively
// imported packages). This will also load the moniker paths for all identifiers in the same
// files.
func (i *Indexer) preload() {
	ch := make(chan func())
	pkgs := getAllReferencedPackages(i.packages)

	go func() {
		defer close(ch)

		for _, p := range pkgs {
			t := p
			ch <- func() { i.preloader.Load(t, getDefinitionPositions(t)) }
		}
	}()

	// Load hovers for each package concurrently
	wg, count := runParallel(ch)
	withProgress(wg, "Preloading hover text and moniker paths", i.animate, i.silent, i.verbose, count, uint64(len(pkgs)))
}

// getDefinitionPositions extracts the positions of all definitions from the given package. This
// returns a sorted slice.
func getDefinitionPositions(p *packages.Package) []token.Pos {
	positions := make([]token.Pos, 0, len(p.TypesInfo.Defs))
	for _, obj := range p.TypesInfo.Defs {
		if obj != nil {
			positions = append(positions, obj.Pos())
		}
	}

	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	return positions
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

// indexDefinitions emits data for each definition in an index target package. This will emit
// a result set, a definition result, a hover result, and export monikers attached to each range.
// This method will also populate each document's definition range identifier slice.
func (i *Indexer) indexDefinitions() {
	i.visitEachPackage("Indexing definitions", i.indexDefinitionsForPackage)
}

// indexDefinitionsForPackage emits data for each definition within the given package.
func (i *Indexer) indexDefinitionsForPackage(p *packages.Package) {
	for ident, obj := range p.TypesInfo.Defs {
		pos, d, ok := i.positionAndDocument(p, ident, obj)
		if !ok || !i.markRange(pos) {
			continue
		}

		rangeID := i.indexDefinition(p, pos.Filename, d, ident, pos, obj)

		i.stripedMutex.LockKey(pos.Filename)
		i.ranges[pos.Filename][pos.Offset] = rangeID
		i.stripedMutex.UnlockKey(pos.Filename)

		d.m.Lock()
		d.DefinitionRangeIDs = append(d.DefinitionRangeIDs, rangeID)
		d.m.Unlock()
	}
}

// positionAndDocument returns the position of the given object and the document info object
// that contains it. If the given package is not the canonical package for the containing file
// in the packagesByFile map, this method returns false.
func (i *Indexer) positionAndDocument(p *packages.Package, ident *ast.Ident, obj types.Object) (token.Position, *DocumentInfo, bool) {
	if obj == nil {
		return token.Position{}, nil, false
	}

	pos := p.Fset.Position(ident.Pos())

	if packages := i.packagesByFile[pos.Filename]; len(packages) == 0 || packages[0] != p {
		return token.Position{}, nil, false
	}

	d, hasDocument := i.documents[pos.Filename]
	if !hasDocument {
		return token.Position{}, nil, false
	}

	return pos, d, true
}

// markRange sets an empty range identifier in the ranges map for the given position.
// If a  range for this identifier has already been marked, this method returns false.
func (i *Indexer) markRange(pos token.Position) bool {
	i.stripedMutex.RLockKey(pos.Filename)
	_, ok := i.ranges[pos.Filename][pos.Offset]
	i.stripedMutex.RUnlockKey(pos.Filename)
	if ok {
		return false
	}

	i.stripedMutex.LockKey(pos.Filename)
	defer i.stripedMutex.UnlockKey(pos.Filename)

	if _, ok := i.ranges[pos.Filename][pos.Offset]; ok {
		return false
	}

	i.ranges[pos.Filename][pos.Offset] = 0 // placeholder
	return true
}

// indexDefinition emits data for the given definition object.
func (i *Indexer) indexDefinition(p *packages.Package, filename string, document *DocumentInfo, ident *ast.Ident, pos token.Position, obj types.Object) uint64 {
	// Create a hover result vertex and cache the result identifier keyed by the definition location.
	// Caching this gives us a big win for package documentation, which is likely to be large and is
	// repeated at each import and selector within referenced files.
	hoverResultID := i.makeCachedHoverResult(nil, obj, func() []protocol.MarkedString {
		return findHoverContents(i.preloader, i.packages, p, obj)
	})

	rangeID := i.emitter.EmitRange(rangeForObject(obj, ident, pos))
	resultSetID := i.emitter.EmitResultSet()
	defResultID := i.emitter.EmitDefinitionResult()

	_ = i.emitter.EmitNext(rangeID, resultSetID)
	_ = i.emitter.EmitTextDocumentDefinition(resultSetID, defResultID)
	_ = i.emitter.EmitItem(defResultID, []uint64{rangeID}, document.DocumentID)
	_ = i.emitter.EmitTextDocumentHover(resultSetID, hoverResultID)

	if _, ok := obj.(*types.PkgName); ok {
		i.emitImportMoniker(resultSetID, p, ident, obj)
	}

	if ident.IsExported() {
		i.emitExportMoniker(resultSetID, p, ident, obj)
	}

	i.setDefinitionInfo(ident, obj, &DefinitionInfo{
		DocumentID:        document.DocumentID,
		RangeID:           rangeID,
		ResultSetID:       resultSetID,
		ReferenceRangeIDs: map[uint64][]uint64{},
	})

	return rangeID
}

// setDefinitionInfo stashes the given definition info indexed by the given object type and name.
// This definition info will be accessible by invoking getDefinitionInfo with the same type and
// name values (but not necessarily the same object).
func (i *Indexer) setDefinitionInfo(ident *ast.Ident, obj types.Object, d *DefinitionInfo) {
	switch v := obj.(type) {
	case *types.Const:
		i.constsMutex.Lock()
		i.consts[ident.Pos()] = d
		i.constsMutex.Unlock()

	case *types.Func:
		i.funcsMutex.Lock()
		i.funcs[v.FullName()] = d
		i.funcsMutex.Unlock()

	case *types.Label:
		i.labelsMutex.Lock()
		i.labels[ident.Pos()] = d
		i.labelsMutex.Unlock()

	case *types.PkgName:
		i.importsMutex.Lock()
		i.imports[ident.Pos()] = d
		i.importsMutex.Unlock()

	case *types.TypeName:
		i.typesMutex.Lock()
		i.types[obj.Type().String()] = d
		i.typesMutex.Unlock()

	case *types.Var:
		i.varsMutex.Lock()
		i.vars[ident.Pos()] = d
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
	for ident, obj := range p.TypesInfo.Uses {
		pos, d, ok := i.positionAndDocument(p, ident, obj)
		if !ok {
			continue
		}

		rangeID, ok := i.indexReference(p, d, ident, pos, obj)
		if !ok {
			continue
		}

		d.m.Lock()
		d.ReferenceRangeIDs = append(d.ReferenceRangeIDs, rangeID)
		d.m.Unlock()
	}
}

// indexReference emits data for the given reference object.
func (i *Indexer) indexReference(p *packages.Package, document *DocumentInfo, ident *ast.Ident, pos token.Position, obj types.Object) (uint64, bool) {
	if def := i.getDefinitionInfo(obj); def != nil {
		return i.indexReferenceToDefinition(document, ident, pos, obj, def)
	}

	return i.indexReferenceToExternalDefinition(p, document, ident, pos, obj)
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
func (i *Indexer) indexReferenceToDefinition(document *DocumentInfo, ident *ast.Ident, pos token.Position, obj types.Object, d *DefinitionInfo) (uint64, bool) {
	rangeID := i.ensureRangeFor(ident, pos, obj)
	_ = i.emitter.EmitNext(rangeID, d.ResultSetID)

	d.m.Lock()
	d.ReferenceRangeIDs[document.DocumentID] = append(d.ReferenceRangeIDs[document.DocumentID], rangeID)
	d.m.Unlock()

	return rangeID, true
}

// indexReferenceToExternalDefinition emits data for the given reference object that is not defined
// within an index target package. This definition _may_ be resolvable by scanning dependencies, but
// it is not guaranteed.
func (i *Indexer) indexReferenceToExternalDefinition(p *packages.Package, document *DocumentInfo, ident *ast.Ident, pos token.Position, obj types.Object) (uint64, bool) {
	definitionPkg := obj.Pkg()
	if definitionPkg == nil {
		return 0, false
	}

	// Create a or retreive a hover result identifier keyed by the target object's identifier
	// (scoped ot the object's package name). Caching this gives us another big win as some
	// methods imported from other packages are likely to be used many times in a dependent
	// project (e.g., context.Context, http.Request, etc).
	hoverResultID := i.makeCachedHoverResult(definitionPkg, obj, func() []protocol.MarkedString {
		return findExternalHoverContents(i.preloader, i.packages, p, obj)
	})

	rangeID := i.ensureRangeFor(ident, pos, obj)
	refResultID := i.emitter.EmitReferenceResult()
	_ = i.emitter.EmitTextDocumentReferences(rangeID, refResultID)
	_ = i.emitter.EmitItemOfReferences(refResultID, []uint64{rangeID}, document.DocumentID)

	if hoverResultID != 0 {
		_ = i.emitter.EmitTextDocumentHover(rangeID, hoverResultID)
	}

	i.emitImportMoniker(rangeID, p, ident, obj)
	return rangeID, true
}

// ensureRangeFor returns a range identifier for the given object. If a range for the object has
// not been emitted, a new vertex is created.
func (i *Indexer) ensureRangeFor(ident *ast.Ident, pos token.Position, obj types.Object) uint64 {
	i.stripedMutex.RLockKey(pos.Filename)
	rangeID, ok := i.ranges[pos.Filename][pos.Offset]
	i.stripedMutex.RUnlockKey(pos.Filename)
	if ok {
		return rangeID
	}

	// Note: we calculate this outside of the critical section
	start, end := rangeForObject(obj, ident, pos)

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
	if len(d.DefinitionRangeIDs) > 0 || len(d.ReferenceRangeIDs) > 0 {
		_ = i.emitter.EmitContains(d.DocumentID, union(d.DefinitionRangeIDs, d.ReferenceRangeIDs))
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
func (i *Indexer) Stats() *IndexerStats {
	return &IndexerStats{
		NumPkgs:     uint(len(i.packages)),
		NumFiles:    uint(len(i.documents)),
		NumDefs:     uint(len(i.consts) + len(i.funcs) + len(i.imports) + len(i.labels) + len(i.types) + len(i.vars)),
		NumElements: i.emitter.NumElements(),
	}
}
