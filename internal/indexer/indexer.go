package indexer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"math"
	"path"
	"strings"
	"sync"

	"github.com/agnivade/levenshtein"
	"github.com/pkg/errors"
	"github.com/sourcegraph/lsif-go/internal/command"
	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/output"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer"
	"golang.org/x/tools/go/packages"
)

type importMonikerReference struct {
	monikerID  uint64
	documentID uint64
	rangeID    uint64
}
type setVal interface{}

type Indexer struct {
	repositoryRoot   string                    // path to repository
	repositoryRemote string                    // import path inferred by git remote
	projectRoot      string                    // path to package
	toolInfo         protocol.ToolInfo         // metadata vertex payload
	moduleName       string                    // name of this module
	moduleVersion    string                    // version of this module
	dependencies     map[string]gomod.GoModule // parsed module data
	emitter          *writer.Emitter           // LSIF data emitter
	outputOptions    output.Options            // What to print to stdout/stderr

	// Definition type cache
	consts  map[interface{}]*DefinitionInfo // position -> info
	funcs   map[interface{}]*DefinitionInfo // name -> info
	imports map[interface{}]*DefinitionInfo // position -> info
	labels  map[interface{}]*DefinitionInfo // position -> info
	types   map[interface{}]*DefinitionInfo // name -> info
	vars    map[interface{}]*DefinitionInfo // position -> info

	// LSIF data cache
	documents                                map[string]*DocumentInfo                // filename -> info
	ranges                                   map[string]map[int]uint64               // filename -> offset -> rangeID
	defined                                  map[string]map[int]struct{}             // set of defined ranges (filename, offset)
	hoverResultCache                         map[string]uint64                       // cache key -> hoverResultID
	importMonikerIDs                         map[string]uint64                       // identifier:packageInformationID -> monikerID
	implementationMonikerIDs                 map[string]uint64                       // identifier:packageInformationID -> monikerID
	importMonikerReferences                  map[uint64]map[uint64]map[uint64]setVal // monikerKey -> documentID -> Set(rangeID)
	packageInformationIDs                    map[string]uint64                       // name -> packageInformationID
	packageDataCache                         *PackageDataCache                       // hover text and moniker path cache
	packages                                 []*packages.Package                     // index target packages
	depPackages                              []*packages.Package                     // dependency packages
	projectID                                uint64                                  // project vertex identifier
	packagesByFile                           map[string][]*packages.Package
	emittedDocumentationResults              map[ObjectLike]uint64 // type object -> documentationResult vertex ID
	emittedDocumentationResultsByPackagePath map[string]uint64     // package path -> documentationResult vertex ID

	constsMutex                sync.Mutex
	funcsMutex                 sync.Mutex
	importsMutex               sync.Mutex
	labelsMutex                sync.Mutex
	typesMutex                 sync.Mutex
	varsMutex                  sync.Mutex
	stripedMutex               *StripedMutex
	hoverResultCacheMutex      sync.RWMutex
	importMonikerIDsMutex      sync.RWMutex
	packageInformationIDsMutex sync.RWMutex

	importMonikerChannel chan importMonikerReference
}

func New(
	repositoryRoot string,
	repositoryRemote string,
	projectRoot string,
	toolInfo protocol.ToolInfo,
	moduleName string,
	moduleVersion string,
	dependencies map[string]gomod.GoModule,
	jsonWriter writer.JSONWriter,
	packageDataCache *PackageDataCache,
	outputOptions output.Options,
) *Indexer {
	return &Indexer{
		repositoryRoot:           repositoryRoot,
		repositoryRemote:         repositoryRemote,
		projectRoot:              projectRoot,
		toolInfo:                 toolInfo,
		moduleName:               moduleName,
		moduleVersion:            moduleVersion,
		dependencies:             dependencies,
		emitter:                  writer.NewEmitter(jsonWriter),
		outputOptions:            outputOptions,
		consts:                   map[interface{}]*DefinitionInfo{},
		funcs:                    map[interface{}]*DefinitionInfo{},
		imports:                  map[interface{}]*DefinitionInfo{},
		labels:                   map[interface{}]*DefinitionInfo{},
		types:                    map[interface{}]*DefinitionInfo{},
		vars:                     map[interface{}]*DefinitionInfo{},
		documents:                map[string]*DocumentInfo{},
		ranges:                   map[string]map[int]uint64{},
		defined:                  map[string]map[int]struct{}{},
		hoverResultCache:         map[string]uint64{},
		importMonikerIDs:         map[string]uint64{},
		implementationMonikerIDs: map[string]uint64{},
		importMonikerReferences:  map[uint64]map[uint64]map[uint64]setVal{},
		packageInformationIDs:    map[string]uint64{},
		packageDataCache:         packageDataCache,
		stripedMutex:             newStripedMutex(),
		importMonikerChannel:     make(chan importMonikerReference, 512),
	}
}

