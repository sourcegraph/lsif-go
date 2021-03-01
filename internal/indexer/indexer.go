package indexer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sourcegraph/lsif-go/protocol"
	"golang.org/x/tools/go/packages"
)

type Indexer struct {
	repositoryRoot string              // path to repository
	projectRoot    string              // path to package
	toolInfo       protocol.ToolInfo   // metadata vertex payload
	moduleName     string              // name of this module
	moduleVersion  string              // version of this module
	filesToIndex   map[string]struct{} // string set of specific files to index
	dependencies   map[string]string   // parsed module data
	emitter        *protocol.Emitter   // LSIF data emitter
	animate        bool                // Whether to animate output
	silent         bool                // Whether to suppress all output
	verbose        bool                // Whether to display elapsed time

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

	constsMutex                sync.RWMutex
	funcsMutex                 sync.RWMutex
	importsMutex               sync.RWMutex
	labelsMutex                sync.RWMutex
	typesMutex                 sync.RWMutex
	varsMutex                  sync.RWMutex
	documentsMutex             sync.RWMutex
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
	filesToIndex map[string]struct{},
	dependencies map[string]string,
	writer protocol.JSONWriter,
	packageDataCache *PackageDataCache,
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
		filesToIndex:          filesToIndex,
		dependencies:          dependencies,
		emitter:               protocol.NewEmitter(writer),
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
		packageDataCache:      packageDataCache,
		stripedMutex:          newStripedMutex(),
	}
}

// Index generates an LSIF dump from a workspace by traversing through source files
// and writing the LSIF equivalent to the output source that implements io.Writer.
// It is caller's responsibility to close the output source if applicable.
func (i *Indexer) Index() error {
	start := time.Now()
	if err := i.loadPackages(); err != nil {
		return errors.Wrap(err, "loadPackages")
	}
	packages := time.Now()

	i.emitMetadataAndProjectVertex()
	i.emitDocuments()
	i.addImports()
	i.indexDefinitions()
	i.indexReferences()
	i.linkReferenceResultsToRanges()
	i.emitContains()

	lsif := time.Now()

	if err := i.emitter.Flush(); err != nil {
		return errors.Wrap(err, "emitter.Flush")
	}

	fmt.Printf("%.2f seconds in packages, %.2f seconds in lsif\n", packages.Sub(start).Seconds(), lsif.Sub(packages).Seconds())

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

		patterns := []string{"./..."}

		if i.filesToIndex != nil {
			patterns = nil
			for file := range i.filesToIndex {
				lastIdx := strings.LastIndex(file, "/")
				patterns = append(patterns, file[:lastIdx])
			}
		}

		pkgs, err := packages.Load(&packages.Config{Mode: loadMode, Dir: i.projectRoot, Tests: true}, patterns...)
		if err != nil {
			errs <- errors.Wrap(err, "packages.Load")
			return
		}

		i.packages = pkgs
		fmt.Printf("%v\n", len(pkgs))
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
	i.visitEachRawFile("Emitting documents", i.maybeEmitDocument)
}

func (i *Indexer) maybeEmitDocument(filename string) {
	if i.filesToIndex != nil {
		if _, exists := i.filesToIndex[filename]; exists {
			i.ensureDocument(filename)
		}
	} else {
		i.ensureDocument(filename)
	}
}

