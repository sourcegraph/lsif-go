package indexer

import (
	"go/types"
	"runtime"
	"strings"

	"github.com/sourcegraph/lsif-go/internal/output"
	"golang.org/x/tools/container/intsets"
	"golang.org/x/tools/go/packages"
)

type implDef struct {
	defInfo            *DefinitionInfo
	identIsExported    bool
	methods            []string
	methodsByName      map[string]methodInfo
	monikerPackage     string
	monikerIdentifier  string
	typeNameIsExported bool
	typeNameIsAlias    bool
}

type methodInfo struct {
	definition        *DefinitionInfo
	monikerIdentifier string
}

func (def implDef) Exported() bool {
	return def.typeNameIsExported || def.identIsExported
}

type implEdge struct {
	from int
	to   int
}

type implRelation struct {
	edges []implEdge

	// concatenated list of (concreteTypes..., interfaces...)
	// this gives every type and interface a unique index.
	nodes []implDef

	// offset index for where interfaces start
	ifaceOffset int
}

func (rel implRelation) forEachImplementation(f func(from implDef, to []implDef)) {
	m := map[int][]implDef{}
	for _, e := range rel.edges {
		if _, ok := m[e.from]; !ok {
			m[e.from] = []implDef{}
		}
		m[e.from] = append(m[e.from], rel.nodes[e.to])
	}

	for fromi, tos := range m {
		f(rel.nodes[fromi], tos)
	}
}

// invert reverses the links for edges for a given implRelation
func (rel implRelation) invert() implRelation {
	inverse := implRelation{
		edges:       []implEdge{},
		nodes:       rel.nodes,
		ifaceOffset: rel.ifaceOffset,
	}

	for _, e := range rel.edges {
		inverse.edges = append(inverse.edges, implEdge{from: e.to, to: e.from})
	}
	return inverse
}

// Translates a `concreteTypes` index into a `nodes` index
func (rel implRelation) concreteTypeIxToNodeIx(idx int) int {
	// Concrete type nodes come first
	return idx
}

// Translates an `interfaces` index into a `nodes` index
func (rel implRelation) interfaceIxToNodeIx(idx int) int {
	// Interface nodes come after the concrete types
	return rel.ifaceOffset + idx
}

func (rel *implRelation) linkInterfaceToReceivers(idx int, interfaceMethods []string, methodToReceivers map[string]*intsets.Sparse) {
	// Empty interface - skip it.
	if len(interfaceMethods) == 0 {
		return
	}

	// Find all the concrete types that implement this interface.
	// Types that implement this interface are the intersection
	// of all sets of receivers of all methods in this interface.
	candidateTypes := &intsets.Sparse{}

	// The rest of this function is effectively "fold" (for those CS PhDs out there).
	//
	// > I think the underlying logic here is really beautiful but the syntax
	// > makes it a bit messy and really obscures the intent and simplicity
	// > behind it
	//
	//    - Dr. Fritz

	// If it doesn't match on the first method, then we can immediately quit.
	// Concrete types must _always_ implement all the methods
	if initialReceivers, ok := methodToReceivers[interfaceMethods[0]]; !ok {
		return
	} else {
		candidateTypes.Copy(initialReceivers)
	}

	// Loop over the rest of the methods and find all the types that intersect
	// every method of the interface.
	for _, method := range interfaceMethods[1:] {
		receivers, ok := methodToReceivers[method]
		if !ok {
			return
		}

		candidateTypes.IntersectionWith(receivers)
		if candidateTypes.IsEmpty() {
			return
		}
	}

	// Add the implementations to the relation.
	for _, ty := range candidateTypes.AppendTo(nil) {
		rel.edges = append(rel.edges, implEdge{rel.concreteTypeIxToNodeIx(ty), rel.interfaceIxToNodeIx(idx)})
	}
}

