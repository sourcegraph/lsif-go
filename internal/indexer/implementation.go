package indexer

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/sourcegraph/lsif-go/internal/output"
	"golang.org/x/tools/container/intsets"
	"golang.org/x/tools/go/packages"
)

type implDef struct {
	pkg           *packages.Package
	typeName      *types.TypeName
	ident         *ast.Ident
	methods       []*types.Selection
	methodsByName map[string]*types.Selection

	// TODO: Consider removing def info and only storing the
	// few items that we actually need.
	defInfo *DefinitionInfo
}

// func (def implDef) forEachImplementationMethod() {}

type implEdge struct {
	from int
	to   int
}

type implRelation struct {
	edges []implEdge
	nodes []implDef
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

// indexImplementations emits data for each implementation of an interface.
//
// NOTE: if indexImplementations becomes multi-threaded then we would need to update
// Indexer.ensureImplementationMoniker to ensure that it uses appropriate locking.
func (i *Indexer) indexImplementations() {
	output.WithProgress("Indexing implementations", func() {

		// Local Implementations
		localInterfaces, localConcreteTypes := i.extractInterfacesAndConcreteTypes(i.packages)

		localRelation := i.buildImplementationRelation(localConcreteTypes, localInterfaces)
		localRelation.forEachImplementation(i.emitLocalImplementation)

		invertedLocalRelation := invert(localRelation)
		invertedLocalRelation.forEachImplementation(i.emitLocalImplementation)

		// Remote Implementations
		remoteInterfaces, remoteConcreteTypes := i.extractInterfacesAndConcreteTypes(i.depPackages)

		localTypesToRemoteInterfaces := i.buildImplementationRelation(localConcreteTypes, filterToExported(remoteInterfaces))
		localTypesToRemoteInterfaces.forEachImplementation(i.emitRemoteImplementation)

		localInterfacesToRemoteTypes := invert(i.buildImplementationRelation(filterToExported(remoteConcreteTypes), localInterfaces))
		localInterfacesToRemoteTypes.forEachImplementation(i.emitRemoteImplementation)

	}, i.outputOptions)
}

// emitLocalImplementation correlates implementations for both structs/interfaces (refered to as typeDefs) and methods.
func (i *Indexer) emitLocalImplementation(from implDef, tos []implDef) {
	typeDefDocToInVs := map[uint64][]uint64{}
	for _, to := range tos {
		documentID := to.defInfo.DocumentID

		if _, ok := typeDefDocToInVs[documentID]; !ok {
			typeDefDocToInVs[documentID] = []uint64{}
		}
		typeDefDocToInVs[documentID] = append(typeDefDocToInVs[documentID], to.defInfo.RangeID)
	}

	// Emit implementation for the typeDefs directly
	i.emitLocalImplementationRelation(from.defInfo.ResultSetID, typeDefDocToInVs)

	// Emit implementation for each of the methods on typeDefs
	for fromName, fromMethod := range from.methodsByName {
		methodDocToInvs := map[uint64][]uint64{}

		fromMethodDef := i.forEachMethodImplementation(tos, fromName, fromMethod, func(to implDef, _ *DefinitionInfo) {
			toMethod := to.methodsByName[fromName]
			toMethodDef := i.getDefinitionInfo(toMethod.Obj(), nil)

			// This method is from an embedded type defined in some dependency.
			if toMethodDef == nil {
				return
			}

			toDocument := toMethodDef.DocumentID
			if _, ok := methodDocToInvs[toDocument]; !ok {
				methodDocToInvs[toDocument] = []uint64{}
			}
			methodDocToInvs[toDocument] = append(methodDocToInvs[toDocument], toMethodDef.RangeID)
		})

		if fromMethodDef == nil {
			continue
		}

		i.emitLocalImplementationRelation(fromMethodDef.ResultSetID, methodDocToInvs)
	}
}

func (i *Indexer) emitLocalImplementationRelation(defResultSetID uint64, documentToInVs map[uint64][]uint64) {
	implResultID := i.emitter.EmitImplementationResult()
	i.emitter.EmitTextDocumentImplementation(defResultSetID, implResultID)

	for documentID, inVs := range documentToInVs {
		i.emitter.EmitItem(implResultID, inVs, documentID)
	}
}

func (i *Indexer) emitRemoteImplementation(from implDef, tos []implDef) {
	for _, to := range tos {
		i.emitImplementationMoniker(from.defInfo.ResultSetID, to.pkg, to.typeName)
	}

	for fromName, fromMethod := range from.methodsByName {
		i.forEachMethodImplementation(tos, fromName, fromMethod, func(to implDef, fromDef *DefinitionInfo) {
			toMethod := to.methodsByName[fromName]
			i.emitImplementationMoniker(fromDef.ResultSetID, to.pkg, toMethod.Obj())
		})
	}
}

func (i *Indexer) forEachMethodImplementation(
	tos []implDef,
	fromName string,
	fromMethod *types.Selection,
	doer func(to implDef, fromDef *DefinitionInfo),
) *DefinitionInfo {
	fromMethodDef := i.getDefinitionInfo(fromMethod.Obj(), nil)

	// This method is from an embedded type defined in some dependency.
	if fromMethodDef == nil {
		return nil
	}

	// if any of the `to` implementations do not have this method,
	// that means this method is NOT part of the required set of
	// methods to be considered an implementation.
	for _, to := range tos {
		if _, ok := to.methodsByName[fromName]; !ok {
			return fromMethodDef
		}
	}

	for _, to := range tos {
		if to.typeName.IsAlias() {
			// Skip aliases because their methods are redundant with
			// the underlying concrete type's methods.
			continue
		}

		doer(to, fromMethodDef)
	}

	return fromMethodDef
}

func (i *Indexer) extractInterfacesAndConcreteTypes(pkgs []*packages.Package) ([]implDef, []implDef) {
	interfaces := []implDef{}
	concreteTypes := []implDef{}
	for _, pkg := range pkgs {
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
			if len(methods) == 0 {
				continue
			}

			methodsByName := map[string]*types.Selection{}
			for _, m := range methods {
				methodsByName[m.Obj().Name()] = m
			}

			d := implDef{
				pkg:           pkg,
				typeName:      typeName,
				ident:         ident,
				defInfo:       i.getDefinitionInfo(typeName, ident),
				methods:       methods,
				methodsByName: methodsByName,
			}
			if types.IsInterface(obj.Type()) {
				interfaces = append(interfaces, d)
			} else {
				concreteTypes = append(concreteTypes, d)
			}
		}
	}

	return interfaces, concreteTypes
}