// Index generates an LSIF dump from a workspace by traversing through source files
// and writing the LSIF equivalent to the output source that implements io.Writer.
// It is caller's responsibility to close the output source if applicable.
func (i *Indexer) Index() error {
	if err := i.loadPackages(true); err != nil {
		return errors.Wrap(err, "failed to load packages")
	}

	wg := new(sync.WaitGroup)
	// Start any channels used to synchronize reference sets
	i.startImportMonikerReferenceTracker(wg)

	// Begin emitting and indexing package
	i.emitMetadataAndProjectVertex()
	i.emitDocuments()
	i.emitImports()
	i.indexPackageDeclarations()
	i.indexDocumentation() // must be invoked before indexDefinitions/indexReferences
	i.indexDefinitions()
	i.indexReferences()
	i.indexImplementations()

	// Stop any channels used to synchronize reference sets
	i.stopImportMonikerReferenceTracker(wg)

	// Link sets of items to corresponding ranges and results.
	i.linkReferenceResultsToRanges()
	i.linkImportMonikersToRanges()
	i.linkContainsToRanges()

	if err := i.emitter.Flush(); err != nil {
		return errors.Wrap(err, "failed to write index to disk")
	}

	return nil
}

func (i *Indexer) startImportMonikerReferenceTracker(wg *sync.WaitGroup) {
	wg.Add(1)

	go func() {
		contained := struct{}{}

		for nextReference := range i.importMonikerChannel {
			monikerID := nextReference.monikerID
			documentID := nextReference.documentID
			rangeID := nextReference.rangeID

			if monikerID == 0 || documentID == 0 || rangeID == 0 {
				// TODO: We should add error logging/warning somehow for these to be easily reported back to user,
				// but I have not had this happen at all in testing.
				continue
			}

			monikerMap, ok := i.importMonikerReferences[monikerID]
			if !ok {
				monikerMap = map[uint64]map[uint64]setVal{}
				i.importMonikerReferences[monikerID] = monikerMap
			}

			documentMap, ok := monikerMap[documentID]
			if !ok {
				documentMap = map[uint64]setVal{}
				monikerMap[documentID] = documentMap
			}

			documentMap[rangeID] = contained
		}

		wg.Done()
	}()
}

func (i *Indexer) stopImportMonikerReferenceTracker(wg *sync.WaitGroup) {
	close(i.importMonikerChannel)
	wg.Wait()
}

var loadMode = packages.NeedDeps | packages.NeedFiles | packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedName

// cachedPackages makes sure that we only load packages once per execution
var cachedPackages map[string][]*packages.Package = map[string][]*packages.Package{}

// cachedDepPackages makes sure that we only load dependency packages once per execution
var cachedDepPackages map[string][]*packages.Package = map[string][]*packages.Package{}

// packages populates the packages field containing an AST for each package within the configured
// project root.
//
// deduplicate should be true in all cases except TestIndexer_shouldVisitPackage.
func (i *Indexer) loadPackages(deduplicate bool) error {
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

		load := func(cache map[string][]*packages.Package, patterns ...string) ([]*packages.Package, bool) {
			// Make sure we only load packages once per execution.
			pkgs, ok := cache[i.projectRoot]
			if !ok {
				var err error
				pkgs, err = packages.Load(config, patterns...)
				if err != nil {
					errs <- errors.Wrap(err, "packages.Load")
					return nil, true
				}

				cache[i.projectRoot] = pkgs
			}

			if !deduplicate {
				return pkgs, false
			}

			keep := make([]*packages.Package, 0, len(pkgs))
			for _, pkg := range pkgs {
				if i.shouldVisitPackage(pkg, pkgs) {
					keep = append(keep, pkg)
				}
			}
			return keep, false
		}

		var hasError bool

		i.packages, hasError = load(cachedPackages, "./...")
		if hasError {
			return
		}

		deps, err := i.listProjectDependencies()
		if err != nil {
			errs <- errors.Wrap(err, "failed to list dependencies")
			return
		}
		i.depPackages, hasError = load(cachedDepPackages, deps...)
		if hasError {
			return
		}

		i.packagesByFile = map[string][]*packages.Package{}
		for _, p := range i.packages {
			for _, f := range p.Syntax {
				filename := p.Fset.Position(f.Package).Filename
				i.packagesByFile[filename] = append(i.packagesByFile[filename], p)
			}
		}
	}()

	output.WithProgressParallel(&wg, "Loading packages", i.outputOptions, nil, 0)
	return <-errs
}

