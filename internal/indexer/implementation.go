package indexer

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/container/intsets"
	"golang.org/x/tools/go/packages"
)

type implementationDef struct {
	pkg           *packages.Package
	typeName      *types.TypeName
	ident         *ast.Ident
	defInfo       *DefinitionInfo
	methods       []*types.Selection
	methodsByName map[string]*types.Selection
}

type implementationEdge struct {
	from int
	to   int
}

type implementationRelation struct {
	edges []implementationEdge
	nodes []implementationDef
}

func forEachImplementation(rel implementationRelation, f func(from implementationDef, to []implementationDef)) {
	m := map[int][]implementationDef{}
	for _, e := range rel.edges {
		if _, ok := m[e.from]; !ok {
			m[e.from] = []implementationDef{}
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
	localInterfaces, localConcreteTypes := i.extractInterfacesAndConcreteTypes(i.packages)
	localRelation := i.buildImplementationRelation(localConcreteTypes, localInterfaces)
	forEachImplementation(localRelation, i.emitLocalImplementation)
	forEachImplementation(invert(localRelation), i.emitLocalImplementation)

	remoteInterfaces, remoteConcreteTypes := i.extractInterfacesAndConcreteTypes(i.depPackages)
	forEachImplementation(i.buildImplementationRelation(localConcreteTypes, filterToExported(remoteInterfaces)), i.emitRemoteImplementation)
	forEachImplementation(invert(i.buildImplementationRelation(filterToExported(remoteConcreteTypes), localInterfaces)), i.emitRemoteImplementation)
}

func filterToExported(defs []implementationDef) []implementationDef {
	filtered := []implementationDef{}
	for _, def := range defs {
		if def.typeName.Exported() || def.ident.IsExported() {
			filtered = append(filtered, def)
		}
	}

	return filtered
}

func (i *Indexer) emitLocalImplementation(from implementationDef, tos []implementationDef) {
	typeDocToInvs := map[uint64][]uint64{}
	for _, to := range tos {
		if _, ok := typeDocToInvs[to.defInfo.DocumentID]; !ok {
			typeDocToInvs[to.defInfo.DocumentID] = []uint64{}
		}
		typeDocToInvs[to.defInfo.DocumentID] = append(typeDocToInvs[to.defInfo.DocumentID], to.defInfo.RangeID)
	}
	implementationResult := i.emitter.EmitImplementationResult()
	i.emitter.EmitTextDocumentImplementation(from.defInfo.ResultSetID, implementationResult)
	for doc, inVs := range typeDocToInvs {
		i.emitter.EmitItem(implementationResult, inVs, doc)
	}

methodLoop:
	for name, method := range from.methodsByName {
		fromMethod := i.getDefinitionInfo(method.Obj(), nil)
		if fromMethod == nil {
			// This method is from an embedded type defined in some dependency.
			continue
		}
		methodDocToInvs := map[uint64][]uint64{}
		for _, to := range tos {
			if to.typeName.IsAlias() {
				// Skip aliases because their methods are redundant with
				// the underlying concrete type's methods.
				continue
			}

			toMethod, ok := to.methodsByName[name]
			if !ok {
				// This is an extraneous method on the concrete type `from`
				// unrelated to the interface `to`, so skip it.
				continue methodLoop
			}

			toObj := toMethod.Obj()
			toMethodDef := i.getDefinitionInfo(toObj, nil)
			if toMethodDef == nil {
				// This method is from an embedded type defined in some dependency.
				continue
			}
			if _, ok := methodDocToInvs[toMethodDef.DocumentID]; !ok {
				methodDocToInvs[toMethodDef.DocumentID] = []uint64{}
			}
			methodDocToInvs[toMethodDef.DocumentID] = append(methodDocToInvs[toMethodDef.DocumentID], toMethodDef.RangeID)
		}

		implementationResult := i.emitter.EmitImplementationResult()
		i.emitter.EmitTextDocumentImplementation(fromMethod.ResultSetID, implementationResult)
		for doc, inVs := range methodDocToInvs {
			i.emitter.EmitItem(implementationResult, inVs, doc)
		}
	}
}

func (i *Indexer) emitRemoteImplementation(from implementationDef, tos []implementationDef) {
	for _, to := range tos {
		i.emitImplementationMoniker(from.defInfo.ResultSetID, to.pkg, to.typeName)
	}

methodLoop:
	for name, method := range from.methodsByName {
		fromMethod := i.getDefinitionInfo(method.Obj(), nil)
		if fromMethod == nil {
			// This method is from an embedded type defined in some dependency.
			continue
		}
		for _, to := range tos {
			if to.typeName.IsAlias() {
				// Skip aliases because their methods are redundant with
				// the underlying concrete type's methods.
				continue
			}

			toMethod, ok := to.methodsByName[name]
			if !ok {
				// This is an extraneous method on the concrete type `from`
				// unrelated to the interface `to`, so skip it.
				continue methodLoop
			}

			i.emitImplementationMoniker(fromMethod.ResultSetID, to.pkg, toMethod.Obj())
		}
	}
}

func (i *Indexer) extractInterfacesAndConcreteTypes(pkgs []*packages.Package) ([]implementationDef, []implementationDef) {
	interfaces := []implementationDef{}
	concreteTypes := []implementationDef{}
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

			d := implementationDef{
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
func (i *Indexer) buildImplementationRelation(concreteTypes, interfaces []implementationDef) implementationRelation {
	rel := implementationRelation{
		edges: []implementationEdge{},
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

	// Returns a string representation of a method that can be used as a key for finding matches in interfaces.
	canonical := func(m *types.Selection) string {
		tuple := func(t *types.Tuple) []string {
			strs := []string{}
			for i := 0; i < t.Len(); i++ {
				strs = append(strs, t.At(i).Type().String())
			}
			return strs
		}

		parens := func(strs []string) string {
			return "(" + strings.Join(strs, ", ") + ")"
		}

		signature := m.Type().(*types.Signature)
		returnTypes := tuple(signature.Results())

		ret := ""
		if !m.Obj().Exported() {
			ret += m.Obj().Pkg().Path() + ":"
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
			rel.edges = append(rel.edges, implementationEdge{concreteTypeIxToNodeIx(ty), interfaceIxToNodeIx(i)})
		}
	}

	return rel
}
func invert(rel implementationRelation) implementationRelation {
	inverse := implementationRelation{
		edges: []implementationEdge{},
		nodes: rel.nodes,
	}
	for _, e := range rel.edges {
		inverse.edges = append(inverse.edges, implementationEdge{from: e.to, to: e.from})
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