// indexImplementations emits data for each implementation of an interface.
//
// NOTE: if indexImplementations becomes multi-threaded then we would need to update
// Indexer.ensureImplementationMoniker to ensure that it uses appropriate locking.
func (i *Indexer) indexImplementations() error {
	if !i.generationOptions.EnableImplementations {
		return nil
	}

	var implErr error

	output.WithProgress("Indexing implementations", func() {
		// When considering the connections we want to draw between the following four categories:
		//   - LocalInterfaces: Interfaces created in the currently project
		//   - LocalTypes: Concrete Types created in the currently project
		//
		//   - RemoteTypes: Concrete Types created in one of the dependencies of the current project
		//   - RemoteInterfaces: Interfaces created in one of the dependencies of the current project
		//
		// We want to connect the four categories like this:
		//
		// ```ascii_art
		// LocalInterfaces   <------------------> LocalConcreteTypes
		//       |                                   |
		//       |                                   |
		//       v                                   v
		//  RemoteConcreteTypes        X         RemoteInterfaces
		// ```
		//
		// NOTES:
		// - We do not need to connect RemoteTypes and RemoteInterfaces because those connections will
		//   be made when we index those projects.
		// - We do not connect Interfaces w/ Interfaces or Types w/ Types, so there is no need to make those
		//   connectsion between the local and remote interfaces/types.

		// =========================
		// Local Implementations
		localInterfaces, localConcreteTypes, err := i.extractInterfacesAndConcreteTypes([]string{"./..."})
		if err != nil {
			implErr = err
			return
		}

		// LocalConcreteTypes -> LocalInterfaces
		localRelation := buildImplementationRelation(localConcreteTypes, localInterfaces)
		localRelation.forEachImplementation(i.emitLocalImplementation)

		// LocalInterfaces -> LocalConcreteTypes
		invertedLocalRelation := localRelation.invert()
		invertedLocalRelation.forEachImplementation(i.emitLocalImplementation)

		// =========================
		// Remote Implementations
		remoteInterfaces, remoteConcreteTypes, err := i.extractInterfacesAndConcreteTypes(i.projectDependencies)
		if err != nil {
			implErr = err
			return
		}

		// LocalConcreteTypes -> RemoteInterfaces (exported only)
		localTypesToRemoteInterfaces := buildImplementationRelation(localConcreteTypes, filterToExported(remoteInterfaces))
		localTypesToRemoteInterfaces.forEachImplementation(i.emitRemoteImplementation)

		// RemoteConcreteTypes (exported only) -> LocalInterfaces
		localInterfacesToRemoteTypes := buildImplementationRelation(filterToExported(remoteConcreteTypes), localInterfaces).invert()
		localInterfacesToRemoteTypes.forEachImplementation(i.emitRemoteImplementation)

	}, i.outputOptions)

	return implErr
}

// emitLocalImplementation correlates implementations for both structs/interfaces (refered to as typeDefs) and methods.
func (i *Indexer) emitLocalImplementation(from implDef, tos []implDef) {
	typeDefDocToInVs := map[uint64][]uint64{}
	for _, to := range tos {
		if to.defInfo == nil {
			continue
		}

		documentID := to.defInfo.DocumentID

		if _, ok := typeDefDocToInVs[documentID]; !ok {
			typeDefDocToInVs[documentID] = []uint64{}
		}
		typeDefDocToInVs[documentID] = append(typeDefDocToInVs[documentID], to.defInfo.RangeID)
	}

	if from.defInfo != nil {
		// Emit implementation for the typeDefs directly
		i.emitLocalImplementationRelation(from.defInfo.ResultSetID, typeDefDocToInVs)
	}

	// Emit implementation for each of the methods on typeDefs
	for fromName, fromMethod := range from.methodsByName {
		methodDocToInvs := map[uint64][]uint64{}

		fromMethodDef := i.forEachMethodImplementation(tos, fromName, fromMethod, func(to implDef, _ *DefinitionInfo) {
			toMethod := to.methodsByName[fromName]

			// This method is from an embedded type defined in some dependency.
			if toMethod.definition == nil {
				return
			}

			toDocument := toMethod.definition.DocumentID
			if _, ok := methodDocToInvs[toDocument]; !ok {
				methodDocToInvs[toDocument] = []uint64{}
			}
			methodDocToInvs[toDocument] = append(methodDocToInvs[toDocument], toMethod.definition.RangeID)
		})

		if fromMethodDef == nil {
			continue
		}

		i.emitLocalImplementationRelation(fromMethodDef.ResultSetID, methodDocToInvs)
	}
}