// shouldVisitPackage tells if the package p should be visited.
//
// According to the `Tests` field in https://pkg.go.dev/golang.org/x/tools/go/packages#Config
// the loader may produce up to four packages for a single Go package directory:
//
// 	// For example, when using the go command, loading "fmt" with Tests=true
// 	// returns four packages, with IDs "fmt" (the standard package),
// 	// "fmt [fmt.test]" (the package as compiled for the test),
// 	// "fmt_test" (the test functions from source files in package fmt_test),
// 	// and "fmt.test" (the test binary).
//
// This function handles deduplication, returning true ("should visit") if it makes sense
// to index the input package (or false if doing so would be duplicative.)
func (i *Indexer) shouldVisitPackage(p *packages.Package, allPackages []*packages.Package) bool {
	// The loader returns 4 packages because (loader.Config).Tests==true and we
	// want to avoid duplication.
	if p.Name == "main" && strings.HasSuffix(p.ID, ".test") {
		return false // synthesized `go test` program
	}
	if strings.HasSuffix(p.Name, "_test") {
		return true
	}

	// Index only the combined test package if it's present. If the package has no test files,
	// it won't be present, and we need to just index the default package.
	pkgHasTests := false
	for _, op := range allPackages {
		if op.ID == fmt.Sprintf("%s [%s.test]", p.PkgPath, p.PkgPath) {
			pkgHasTests = true
			break
		}
	}
	if pkgHasTests && !strings.HasSuffix(p.ID, ".test]") {
		return false
	}
	return true
}

