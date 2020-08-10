package indexer

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

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

	// Definition type cache
	consts  map[token.Pos]*DefinitionInfo // position -> info
	funcs   map[string]*DefinitionInfo    // name -> info
	imports map[token.Pos]*DefinitionInfo // position -> info
	labels  map[token.Pos]*DefinitionInfo // position -> info
	types   map[string]*DefinitionInfo    // name -> info
	vars    map[token.Pos]*DefinitionInfo // position -> info

	// LSIF data cache
	documents             map[string]*DocumentInfo        // filename -> info
	ranges                map[string]map[int]uint64       // filename -> offset -> rangeID
	hoverResultCache      map[string]uint64               // cache key -> hoverResultID
	referenceResults      map[uint64]*ReferenceResultInfo // rangeID -> info
	packageInformationIDs map[string]uint64               // name -> packageInformationID
	hoverLoader           *HoverLoader                    // hover text cache
	packages              []*packages.Package             // index target packages
	projectID             uint64                          // project vertex identifier
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
		consts:                map[token.Pos]*DefinitionInfo{},
		funcs:                 map[string]*DefinitionInfo{},
		imports:               map[token.Pos]*DefinitionInfo{},
		labels:                map[token.Pos]*DefinitionInfo{},
		types:                 map[string]*DefinitionInfo{},
		vars:                  map[token.Pos]*DefinitionInfo{},
		documents:             map[string]*DocumentInfo{},
		ranges:                map[string]map[int]uint64{},
		hoverResultCache:      map[string]uint64{},
		referenceResults:      map[uint64]*ReferenceResultInfo{},
		packageInformationIDs: map[string]uint64{},
		hoverLoader:           newHoverLoader(),
	}
}

// Index generates an LSIF dump from a workspace by traversing through source files
// and writing the LSIF equivalent to the output source that implements io.Writer.
// It is caller's responsibility to close the output source if applicable.
func (i *Indexer) Index() (*Stats, error) {
	if err := i.loadPackages(); err != nil {
		return nil, errors.Wrap(err, "loadPackages")
	}

	i.emitMetadataAndProjectVertex()
	i.emitDocuments()
	i.addImports()
	i.preloadHoverText()
	i.indexDefinitions()
	i.indexReferences()
	i.linkReferenceResultsToRanges()
	i.emitContains()

	if err := i.emitter.Flush(); err != nil {
		return nil, errors.Wrap(err, "emitter.Flush")
	}

	if err := i.emitter.Flush(); err != nil {
		return nil, errors.Wrap(err, "emitter.Flush")
	}

	return i.stats(), nil
}

var loadMode = packages.NeedDeps | packages.NeedFiles | packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedName

// packages populates the packages field containing an AST for each package within the configured
// project root.
func (i *Indexer) loadPackages() error {
	wg, errs, count := runParallel(func() (err error) {
		i.packages, err = packages.Load(&packages.Config{
			Mode:  loadMode,
			Dir:   i.projectRoot,
			Tests: true,
		}, "./...")

		return errors.Wrap(err, "packages.Load")
	})

	withProgress(wg, "Loading packages", i.animate, count, 1)
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
	i.visitEachRawFile("Emitting documents", i.animate, i.emitDocument)
}

// emitDocument emits a document vertex and a contains relation to the enclosing project. This method
// also prepares the documents and ranges maps (alternatively: this method must be called before any
// other method that requires the filename key be present in either map).
func (i *Indexer) emitDocument(f FileInfo) {
	// Emit each document only once
	if _, ok := i.documents[f.Filename]; ok {
		return
	}

	// Indexing test files means that we're also indexing the code _generated_ by go test;
	// e.g. file://Users/efritz/Library/Caches/go-build/07/{64-character identifier}-d. Skip
	// These files as they won't be navigable outside of the machine that indexed the project.
	if !strings.HasPrefix(f.Filename, i.projectRoot) {
		return
	}

	documentID := i.emitter.EmitDocument(languageGo, f.Filename)
	_ = i.emitter.EmitContains(i.projectID, []uint64{documentID})
	i.documents[f.Filename] = &DocumentInfo{DocumentID: documentID}
	i.ranges[f.Filename] = map[int]uint64{}
}