// emitLocalImplementationRelation emits the required LSIF nodes for an implementation
func (i *Indexer) emitLocalImplementationRelation(defResultSetID uint64, documentToInVs map[uint64][]uint64) {
	implResultID := i.emitter.EmitImplementationResult()
	i.emitter.EmitTextDocumentImplementation(defResultSetID, implResultID)

	for documentID, inVs := range documentToInVs {
		i.emitter.EmitItem(implResultID, inVs, documentID)
	}
}

// emitRemoteImplementation emits implementation monikers
// (kind: "implementation") to connect remote implementations
func (i *Indexer) emitRemoteImplementation(from implDef, tos []implDef) {
	for _, to := range tos {
		if from.defInfo == nil {
			continue
		}
		i.emitImplementationMoniker(from.defInfo.ResultSetID, to.monikerPackage, to.monikerIdentifier)
	}

	for fromName, fromMethod := range from.methodsByName {
		i.forEachMethodImplementation(tos, fromName, fromMethod, func(to implDef, fromDef *DefinitionInfo) {
			toMethod := to.methodsByName[fromName]
			i.emitImplementationMoniker(fromDef.ResultSetID, to.monikerPackage, toMethod.monikerIdentifier)
		})
	}
}

// forEachMethodImplementation will call callback for each to in tos when the
// method is a method that is properly implemented.
//
// It returns the definition of the method that can be linked for each of the
// associated tos
func (i *Indexer) forEachMethodImplementation(
	tos []implDef,
	fromName string,
	fromMethod methodInfo,
	callback func(to implDef, fromDef *DefinitionInfo),
) *DefinitionInfo {
	// This method is from an embedded type defined in some dependency.
	if fromMethod.definition == nil {
		return nil
	}

	// if any of the `to` implementations do not have this method,
	// that means this method is NOT part of the required set of
	// methods to be considered an implementation.
	for _, to := range tos {
		if _, ok := to.methodsByName[fromName]; !ok {
			return fromMethod.definition
		}
	}

	for _, to := range tos {
		// Skip aliases because their methods are redundant with
		// the underlying concrete type's methods.
		if to.typeNameIsAlias {
			continue
		}

		callback(to, fromMethod.definition)
	}

	return fromMethod.definition
}

// extractInterfacesAndConcreteTypes constructs a list of interfaces and
// concrete types from the list of given packages.
func (i *Indexer) extractInterfacesAndConcreteTypes(pkgNames []string) (interfaces []implDef, concreteTypes []implDef, err error) {
	visit := func(pkg *packages.Package) {
		for ident, obj := range pkg.TypesInfo.Defs {
			if obj == nil {
				continue
			}

			// We ignore aliases 'type M = N' to avoid duplicate reporting
			// of the Named type N.
			typeName, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}

			if _, ok := obj.Type().(*types.Named); !ok {
				continue
			}

			methods := listMethods(obj.Type().(*types.Named))

			canonicalizedMethods := []string{}
			for _, m := range methods {
				canonicalizedMethods = append(canonicalizedMethods, canonicalize(m))
			}

			// ignore interfaces that are empty. they are too
			// plentiful and don't provide useful intelligence.
			if len(methods) == 0 {
				continue
			}

			methodsByName := map[string]methodInfo{}
			for _, m := range methods {
				methodsByName[m.Obj().Name()] = methodInfo{
					definition:        i.getDefinitionInfo(m.Obj(), nil),
					monikerIdentifier: joinMonikerParts(makeMonikerPackage(m.Obj()), makeMonikerIdentifier(i.packageDataCache, pkg, m.Obj())),
				}
			}

			monikerPackage := makeMonikerPackage(obj)

			d := implDef{
				monikerPackage:     monikerPackage,
				monikerIdentifier:  joinMonikerParts(monikerPackage, makeMonikerIdentifier(i.packageDataCache, pkg, obj)),
				typeNameIsExported: typeName.Exported(),
				typeNameIsAlias:    typeName.IsAlias(),
				identIsExported:    ident.IsExported(),
				defInfo:            i.getDefinitionInfo(typeName, ident),
				methods:            canonicalizedMethods,
				methodsByName:      methodsByName,
			}
			if types.IsInterface(obj.Type()) {
				interfaces = append(interfaces, d)
			} else {
				concreteTypes = append(concreteTypes, d)
			}
		}
	}

	batch := func(pkgBatch []string) error {
		pkgs, err := i.loadPackage(true, pkgBatch...)
		if err != nil {
			return err
		}

		for _, pkg := range pkgs {
			visit(pkg)
		}
		return nil
	}

	pkgBatch := []string{}
	for ix, pkgName := range pkgNames {
		pkgBatch = append(pkgBatch, pkgName)

		if i.generationOptions.DepBatchSize != 0 && ix%i.generationOptions.DepBatchSize == 0 {
			err := batch(pkgBatch)
			runtime.GC() // Prevent a garbage pile
			if err != nil {
				return nil, nil, err
			}
			pkgBatch = pkgBatch[:0]
		}
	}
	if err := batch(pkgBatch); err != nil {
		return nil, nil, err
	}

	return interfaces, concreteTypes, nil
}