// packagesLoadLogger logs the debug messages from the packages.Load function.
//
// We only care about one message, which contains the output of the `go list`
// command. In order to determine what relevant data we should print, we try to
// unmarshal the fourth log argument value (a *bytes.Buffer of go list stdout)
// as a stream of JSON objects representing loaded (or candidate) packages.
func (i *Indexer) packagesLoadLogger(format string, args ...interface{}) {
	if i.outputOptions.Verbosity < output.VeryVeryVerboseOutput || len(args) < 4 {
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
	i.defined[filename] = map[int]struct{}{}
}

// emitImports will emit the appropriate import monikers and named definitions for all packages.
func (i *Indexer) emitImports() {
	i.visitEachPackage("Emitting import references and definitions", i.emitImportsForPackage)
}

// emitImportsForPackage will emit the appropriate import monikers and named definitions for a package.
func (i *Indexer) emitImportsForPackage(p *packages.Package) {
	for _, f := range p.Syntax {
		for _, spec := range f.Imports {
			pkg := p.Imports[strings.Trim(spec.Path.Value, `"`)]
			if pkg == nil {
				continue
			}

			i.emitImportMonikerReference(p, pkg, spec)

			// spec.Name is only non-nil when we have an import of the form:
			//     import f "fmt"
			//
			// So, we want to emit a local defition for the `f` token
			if spec.Name != nil {
				i.emitImportMonikerNamedDefinition(p, pkg, spec)
			}
		}
	}
}

// emitImportMonikerReference will emit the associated reference to the import moniker.
// This will emit the reference in either case:
//
//    import "fmt"
//            ^^^------ reference github.com/golang/go/std/fmt
//
//    import f "fmt"
//              ^^^---- reference github.com/golang/go/std/fmt
//
// In both cases, this will emit the corresponding import moniker for "fmt". This is ImportSpec.Path
func (i *Indexer) emitImportMonikerReference(p *packages.Package, pkg *packages.Package, spec *ast.ImportSpec) {
	pos := spec.Path.Pos()
	name := spec.Path.Value

	position, document, _ := i.positionAndDocument(p, pos)
	obj := types.NewPkgName(pos, p.Types, name, pkg.Types)

	rangeID, _ := i.ensureRangeFor(position, obj)
	if ok := i.emitImportMoniker(rangeID, p, obj, document); !ok {
		return
	}

	// TODO(perf): When we have better coverage, it may be possible to skip emitting this.
	_ = i.emitter.EmitTextDocumentHover(rangeID, i.makeCachedHoverResult(nil, obj, func() protocol.MarkupContent {
		return findHoverContents(i.packageDataCache, i.packages, p, obj)
	}))

	document.appendReference(rangeID)
}

// emitImportMonikerNamedDefinition will emit the local, non-exported definition for the named import.
// This will emit the definition for:
//
//    import "fmt"
//                  no local defintion
//
//    import f "fmt"
//           ^----- local definition
func (i *Indexer) emitImportMonikerNamedDefinition(p *packages.Package, pkg *packages.Package, spec *ast.ImportSpec) {
	pos := spec.Name.Pos()
	name := spec.Name.Name
	ident := spec.Name

	// Don't generate a definition if we import directly into the same namespace (i.e. "." imports)
	if name == "." {
		return
	}

	position, document, _ := i.positionAndDocument(p, pos)
	obj := types.NewPkgName(pos, p.Types, name, pkg.Types)

	rangeID, _ := i.ensureRangeFor(position, obj)
	resultSetID := i.emitter.EmitResultSet()
	_ = i.emitter.EmitNext(rangeID, resultSetID)

	i.indexDefinitionForRangeAndResult(p, document, obj, rangeID, resultSetID, false, ident)
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
	caseClauses := map[token.Pos]ObjectLike{}
	for node, obj := range p.TypesInfo.Implicits {
		if _, ok := node.(*ast.CaseClause); ok {
			caseClauses[obj.Pos()] = obj
		}
	}

	for ident, typeObj := range p.TypesInfo.Defs {
		// Must cast because other we have errors from being unable to assign
		// an ObjectLike to a types.Object due to missing things like `color` and other
		// private methods.
		var obj ObjectLike = typeObj

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

		position, document, ok := i.positionAndDocument(p, obj.Pos())
		if !ok {
			continue
		}

		// Always skip types.PkgName because we handle them in emitImports()
		//    we do not want to emit anything new here.
		if _, isPkgName := typeObj.(*types.PkgName); isPkgName {
			continue
		}

		if !i.markRange(position) {
			// This performs a quick assignment to a map that will ensure that
			// we don't race against another routine indexing the same definition
			// reachable from another dataflow path through the indexer. If we
			// lose a race, we'll just bail out and look at the next definition.
			continue
		}

		if typVar, ok := typeObj.(*types.Var); ok {
			if typVar.IsField() && typVar.Anonymous() {
				i.indexDefinitionForAnonymousField(p, document, ident, typVar, position)
				continue
			}
		}

		i.indexDefinition(p, document, position, obj, typeSwitchHeader, ident)
	}
}

// indexDefinitionForAnonymousField will handle anonymous fields definitions.
//
// The reason they have to be handled separately is because they are _both_ a:
// - Defintion
// - Reference
//
// See docs/structs.md for more information.
func (i *Indexer) indexDefinitionForAnonymousField(p *packages.Package, document *DocumentInfo, ident *ast.Ident, typVar *types.Var, position token.Position) {
	// NOTE: Subtract 1 because we are switching indexing strategy (1-based -> 0-based)
	startCol := position.Column - 1

	// To find the end of the identifier, we use the identifier End() Pos and not the length
	// of the name, because there may be package names prefixing the name ("http.Client").
	endCol := p.Fset.Position(ident.End()).Column - 1

	var rangeID uint64
	if endCol-startCol == len(typVar.Name()) {
		rangeID, _ = i.ensureRangeFor(position, typVar)
	} else {
		// This will be a separate range that encompasses _two_ items. So it is kind of
		// "floating" in the nothingness, and should not be looked up in the future when
		// trying to create a new range for whatever occurs at the start position of this location.
		//
		// In other words, this skips setting `i.ranges` for this range.
		//
		// Note to future readers: Do not use EmitRange directly unless you know why you don't want i.ensureRangeFor
		rangeID = i.emitter.EmitRange(
			protocol.Pos{Line: position.Line - 1, Character: startCol},
			protocol.Pos{Line: position.Line - 1, Character: endCol},
		)
	}

	resultSetID := i.emitter.EmitResultSet()
	i.indexDefinitionForRangeAndResult(p, document, typVar, rangeID, resultSetID, false, ident)
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

// markRange sets a zero-size struct into a map for the given position. If this position
// has already been marked, this method returns false.
func (i *Indexer) markRange(pos token.Position) bool {
	i.stripedMutex.RLockKey(pos.Filename)
	_, ok := i.defined[pos.Filename][pos.Offset]
	i.stripedMutex.RUnlockKey(pos.Filename)
	if ok {
		return false
	}

	i.stripedMutex.LockKey(pos.Filename)
	defer i.stripedMutex.UnlockKey(pos.Filename)

	if _, ok := i.defined[pos.Filename][pos.Offset]; ok {
		return false
	}

	i.defined[pos.Filename][pos.Offset] = struct{}{}
	return true
}

// indexDefinitionForRangeAndResult will handle all Indexer related handling of
// a definition for a given rangeID and resultSetID.
func (i *Indexer) indexDefinitionForRangeAndResult(p *packages.Package, document *DocumentInfo, obj ObjectLike, rangeID, resultSetID uint64, typeSwitchHeader bool, ident *ast.Ident) *DefinitionInfo {
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
		_ = i.emitter.EmitTextDocumentHover(resultSetID, i.makeCachedHoverResult(nil, obj, func() protocol.MarkupContent {
			return findHoverContents(i.packageDataCache, i.packages, p, obj)
		}))
	}

	// NOTE: Import monikers are emitted by emitImports, they do not need to be emitted here.

	if obj.Exported() {
		i.emitExportMoniker(resultSetID, p, obj)
	}

	// If the pkg/object has associated documentation, link to it. This enables e.g. going from documentation
	// for a symbol <-> its definition/hover/references/etc in either direction.
	if pkgName, ok := obj.(*types.PkgName); ok {
		if documentationResultID, ok := i.emittedDocumentationResultsByPackagePath[pkgName.Imported().Path()]; ok {
			_ = i.emitter.EmitDocumentationResultEdge(documentationResultID, resultSetID)
		}
	} else if documentationResultID, ok := i.emittedDocumentationResults[obj]; ok {
		_ = i.emitter.EmitDocumentationResultEdge(documentationResultID, resultSetID)
	}

	definitionInfo := &DefinitionInfo{
		DocumentID:         document.DocumentID,
		RangeID:            rangeID,
		ResultSetID:        resultSetID,
		DefinitionResultID: defResultID,
		ReferenceRangeIDs:  map[uint64][]uint64{},
		TypeSwitchHeader:   typeSwitchHeader,
	}
	i.setDefinitionInfo(obj, ident, definitionInfo)

	document.appendDefinition(rangeID)

	return definitionInfo
}

// indexDefinition emits data for the given definition object.
func (i *Indexer) indexDefinition(p *packages.Package, document *DocumentInfo, position token.Position, obj ObjectLike, typeSwitchHeader bool, ident *ast.Ident) *DefinitionInfo {
	// Ensure the range exists, but don't emit a new one as it might already exist due to another
	// phase of indexing (such as symbols) having emitted the range.
	rangeID, _ := i.ensureRangeFor(position, obj)
	resultSetID := i.emitter.EmitResultSet()

	return i.indexDefinitionForRangeAndResult(p, document, obj, rangeID, resultSetID, typeSwitchHeader, ident)
}

// setDefinitionInfo stashes the given definition info indexed by the given object type and name.
// This definition info will be accessible by invoking getDefinitionInfo with the same type and
// name values (but not necessarily the same object).
func (i *Indexer) setDefinitionInfo(obj ObjectLike, ident *ast.Ident, d *DefinitionInfo) {
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
		// solves issue when in the case of a type alias, the obj.Type().String() of the type alias
		// is == to the obj.Type().String() of the type it aliases.
		i.types[ident.String()+"="+obj.Type().String()] = d
		i.typesMutex.Unlock()

	case *types.Var:
		i.varsMutex.Lock()
		i.vars[obj.Pos()] = d
		i.varsMutex.Unlock()

	case *PkgDeclaration:
		// Do nothing -- we don't need to reference these ever again.
		break

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

		pos, document, ok := i.positionAndDocument(p, ident.Pos())
		if !ok {
			continue
		}

		rangeID, ok := i.indexReference(p, document, pos, definitionObj, ident)
		if !ok {
			continue
		}

		document.appendReference(rangeID)
	}
}