// addImports modifies the definitions map of each file to include entries for import statements so
// they can be indexed uniformly in subsequent steps.
func (i *Indexer) addImports() {
	i.visitEachFile("Adding import definitions", i.animate, i.addImportsToFile)
}

// addImportsToFile modifies the definitions map of the given file to include entries for import
// statements so they can be indexed uniformly in subsequent steps.
func (i *Indexer) addImportsToFile(f FileInfo) {
	for _, spec := range f.File.Imports {
		pkg := f.Package.Imports[strings.Trim(spec.Path.Value, `"`)]
		if pkg == nil {
			continue
		}

		name := importSpecName(spec)
		ident := &ast.Ident{NamePos: spec.Pos(), Name: name, Obj: ast.NewObj(ast.Pkg, name)}
		f.Package.TypesInfo.Defs[ident] = types.NewPkgName(spec.Pos(), f.Package.Types, name, pkg.Types)
	}
}

// importSpecName extracts the name from the given import spec.
func importSpecName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.String()
	}

	return spec.Path.Value
}

// preloadHoverText populates the hover loader with hover text for every definition within
// the index target packages, as well as the definitions in all directly imported packages
// (but no transitively imported packages).
func (i *Indexer) preloadHoverText() error {
	var fns []func() error
	for _, p := range getAllReferencedPackages(i.packages) {
		for _, f := range p.Syntax {
			// This is wrapped in an immediately invoked function to ensure
			// we get the correct non-loop values for the package and file.
			fns = append(fns, func(p *packages.Package, f *ast.File) func() error {
				return func() error {
					// TODO(efritz) - this does not depend on the file
					positions := make([]token.Pos, 0, len(p.TypesInfo.Defs))
					for _, obj := range p.TypesInfo.Defs {
						if obj != nil {
							positions = append(positions, obj.Pos())
						}
					}

					i.hoverLoader.Load(f, positions)
					return nil
				}
			}(p, f))
		}
	}

	// Load hovers for each package concurrently
	wg, errs, count := runParallel(fns...)
	withProgress(wg, "Preloading hover text", i.animate, count, len(fns))
	return <-errs
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
	i.visitEachFile("Indexing definitions", i.animate, i.indexDefinitionsForFile)
}

// indexDefinitions emits data for each definition within the given document.
func (i *Indexer) indexDefinitionsForFile(f FileInfo) {
	for ident, obj := range f.Package.TypesInfo.Defs {
		ipos := f.Package.Fset.Position(ident.Pos())

		// Only emit definitions in the current file
		if obj == nil || ipos.Filename != f.Filename {
			continue
		}

		// Already indexed (can happen due to build flags)
		if _, ok := i.ranges[f.Filename][ipos.Offset]; ok {
			continue
		}

		f.Document.DefinitionRangeIDs = append(f.Document.DefinitionRangeIDs, i.indexDefinition(ObjectInfo{
			FileInfo: f,
			Position: ipos,
			Object:   obj,
			Ident:    ident,
		}))
	}
}