// buildImplementationRelation builds a map from concrete types to all the interfaces that they implement.
func (i *Indexer) buildImplementationRelation(concreteTypes, interfaces []implDef) implRelation {
	rel := implRelation{
		edges: []implEdge{},
		// Put concrete types and interfaces in the same slice to give them all unique indexes
		nodes: append(concreteTypes, interfaces...),
	}

	// Translates a `concreteTypes` index into a `nodes` index
	concreteTypeIxToNodeIx := func(i int) int {
		// Concrete type nodes come first
		return 0 + i
	}

	// Translates an `interfaces` index into a `nodes` index
	interfaceIxToNodeIx := func(i int) int {
		// Interface nodes come after the concrete types
		return len(concreteTypes) + i
	}

	// stringify a tuple
	tuple := func(t *types.Tuple) []string {
		strs := []string{}
		for i := 0; i < t.Len(); i++ {
			strs = append(strs, t.At(i).Type().String())
		}
		return strs
	}

	// wrap a list of strings with parenths
	parens := func(strs []string) string {
		return "(" + strings.Join(strs, ", ") + ")"
	}

	// Returns a string representation of a method that can be used as a key for finding matches in interfaces.
	canonical := func(m *types.Selection) string {
		signature := m.Type().(*types.Signature)
		returnTypes := tuple(signature.Results())

		ret := ""
		if !m.Obj().Exported() {
			ret += pkgPath(m.Obj()) + ":"
		}
		ret += m.Obj().Name()
		ret += parens(tuple(signature.Params()))
		if len(returnTypes) == 1 {
			ret += " " + returnTypes[0]
		} else if len(returnTypes) > 1 {
			ret += " " + parens(returnTypes)
		}

		return ret
	}

	// Build a map from methods to all their receivers (concrete types that define those methods).
	methodToReceivers := map[string]*intsets.Sparse{}
	for i, t := range concreteTypes {
		for _, method := range t.methods {
			key := canonical(method)
			if _, ok := methodToReceivers[key]; !ok {
				methodToReceivers[key] = &intsets.Sparse{}
			}
			methodToReceivers[key].Insert(i)
		}
	}

	// Loop over all the interfaces and find the concrete types that implement them.
interfaceLoop:
	for i, interfase := range interfaces {
		methods := interfase.methods

		if len(methods) == 0 {
			// Empty interface - skip it.
			continue
		}

		// Find all the concrete types that implement this interface.
		// Types that implement this interface are the intersection
		// of all sets of receivers of all methods in this interface.
		candidateTypes := &intsets.Sparse{}

		if initialReceivers, ok := methodToReceivers[canonical(methods[0])]; !ok {
			continue
		} else {
			candidateTypes.Copy(initialReceivers)
		}

		for _, method := range methods[1:] {
			receivers, ok := methodToReceivers[canonical(method)]
			if !ok {
				continue interfaceLoop
			}

			candidateTypes.IntersectionWith(receivers)
			if candidateTypes.IsEmpty() {
				continue interfaceLoop
			}
		}

		// Add the implementations to the relation.
		for _, ty := range candidateTypes.AppendTo(nil) {
			rel.edges = append(rel.edges, implEdge{concreteTypeIxToNodeIx(ty), interfaceIxToNodeIx(i)})
		}
	}

	return rel
}

// invert reverses the links for edges for a given implRelation
func invert(rel implRelation) implRelation {
	inverse := implRelation{
		edges: []implEdge{},
		nodes: rel.nodes,
	}

	for _, e := range rel.edges {
		inverse.edges = append(inverse.edges, implEdge{from: e.to, to: e.from})
	}
	return inverse
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

// filterToExported removes any nonExported types or identifiers from a list of []implDef
func filterToExported(defs []implDef) []implDef {
	filtered := []implDef{}
	for _, def := range defs {
		if def.typeName.Exported() || def.ident.IsExported() {
			filtered = append(filtered, def)
		}
	}

	return filtered
}