// indexReference emits data for the given reference object.
func (i *Indexer) indexReference(p *packages.Package, document *DocumentInfo, pos token.Position, definitionObj ObjectLike, ident *ast.Ident) (uint64, bool) {
	return i.indexReferenceWithDefinitionInfo(p, document, pos, definitionObj, ident, i.getDefinitionInfo(definitionObj, ident))
}

// indexReferenceWithDefinitionInfo emits data for the given reference object and definition info.
// This can be used when the DefinitionInfo is already known, which will skip needing to get and release locks.
func (i *Indexer) indexReferenceWithDefinitionInfo(p *packages.Package, document *DocumentInfo, pos token.Position, definitionObj ObjectLike, ident *ast.Ident, definitionInfo *DefinitionInfo) (uint64, bool) {
	if definitionInfo != nil {
		return i.indexReferenceToDefinition(p, document, pos, definitionObj, definitionInfo)
	} else {
		return i.indexReferenceToExternalDefinition(p, document, pos, definitionObj)
	}
}

// getDefinitionInfo returns the definition info object for the given object. This requires that
// setDefinitionInfo was previously called an object that can be resolved in the same way. This
// will only return definitions which are defined in an index target (not a dependency).
func (i *Indexer) getDefinitionInfo(obj ObjectLike, ident *ast.Ident) *DefinitionInfo {
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
		return i.types[ident.String()+"="+obj.Type().String()]
	case *types.Var:
		return i.vars[v.Pos()]
	case *PkgDeclaration:
		// We don't store definition info for PkgDeclaration.
		// They are never referenced after the first iteration.
	}

	return nil
}