// indexDefinition emits data for the given definition object.
func (i *Indexer) indexDefinition(o ObjectInfo) uint64 {
	// Create a hover result vertex and cache the result identifier keyed by the definition location.
	// Caching this gives us a big win for package documentation, which is likely to be large and is
	// repeated at each import and selector within referenced files.
	hoverResultID := i.makeCachedHoverResult(nil, o.Object, func() []protocol.MarkedString {
		return findHoverContents(i.hoverLoader, i.packages, o)
	})

	rangeID := i.emitter.EmitRange(rangeForObject(o))
	resultSetID := i.emitter.EmitResultSet()
	defResultID := i.emitter.EmitDefinitionResult()

	_ = i.emitter.EmitNext(rangeID, resultSetID)
	_ = i.emitter.EmitTextDocumentDefinition(resultSetID, defResultID)
	_ = i.emitter.EmitItem(defResultID, []uint64{rangeID}, o.FileInfo.Document.DocumentID)
	_ = i.emitter.EmitTextDocumentHover(resultSetID, hoverResultID)

	if _, ok := o.Object.(*types.PkgName); ok {
		i.emitImportMoniker(resultSetID, o)
	}

	if o.Ident.IsExported() {
		i.emitExportMoniker(resultSetID, o)
	}

	i.setDefinitionInfo(o.Ident, o.Object, &DefinitionInfo{
		DocumentID:  o.FileInfo.Document.DocumentID,
		RangeID:     rangeID,
		ResultSetID: resultSetID,
	})

	i.referenceResults[rangeID] = &ReferenceResultInfo{
		ResultSetID:        resultSetID,
		DefinitionRangeIDs: map[uint64][]uint64{o.FileInfo.Document.DocumentID: {rangeID}},
		ReferenceRangeIDs:  map[uint64][]uint64{},
	}

	i.ranges[o.Filename][o.Position.Offset] = rangeID
	return rangeID
}

// setDefinitionInfo stashes the given definition info indexed by the given object type and name.
// This definition info will be accessible by invoking getDefinitionInfo with the same type and
// name values (but not necessarily the same object).
func (i *Indexer) setDefinitionInfo(ident *ast.Ident, obj types.Object, d *DefinitionInfo) {
	switch v := obj.(type) {
	case *types.Const:
		i.consts[ident.Pos()] = d
	case *types.Func:
		i.funcs[v.FullName()] = d
	case *types.Label:
		i.labels[ident.Pos()] = d
	case *types.PkgName:
		i.imports[ident.Pos()] = d
	case *types.TypeName:
		i.types[obj.Type().String()] = d
	case *types.Var:
		i.vars[ident.Pos()] = d
	}
}

// indexReferences emits data for each reference in an index target package. This will attach
// the range to a local definition (if one exists), or will emit a result set, a reference result,
// a hover result, and import monikers (for external definitions). This method will also populate
// each document's reference range identifier slice.
func (i *Indexer) indexReferences() {
	i.visitEachFile("Indexing references", i.animate, i.indexReferencesForFile)
}

// indexReferencesForFile emits data for each reference within the given document.
func (i *Indexer) indexReferencesForFile(f FileInfo) {
	for ident, obj := range f.Package.TypesInfo.Uses {
		ipos := f.Package.Fset.Position(ident.Pos())

		// Only emit definitions in the current file
		if ipos.Filename != f.Filename {
			continue
		}

		if rangeID, ok := i.indexReference(ObjectInfo{
			FileInfo: f,
			Position: ipos,
			Object:   obj,
			Ident:    ident,
		}); ok {
			f.Document.ReferenceRangeIDs = append(f.Document.ReferenceRangeIDs, rangeID)
		}
	}
}