// buildImplementationRelation builds a map from concrete types to all the interfaces that they implement.
func buildImplementationRelation(concreteTypes, interfaces []implDef) implRelation {
	rel := implRelation{
		edges:       []implEdge{},
		nodes:       append(concreteTypes, interfaces...),
		ifaceOffset: len(concreteTypes),
	}

	// Build a map from methods to all their receivers (concrete types that define those methods).
	methodToReceivers := map[string]*intsets.Sparse{}
	for idx, t := range concreteTypes {
		for _, method := range t.methods {
			if _, ok := methodToReceivers[method]; !ok {
				methodToReceivers[method] = &intsets.Sparse{}
			}
			methodToReceivers[method].Insert(idx)
		}
	}

	// Loop over all the interfaces and find the concrete types that implement them.
	for idx, interfase := range interfaces {
		rel.linkInterfaceToReceivers(idx, interfase.methods, methodToReceivers)
	}

	return rel
}

// listMethods returns the method set for a named type T
// merged with all the methods of *T that have different names than
// the methods of T.
//
// Copied from https://github.com/golang/tools/blob/1a7ca93429f83e087f7d44d35c0e9ea088fc722e/cmd/godex/print.go#L355
func listMethods(T *types.Named) []*types.Selection {
	// method set for T
	mset := types.NewMethodSet(T)
	var res []*types.Selection
	for i, n := 0, mset.Len(); i < n; i++ {
		res = append(res, mset.At(i))
	}

	// add all *T methods with names different from T methods
	pmset := types.NewMethodSet(types.NewPointer(T))
	for i, n := 0, pmset.Len(); i < n; i++ {
		pm := pmset.At(i)
		if obj := pm.Obj(); mset.Lookup(obj.Pkg(), obj.Name()) == nil {
			res = append(res, pm)
		}
	}

	return res
}

// Returns a string representation of a method that can be used as a key for finding matches in interfaces.
func canonicalize(m *types.Selection) string {
	builder := strings.Builder{}

	writeTuple := func(t *types.Tuple) {
		for i := 0; i < t.Len(); i++ {
			builder.WriteString(t.At(i).Type().String())
		}
	}

	signature := m.Type().(*types.Signature)

	// if an object is not exported, then we need to make the canonical
	// representation of the object not able to match any other representations
	if !m.Obj().Exported() {
		builder.WriteString(pkgPath(m.Obj()))
		builder.WriteString(":")
	}

	builder.WriteString(m.Obj().Name())
	builder.WriteString("(")
	writeTuple(signature.Params())
	builder.WriteString(")")

	returnTypes := signature.Results()
	returnLen := returnTypes.Len()
	if returnLen == 0 {
		// Don't add anything
	} else if returnLen == 1 {
		builder.WriteString(" ")
		writeTuple(returnTypes)
	} else {
		builder.WriteString(" (")
		writeTuple(returnTypes)
		builder.WriteString(")")
	}

	// fmt.Println(builder.String())
	return builder.String()
}

// filterToExported removes any nonExported types or identifiers from a list of []implDef
// NOTE: defs is modified in place by this function.
func filterToExported(defs []implDef) []implDef {
	// filter in place.
	filtered := defs[:0]

	for _, def := range defs {
		if def.Exported() {
			filtered = append(filtered, def)
		}
	}

	return filtered
}