// indexReferenceToDefinition emits data for the given reference object that is defined within
// an index target package.
func (i *Indexer) indexReferenceToDefinition(p *packages.Package, document *DocumentInfo, pos token.Position, definitionObj ObjectLike, d *DefinitionInfo) (uint64, bool) {
	rangeID, ok := i.ensureRangeFor(pos, definitionObj)
	if !ok {
		// Not a new range result; this occurs when the definition and reference
		// ranges overlap (e.g., unnamed nested structs). We attach a defintion
		// edge directly to the range (instead of the result set) so that it takes
		// precedence over the definition attached to the result set (itself).
		//
		// In the case of unnamed nested structs, this supports go to definition
		// from the field to the type definition.
		_ = i.emitter.EmitTextDocumentDefinition(rangeID, d.DefinitionResultID)
	} else {
		// If this was a new range (a reference range only), then we just link the
		// range to the result set of the definition to attach it to the existing
		// graph of definitions, references, and hover text.
		_ = i.emitter.EmitNext(rangeID, d.ResultSetID)
	}

	d.m.Lock()
	d.ReferenceRangeIDs[document.DocumentID] = append(d.ReferenceRangeIDs[document.DocumentID], rangeID)
	d.m.Unlock()

	if d.TypeSwitchHeader {
		// Attach a hover text result _directly_ to the given range so that it "overwrites" the
		// hover result of the type switch header for this use. Each reference of such a variable
		// will need a more specific hover text, as the type of the variable is refined in the body
		// of case clauses of the type switch.
		_ = i.emitter.EmitTextDocumentHover(rangeID, i.makeCachedHoverResult(nil, definitionObj, func() protocol.MarkupContent {
			return findHoverContents(i.packageDataCache, i.packages, p, definitionObj)
		}))
	}

	return rangeID, true
}

// indexReferenceToExternalDefinition emits data for the given reference object that is not defined
// within an index target package. This definition _may_ be resolvable by scanning dependencies, but
// it is not guaranteed.
func (i *Indexer) indexReferenceToExternalDefinition(p *packages.Package, document *DocumentInfo, pos token.Position, definitionObj ObjectLike) (uint64, bool) {
	definitionPkg := definitionObj.Pkg()
	if definitionPkg == nil {
		return 0, false
	}

	// Create a or retreive a hover result identifier keyed by the target object's identifier
	// (scoped ot the object's package name). Caching this gives us another big win as some
	// methods imported from other packages are likely to be used many times in a dependent
	// project (e.g., context.Context, http.Request, etc).
	hoverResultID := i.makeCachedHoverResult(definitionPkg, definitionObj, func() protocol.MarkupContent {
		return findExternalHoverContents(i.packageDataCache, i.packages, p, definitionObj)
	})

	rangeID, _ := i.ensureRangeFor(pos, definitionObj)
	if hoverResultID != 0 {
		_ = i.emitter.EmitTextDocumentHover(rangeID, hoverResultID)
	}

	// Only emit an import moniker which will link to the external definition. If we actually
	// put a textDocument/references result here, we would not traverse to lookup the external defintion
	// via the moniker.
	if ok := i.emitImportMoniker(rangeID, p, definitionObj, document); !ok {
		return 0, false
	}

	return rangeID, true
}

func (i *Indexer) addImportMonikerReference(monikerID, rangeID, documentID uint64) {
	i.importMonikerChannel <- importMonikerReference{monikerID, documentID, rangeID}
}