// indexReference emits data for the given reference object.
func (i *Indexer) indexReference(o ObjectInfo) (uint64, bool) {
	if def := i.getDefinitionInfo(o.Object); def != nil {
		return i.indexReferenceToDefinition(o, def)
	}

	return i.indexReferenceToExternalDefinition(o)
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
func (i *Indexer) indexReferenceToDefinition(o ObjectInfo, d *DefinitionInfo) (uint64, bool) {
	rangeID := i.ensureRangeFor(o)
	_ = i.emitter.EmitNext(rangeID, d.ResultSetID)

	if refResult := i.referenceResults[d.RangeID]; refResult != nil {
		refResult.ReferenceRangeIDs[o.FileInfo.Document.DocumentID] = append(refResult.ReferenceRangeIDs[o.FileInfo.Document.DocumentID], rangeID)
	}

	return rangeID, true
}

// indexReferenceToExternalDefinition emits data for the given reference object that is not defined
// within an index target package. This definition _may_ be resolvable by scanning dependencies, but
// it is not guaranteed.
func (i *Indexer) indexReferenceToExternalDefinition(o ObjectInfo) (uint64, bool) {
	definitionPkg := o.Object.Pkg()
	if definitionPkg == nil {
		return 0, false
	}

	// Create a or retreive a hover result identifier keyed by the target object's identifier
	// (scoped ot the object's package name). Caching this gives us another big win as some
	// methods imported from other packages are likely to be used many times in a dependent
	// project (e.g., context.Context, http.Request, etc).
	hoverResultID := i.makeCachedHoverResult(definitionPkg, o.Object, func() []protocol.MarkedString {
		return findExternalHoverContents(i.hoverLoader, i.packages, o)
	})

	rangeID := i.ensureRangeFor(o)
	refResultID := i.emitter.EmitReferenceResult()
	_ = i.emitter.EmitTextDocumentReferences(rangeID, refResultID)
	_ = i.emitter.EmitItemOfReferences(refResultID, []uint64{rangeID}, o.FileInfo.Document.DocumentID)

	if hoverResultID != 0 {
		_ = i.emitter.EmitTextDocumentHover(rangeID, hoverResultID)
	}

	i.emitImportMoniker(rangeID, o)
	return rangeID, true
}

// ensureRangeFor returns a range identifier for the given object. If a range for the object has
// not been emitted, a new vertex is created.
func (i *Indexer) ensureRangeFor(o ObjectInfo) uint64 {
	if rangeID, ok := i.ranges[o.Filename][o.Position.Offset]; ok {
		return rangeID
	}

	rangeID := i.emitter.EmitRange(rangeForObject(o))
	i.ranges[o.Filename][o.Position.Offset] = rangeID
	return rangeID
}

// linkReferenceResultsToRanges emits textDocument/definition and textDocument/hover relations
// for each indexed reference result.
func (i *Indexer) linkReferenceResultsToRanges() {
	// TODO(efritz) - emit in a more efficient order
	i.visitEachFile("Linking reference results to ranges", i.animate, i.linkReferenceResultsToRangesInFile)
}

// linkReferenceResultsToRangesInFile links textDocument/definition and textDocument/hover, relations
// for each indexed reference result in the given document.
func (i *Indexer) linkReferenceResultsToRangesInFile(f FileInfo) {
	for _, rangeID := range f.Document.DefinitionRangeIDs {
		referenceResult, ok := i.referenceResults[rangeID]
		if !ok {
			continue
		}

		refResultID := i.emitter.EmitReferenceResult()
		_ = i.emitter.EmitTextDocumentReferences(referenceResult.ResultSetID, refResultID)

		for documentID, rangeIDs := range referenceResult.DefinitionRangeIDs {
			_ = i.emitter.EmitItemOfDefinitions(refResultID, rangeIDs, documentID)
		}

		for documentID, rangeIDs := range referenceResult.ReferenceRangeIDs {
			_ = i.emitter.EmitItemOfReferences(refResultID, rangeIDs, documentID)
		}
	}
}

// emitContains emits the contains relationship for all documents and the ranges that it contains.
func (i *Indexer) emitContains() {
	i.visitEachFile("Emitting contains relations", i.animate, i.emitContainsForFile)
}

// emitContainsForFile emits a contains edge between the given document and the ranges that in contains.
// No edge is emitted if the document contains no ranges.
func (i *Indexer) emitContainsForFile(f FileInfo) {
	if len(f.Document.DefinitionRangeIDs) > 0 || len(f.Document.ReferenceRangeIDs) > 0 {
		_ = i.emitter.EmitContains(f.Document.DocumentID, union(f.Document.DefinitionRangeIDs, f.Document.ReferenceRangeIDs))
	}
}

// stats returns a Stats object with the number of packages, files, and elements analyzed/emitted.
func (i *Indexer) stats() *Stats {
	return &Stats{
		NumPkgs:     uint(len(i.packages)),
		NumFiles:    uint(len(i.documents)),
		NumDefs:     uint(len(i.consts) + len(i.funcs) + len(i.imports) + len(i.labels) + len(i.types) + len(i.vars)),
		NumElements: i.emitter.NumElements(),
	}
}