// emitDocument emits a document vertex and a contains relation to the enclosing project. This method
// also prepares the documents and ranges maps (alternatively: this method must be called before any
// other method that requires the filename key be present in either map).
func (i *Indexer) ensureDocument(filename string) (*DocumentInfo, bool) {
	// Emit each document only once
	i.documentsMutex.RLock()
	d, exists := i.documents[filename]
	i.documentsMutex.RUnlock()
	if exists {
		return d, exists
	}

	// Indexing test files means that we're also indexing the code _generated_ by go test;
	// e.g. file://Users/efritz/Library/Caches/go-build/07/{64-character identifier}-d. Skip
	// These files as they won't be navigable outside of the machine that indexed the project.
	if !strings.HasPrefix(filename, i.projectRoot) {
		return nil, false
	}

	i.documentsMutex.Lock()
	i.stripedMutex.LockKey(filename)
	defer i.documentsMutex.Unlock()
	defer i.stripedMutex.UnlockKey(filename)
	if d, exists := i.documents[filename]; exists {
		return d, exists
	}

	documentID := i.emitter.EmitDocument(languageGo, filename)
	documentInfo := DocumentInfo{DocumentID: documentID}
	i.documents[filename] = &documentInfo
	i.ranges[filename] = map[int]uint64{}

	return &documentInfo, true
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

// indexDefinitions emits data for each definition in an index target package. This will emit
// a result set, a definition result, a hover result, and export monikers attached to each range.
// This method will also populate each document's definition range identifier slice.
func (i *Indexer) indexDefinitions() {
	i.visitEachPackage("Indexing definitions", i.indexDefinitionsForPackage)
}

// indexDefinitionsForPackage emits data for each definition within the given package.
func (i *Indexer) indexDefinitionsForPackage(p *packages.Package) {
	for _, obj := range p.TypesInfo.Defs {
		if obj == nil {
			continue
		}

		pos, d, ok := i.positionAndDocument(p, obj.Pos())
		if !ok {
			continue
		}

		if i.filesToIndex != nil {
			if _, exists := i.filesToIndex[pos.Filename]; !exists {
				continue
			}
		}

		i.ensureDefinition(p, d, pos, obj)
	}
}

// positionAndDocument returns the position of the given object and the document info object
// that contains it. If the given package is not the canonical package for the containing file
// in the packagesByFile map, this method returns false.
func (i *Indexer) positionAndDocument(p *packages.Package, pos token.Pos) (token.Position, *DocumentInfo, bool) {
	position := p.Fset.Position(pos)

	if packages := i.packagesByFile[position.Filename]; len(packages) == 0 || packages[0] != p {
		return position, nil, false
	}

	i.documentsMutex.RLock()
	d, hasDocument := i.documents[position.Filename]
	i.documentsMutex.RUnlock()
	if !hasDocument {
		return position, nil, false
	}

	return position, d, true
}

// markRange sets an empty range identifier in the ranges map for the given position.
// If a range for this identifier has already been marked, this method returns false.
func (i *Indexer) markRange(pos token.Position) bool {
	i.documentsMutex.RLock()
	_, ok := i.ranges[pos.Filename][pos.Offset]
	i.documentsMutex.RUnlock()
	if ok {
		return false
	}

	i.documentsMutex.Lock()
	defer i.documentsMutex.Unlock()

	if _, ok := i.ranges[pos.Filename][pos.Offset]; ok {
		return false
	}

	i.ranges[pos.Filename][pos.Offset] = 0 // placeholder
	return true
}

// indexDefinition emits data for the given definition object.
func (i *Indexer) ensureDefinition(p *packages.Package, document *DocumentInfo, pos token.Position, obj types.Object) *DefinitionInfo {
	if !i.markRange(pos) {
		info := i.getDefinitionInfo(p, obj)
		return info
	}

	// Create a hover result vertex and cache the result identifier keyed by the definition location.
	// Caching this gives us a big win for package documentation, which is likely to be large and is
	// repeated at each import and selector within referenced files.
	hoverResultID := i.makeCachedHoverResult(nil, obj, func() []protocol.MarkedString {
		return findHoverContents(i.packageDataCache, i.packages, p, obj)
	})

	rangeID := i.emitter.EmitRange(rangeForObject(obj, pos))
	resultSetID := i.emitter.EmitResultSet()
	defResultID := i.emitter.EmitDefinitionResult()

	_ = i.emitter.EmitNext(rangeID, resultSetID)
	_ = i.emitter.EmitTextDocumentDefinition(resultSetID, defResultID)
	_ = i.emitter.EmitItem(defResultID, []uint64{rangeID}, document.DocumentID)
	_ = i.emitter.EmitTextDocumentHover(resultSetID, hoverResultID)

	if _, ok := obj.(*types.PkgName); ok {
		i.emitImportMoniker(resultSetID, p, obj)
	}

	if obj.Exported() {
		i.emitExportMoniker(resultSetID, p, obj)
	}

	defInfo := DefinitionInfo{
		DocumentID:        document.DocumentID,
		RangeID:           rangeID,
		ResultSetID:       resultSetID,
		ReferenceRangeIDs: map[uint64][]uint64{},
	}

	i.setDefinitionInfo(obj, &defInfo)

	i.documentsMutex.Lock()
	i.ranges[pos.Filename][pos.Offset] = rangeID
	i.documentsMutex.Unlock()

	document.m.Lock()
	document.DefinitionRangeIDs = append(document.DefinitionRangeIDs, rangeID)
	document.m.Unlock()

	return &defInfo
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
	for ident, defObj := range p.TypesInfo.Uses {
		pos, d, ok := i.positionAndDocument(p, ident.Pos())
		if !ok {
			continue
		}

		if i.filesToIndex != nil {
			if _, exists := i.filesToIndex[pos.Filename]; !exists {
				continue
			}
		}

		rangeID, ok := i.indexReference(p, d, pos, defObj)
		if !ok {
			continue
		}

		d.m.Lock()
		d.ReferenceRangeIDs = append(d.ReferenceRangeIDs, rangeID)
		d.m.Unlock()
	}
}

// indexReference emits data for the given reference object.
func (i *Indexer) indexReference(p *packages.Package, document *DocumentInfo, refPos token.Position, defObj types.Object) (uint64, bool) {
	if def := i.ensureDefinitionInfo(p, defObj); def != nil {
		return i.indexReferenceToDefinition(document, refPos, defObj, def)
	}

	return i.indexReferenceToExternalDefinition(p, document, refPos, defObj)
}

func (i *Indexer) ensureDefinitionInfo(p *packages.Package, obj types.Object) (def *DefinitionInfo) {
	pos := p.Fset.Position(obj.Pos())

	d, ok := i.ensureDocument(pos.Filename)
	if !ok {
		return nil
	}

	return i.ensureDefinition(p, d, pos, obj)
}

// getDefinitionInfo returns the definition info object for the given object. This requires that
// setDefinitionInfo was previously called an object that can be resolved in the same way. This
// will only return definitions which are defined in an index target (not a dependency).
func (i *Indexer) getDefinitionInfo(p *packages.Package, obj types.Object) (def *DefinitionInfo) {
	switch v := obj.(type) {
	case *types.Const:
		i.constsMutex.RLock()
		def = i.consts[v.Pos()]
		i.constsMutex.RUnlock()
	case *types.Func:
		i.funcsMutex.RLock()
		def = i.funcs[v.FullName()]
		i.funcsMutex.RUnlock()
	case *types.Label:
		i.labelsMutex.RLock()
		def = i.labels[v.Pos()]
		i.labelsMutex.RUnlock()
	case *types.PkgName:
		i.importsMutex.RLock()
		def = i.imports[v.Pos()]
		i.importsMutex.RUnlock()
	case *types.TypeName:
		i.typesMutex.RLock()
		def = i.types[obj.Type().String()]
		i.typesMutex.RUnlock()
	case *types.Var:
		i.varsMutex.RLock()
		def = i.vars[v.Pos()]
		i.varsMutex.RUnlock()
	}
	return def
}

// indexReferenceToDefinition emits data for the given reference object that is defined within
// an index target package.
func (i *Indexer) indexReferenceToDefinition(document *DocumentInfo, pos token.Position, obj types.Object, d *DefinitionInfo) (uint64, bool) {
	rangeID := i.ensureRangeFor(pos, obj)
	_ = i.emitter.EmitNext(rangeID, d.ResultSetID)

	d.m.Lock()
	d.ReferenceRangeIDs[document.DocumentID] = append(d.ReferenceRangeIDs[document.DocumentID], rangeID)
	d.m.Unlock()

	return rangeID, true
}

// indexReferenceToExternalDefinition emits data for the given reference object that is not defined
// within an index target package. This definition _may_ be resolvable by scanning dependencies, but
// it is not guaranteed.
func (i *Indexer) indexReferenceToExternalDefinition(p *packages.Package, document *DocumentInfo, pos token.Position, obj types.Object) (uint64, bool) {
	definitionPkg := obj.Pkg()
	if definitionPkg == nil {
		return 0, false
	}

	// Create a or retreive a hover result identifier keyed by the target object's identifier
	// (scoped ot the object's package name). Caching this gives us another big win as some
	// methods imported from other packages are likely to be used many times in a dependent
	// project (e.g., context.Context, http.Request, etc).
	hoverResultID := i.makeCachedHoverResult(definitionPkg, obj, func() []protocol.MarkedString {
		return findExternalHoverContents(i.packageDataCache, i.packages, p, obj)
	})

	rangeID := i.ensureRangeFor(pos, obj)
	refResultID := i.emitter.EmitReferenceResult()
	_ = i.emitter.EmitTextDocumentReferences(rangeID, refResultID)
	_ = i.emitter.EmitItemOfReferences(refResultID, []uint64{rangeID}, document.DocumentID)

	if hoverResultID != 0 {
		_ = i.emitter.EmitTextDocumentHover(rangeID, hoverResultID)
	}

	i.emitImportMoniker(rangeID, p, obj)
	return rangeID, true
}

// ensureRangeFor returns a range identifier for the given object. If a range for the object has
// not been emitted, a new vertex is created.
func (i *Indexer) ensureRangeFor(pos token.Position, obj types.Object) uint64 {
	i.documentsMutex.RLock()
	rangeID, ok := i.ranges[pos.Filename][pos.Offset]
	i.documentsMutex.RUnlock()
	if ok {
		return rangeID
	}

	// Note: we calculate this outside of the critical section
	start, end := rangeForObject(obj, pos)

	i.documentsMutex.Lock()
	defer i.documentsMutex.Unlock()

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
func (i *Indexer) Stats() IndexerStats {
	return IndexerStats{
		NumPkgs:     uint(len(i.packages)),
		NumFiles:    uint(len(i.documents)),
		NumDefs:     uint(len(i.consts) + len(i.funcs) + len(i.imports) + len(i.labels) + len(i.types) + len(i.vars)),
		NumElements: i.emitter.NumElements(),
	}
}