// ensureRangeFor returns a range identifier for the given object. If a range for the object has
// not been emitted, a new vertex is created.
func (i *Indexer) ensureRangeFor(pos token.Position, obj ObjectLike) (uint64, bool) {
	i.stripedMutex.RLockKey(pos.Filename)
	rangeID, ok := i.ranges[pos.Filename][pos.Offset]
	i.stripedMutex.RUnlockKey(pos.Filename)
	if ok {
		return rangeID, false
	}

	// Note: we calculate this outside of the critical section
	start, end := rangeForObject(obj, pos)

	i.stripedMutex.LockKey(pos.Filename)
	defer i.stripedMutex.UnlockKey(pos.Filename)

	if rangeID, ok := i.ranges[pos.Filename][pos.Offset]; ok {
		return rangeID, false
	}

	rangeID = i.emitter.EmitRange(start, end)
	i.ranges[pos.Filename][pos.Offset] = rangeID
	return rangeID, true
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

func (i *Indexer) linkImportMonikersToRanges() {
	for monikerID, documentReferences := range i.importMonikerReferences {
		// emit one result set and reference result per monikerID
		resultSetID := i.emitter.EmitResultSet()
		referenceResultID := i.emitter.EmitReferenceResult()

		// Link the result set to the correct moniker
		_ = i.emitter.EmitMonikerEdge(resultSetID, monikerID)

		// Link the ranges correctly to the result
		for documentID, rangeSet := range documentReferences {
			rangeIDs := make([]uint64, 0, len(rangeSet))
			for rangeID := range rangeSet {
				rangeIDs = append(rangeIDs, rangeID)

				_ = i.emitter.EmitNext(rangeID, resultSetID)
			}

			_ = i.emitter.EmitTextDocumentReferences(resultSetID, referenceResultID)
			_ = i.emitter.EmitItemOfReferences(referenceResultID, rangeIDs, documentID)
		}

	}
}

// linkContainsToRanges emits the contains relationship for all documents and the ranges that it contains.
func (i *Indexer) linkContainsToRanges() {
	i.visitEachDocument("Emitting contains relations", i.linkContainsForDocument)

	// TODO(efritz) - think about printing a title here
	i.linkContainsForProject()
}

// emitContainsForProject emits a contains edge between a document and its ranges.
func (i *Indexer) linkContainsForDocument(d *DocumentInfo) {
	if len(d.DefinitionRangeIDs) > 0 || len(d.ReferenceRangeIDs) > 0 {
		_ = i.emitter.EmitContains(d.DocumentID, union(d.DefinitionRangeIDs, d.ReferenceRangeIDs))
	}
}

// linkContainsForProject emits a contains edge between the target project and all indexed documents.
func (i *Indexer) linkContainsForProject() {
	documentIDs := make([]uint64, 0, len(i.documents))
	for _, info := range i.documents {
		documentIDs = append(documentIDs, info.DocumentID)
	}

	if len(documentIDs) > 0 {
		_ = i.emitter.EmitContains(i.projectID, documentIDs)
	}
}

func (i *Indexer) indexPackageDeclarations() {
	i.visitEachPackage("Indexing package declarations", i.indexPackageDeclarationForPackage)
}

type DeclInfo struct {
	HasDoc bool
	Path   string
}

// Pick the filename that is the most idiomatic for the defintion of the package.
// This will make jump to def always send you to a better go file than the $PKG_test.go, for example.
func (i *Indexer) indexPackageDeclarationForPackage(p *packages.Package) {
	packageDeclarations := make([]DeclInfo, 0, len(p.Syntax))
	for _, f := range p.Syntax {
		_, position := newPkgDeclaration(p, f)
		packageDeclarations = append(packageDeclarations, DeclInfo{
			HasDoc: f.Doc != nil,
			Path:   position.Filename,
		})
	}

	bestFilename, err := findBestPackageDefinitionPath(p.Name, packageDeclarations)
	if err != nil {
		return
	}

	// First, index the defition, which is the best package info.
	var definitionInfo *DefinitionInfo
	for _, f := range p.Syntax {
		obj, position := newPkgDeclaration(p, f)

		// Skip everything that isn't the best
		if position.Filename != bestFilename {
			continue
		}

		name := obj.Name()
		_, d, ok := i.positionAndDocument(p, obj.Pos())
		if !ok {
			return
		}

		definitionInfo = i.indexDefinition(p, d, position, obj, false, &ast.Ident{
			NamePos: obj.Pos(),
			Name:    name,
			Obj:     nil,
		})

		// Once we've indexed the best one, we can quit this loop
		break
	}

	// Then, index the rest of the files, which are references to that package info.
	for _, f := range p.Syntax {
		obj, position := newPkgDeclaration(p, f)

		// Skip the definition, it is already indexed
		if position.Filename == bestFilename {
			continue
		}

		name := obj.Name()

		_, document, ok := i.positionAndDocument(p, obj.Pos())
		if !ok {
			continue
		}
		ident := &ast.Ident{
			NamePos: obj.Pos(),
			Name:    name,
			Obj:     nil,
		}
		rangeID, ok := i.indexReferenceWithDefinitionInfo(p, document, position, obj, ident, definitionInfo)

		if !ok {
			continue
		}

		i.setRangeForPosition(position, rangeID)
		document.appendReference(rangeID)
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

func (i *Indexer) setRangeForPosition(position token.Position, id uint64) {
	i.stripedMutex.LockKey(position.Filename)
	i.ranges[position.Filename][position.Offset] = id
	i.stripedMutex.UnlockKey(position.Filename)
}

// findBestPackageDefinitionPath searches paths in possiblePaths and finds the one that seems best.
// Chooses one with documentation if possible, otherwise looks for most similar name.
func findBestPackageDefinitionPath(packageName string, possiblePaths []DeclInfo) (string, error) {
	if len(possiblePaths) == 0 {
		return "", errors.New("must have at least one possible path")
	}

	pathsWithDocs := []DeclInfo{}
	for _, v := range possiblePaths {
		if v.HasDoc {
			pathsWithDocs = append(pathsWithDocs, v)
		}
	}

	// The idiomatic way is to _only_ have one .go file per package that has a docstring
	// for the package. This should generally return here.
	if len(pathsWithDocs) == 1 {
		return pathsWithDocs[0].Path, nil
	}

	// If we for some reason have more than one .go file per package that has a docstring,
	// only consider returning paths that contain the docstring (instead of any of the possible
	// paths).
	if len(pathsWithDocs) > 1 {
		possiblePaths = pathsWithDocs
	}

	// Try to only pick non _test files for non _test packages and vice versa.
	possiblePaths = filterBasedOnTestFiles(possiblePaths, packageName)

	// Find the best remaining path.
	// Chooses:
	//     1. doc.go
	//     2. exact match
	//     3. computes levenshtein and picks best score
	minDistance, bestPath := math.MaxInt32, ""
	for _, v := range possiblePaths {
		fileName := fileNameWithoutExtension(v.Path)

		if "doc.go" == path.Base(v.Path) {
			return v.Path, nil
		}

		if packageName == fileName {
			return v.Path, nil
		}

		distance := levenshtein.ComputeDistance(packageName, fileName)
		if distance < minDistance {
			minDistance = distance
			bestPath = v.Path
		}
	}

	return bestPath, nil
}

func fileNameWithoutExtension(fileName string) string {
	return strings.TrimSuffix(fileName, path.Ext(fileName))
}

func filterBasedOnTestFiles(possiblePaths []DeclInfo, packageName string) []DeclInfo {
	packageNameEndsWithTest := strings.HasSuffix(packageName, "_test")

	preferredPaths := []DeclInfo{}
	for _, v := range possiblePaths {
		if packageNameEndsWithTest == strings.HasSuffix(v.Path, "_test.go") {
			preferredPaths = append(preferredPaths, v)
		}
	}

	if len(preferredPaths) > 0 {
		return preferredPaths
	}

	return possiblePaths
}

// listProjectDependencies finds any packages from "$ go list all" that are NOT declared
// as part of the current project.
//
// NOTE: This is different from the other dependencies stored in the indexer because it
// does not modules, but packages.
func (i *Indexer) listProjectDependencies() ([]string, error) {
	output, err := command.Run(i.projectRoot, "go", "list", "all")
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %v\n%s", err, output)
	}

	contained := struct{}{}

	projectPackages := map[string]struct{}{}
	for _, pkg := range i.packages {
		projectPackages[pkg.PkgPath] = contained
	}

	dependencyPackages := []string{"std"}
	for _, dep := range strings.Split(output, "\n") {
		// It's a dependency if it's not in the projectPackages
		if _, ok := projectPackages[dep]; !ok {
			dependencyPackages = append(dependencyPackages, dep)
		}
	}

	return dependencyPackages, nil
}
