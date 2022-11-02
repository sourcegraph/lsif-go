package index

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/output"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"github.com/sourcegraph/scip/bindings/go/scip/testutil"
)

func Parse() {
	index, _ := IndexProject("/home/tjdevries/git/smol_go/")

	for _, doc := range index.Documents {
		formatted, _ := testutil.FormatSnapshot(doc, index, "//", scip.VerboseSymbolFormatter)
		fmt.Println("\nSnapshot:", doc.RelativePath)
		fmt.Println(formatted)
	}
}

func IndexProject(moduleRoot string) (*scip.Index, error) {
	outputOptions := output.Options{
		Verbosity:      0,
		ShowAnimations: false,
	}

	dependencies, err := gomod.ListDependencies(moduleRoot, "smol_go", "test", outputOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list dependencies: %v", err)
	}

	index := scip.Index{
		Metadata: &scip.Metadata{
			Version: 0,
			ToolInfo: &scip.ToolInfo{
				Name:      "scip-go",
				Version:   "0.1",
				Arguments: []string{},
			},
			ProjectRoot:          "file://" + moduleRoot,
			TextDocumentEncoding: scip.TextEncoding_UnspecifiedTextEncoding,
		},
		Documents:       []*scip.Document{},
		ExternalSymbols: []*scip.SymbolInformation{},
	}

	getListPkg("builtin", moduleRoot, false)

	pkgs, _ := getListPkg("./...", moduleRoot, true)
	for _, p := range pkgs {
		fmt.Println("")
		fmt.Println("Indexing:", p.Name)

		for _, f := range p.Syntax {
			fmt.Println("\nFile:", f.Name)
			relative, _ := filepath.Rel(moduleRoot, p.Fset.File(f.Package).Name())

			doc := scip.Document{
				Language:     "go",
				RelativePath: relative,
				Occurrences:  []*scip.Occurrence{},
				Symbols:      []*scip.SymbolInformation{},
			}

			// fmt.Println("DECLS:", f.Decls)
			// for _, dec := range f.Decls {
			// 	fmt.Println(dec)
			// 	// case type
			// 	switch concreteDecl := dec.(type) {
			// 	case *ast.GenDecl:
			// 		fmt.Println("GEN DECL:", concreteDecl)
			// 	case *ast.FuncDecl:
			// 		fmt.Println("FUNC DECL:", concreteDecl)
			// 		fmt.Println("	BODY:")
			// 		for _, stmt := range concreteDecl.Body.List {
			// 			fmt.Println(stmt)
			// 		}
			// 	}
			// }

			visitor := FileVisitor{
				deps: dependencies,
				pkg:  p,
				file: f,
				doc:  &doc,
			}

			ast.Walk(visitor, f)

			for _, spec := range f.Imports {
				pkg := p.Imports[strings.Trim(spec.Path.Value, `"`)]
				if pkg == nil {
					fmt.Println("Could not find: ", spec.Path)
					continue
				}

				// fmt.Println("Found Package:", pkg)
				markImportReference(&doc, p, pkg, spec, dependencies)
			}

			// pkg := p.Imports[strings.Trim(spec.Path.Value, `"`)]
			// if pkg == nil {
			// 	continue
			// }
			//
			// i.emitImportMonikerReference(p, pkg, spec)
			//
			// // spec.Name is only non-nil when we have an import of the form:
			// //     import f "fmt"
			// //
			// // So, we want to emit a local defition for the `f` token
			// if spec.Name != nil {
			// 	i.emitImportMonikerNamedDefinition(p, pkg, spec)
			// }

			index.Documents = append(index.Documents, &doc)
		}

		// // indexDefinition
		// caseClauses := map[token.Pos]ObjectLike{}
		// for node, obj := range p.TypesInfo.Implicits {
		// 	if _, ok := node.(*ast.CaseClause); ok {
		// 		caseClauses[obj.Pos()] = obj
		// 	}
		// }
		// for ident, typeObj := range p.TypesInfo.Defs {
		// 	// Must cast because other we have errors from being unable to assign
		// 	// an ObjectLike to a types.Object due to missing things like `color` and other
		// 	// private methods.
		// 	var obj ObjectLike = typeObj
		//
		// 	typeSwitchHeader := false
		// 	if obj == nil {
		// 		// The definitions map contains nil objects for symbolic variables t in t := x.(type)
		// 		// of type switch headers. In these cases we select an arbitrary case clause for the
		// 		// same type switch to index the definition. We mark this object as a typeSwitchHeader
		// 		// so that it can distinguished from other definitions with non-nil objects.
		// 		caseClause, ok := caseClauses[ident.Pos()]
		// 		if !ok {
		// 			continue
		// 		}
		//
		// 		obj = caseClause
		// 		typeSwitchHeader = true
		// 	}
		//
		// 	position, document, ok := i.positionAndDocument(p, obj.Pos())
		// 	if !ok || document == nil {
		// 		continue
		// 	}
		//
		// 	// Always skip types.PkgName because we handle them in emitImports()
		// 	//    we do not want to emit anything new here.
		// 	if _, isPkgName := typeObj.(*types.PkgName); isPkgName {
		// 		continue
		// 	}
		//
		// 	if !i.markRange(position) {
		// 		// This performs a quick assignment to a map that will ensure that
		// 		// we don't race against another routine indexing the same definition
		// 		// reachable from another dataflow path through the indexer. If we
		// 		// lose a race, we'll just bail out and look at the next definition.
		// 		continue
		// 	}
		//
		// 	if typVar, ok := typeObj.(*types.Var); ok {
		// 		if typVar.IsField() && typVar.Anonymous() {
		// 			i.indexDefinitionForAnonymousField(p, document, ident, typVar, position)
		// 			continue
		// 		}
		// 	}
		//
		// 	i.indexDefinition(p, document, position, obj, typeSwitchHeader, ident)
		// }

	}

	return &index, nil
}

func markImportReference(
	doc *scip.Document,
	p *PackageInfo,
	pkg *PackageInfo,
	spec *ast.ImportSpec,
	dependencies map[string]gomod.GoModule,
) {
	pos := spec.Path.Pos()
	position := p.Fset.Position(pos)

	name := spec.Path.Value

	obj := types.NewPkgName(pos, p.Types, name, pkg.Types)
	dep := findModuleForObj(dependencies, obj)

	// moniker := makeMonikerPackage(obj)
	symbol := scip.Symbol{
		Scheme: "scip-go",
		Package: &scip.Package{
			Manager: "gomod",
			// TODO: We might not have a dep, so we should handle that
			Name:    dep.Name,
			Version: dep.Version,
		},
		Descriptors: []*scip.Descriptor{{Name: makeMonikerPackage(obj), Suffix: scip.Descriptor_Package}},
	}

	// gomod.GetGolangDependency()

	doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
		// Range:                 []int32{0, 0, 5},
		Range:                 scipRange(position, obj),
		Symbol:                scip.VerboseSymbolFormatter.FormatSymbol(&symbol),
		SymbolRoles:           int32(scip.SymbolRole_ReadAccess),
		OverrideDocumentation: []string{},
		SyntaxKind:            0,
		Diagnostics:           []*scip.Diagnostic{},
	})

	// i.emitImportMoniker(rangeID, p, obj, document)

	// TODO(perf): When we have better coverage, it may be possible to skip emitting this.
	// _ = i.emitter.EmitTextDocumentHover(rangeID, i.makeCachedHoverResult(nil, obj, func() protocol.MarkupContent {
	// 	return findHoverContents(i.packageDataCache, i.packages, p, obj)
	// }))
	// document.appendReference(rangeID)
}

func makeMonikerPackage(obj ObjectLike) string {
	var pkgName string
	if v, ok := obj.(*types.PkgName); ok {
		// gets the full path of the package name, rather than just the name.
		// So instead of "http", it will return "net/http"
		pkgName = v.Imported().Path()
	} else {
		pkgName = pkgPath(obj)
	}

	return gomod.NormalizeMonikerPackage(pkgName)
}

func pkgPath(obj ObjectLike) string {
	pkg := obj.Pkg()

	// Handle Universe Scoped objs.
	if pkg == nil {
		// Here be dragons:
		switch v := obj.(type) {
		case *types.Func:
			switch typ := v.Type().(type) {
			case *types.Signature:
				recv := typ.Recv()
				universeObj := types.Universe.Lookup(recv.Type().String())
				if universeObj != nil {
					return "builtin"
				}
			}
		}

		// Do not allow to fall through to returning pkg.Path()
		//
		// If this becomes a problem more in the future, we can just default to
		// returning "builtin" but as of now this handles all the cases that I
		// know of.
		panic("Unhandled nil obj.Pkg()")
	}

	return pkg.Path()
}

func scipRange(position token.Position, obj ObjectLike) []int32 {
	var adjustment int32 = 0
	if pkgName, ok := obj.(*types.PkgName); ok && strings.HasPrefix(pkgName.Name(), `"`) {
		adjustment = 1
	}

	line := int32(position.Line - 1)
	column := int32(position.Column - 1)
	n := int32(len(obj.Name()))

	return []int32{line, column + adjustment, column + n - adjustment}
}

func findModuleForObj(dependencies map[string]gomod.GoModule, obj ObjectLike) *gomod.GoModule {
	pkg := makeMonikerPackage(obj)
	for _, moduleName := range packagePrefixes(pkg) {
		if module, ok := dependencies[moduleName]; ok {
			return &module
		}
	}

	return nil
}

func emitImportMoniker(dependencies map[string]gomod.GoModule, obj ObjectLike) bool {
	// pkg := makeMonikerPackage(obj)
	// monikerIdentifier := joinMonikerParts(pkg, makeMonikerIdentifier(i.packageDataCache, p, obj))

	// for _, moduleName := range packagePrefixes(pkg) {
	// 	if module, ok := dependencies[moduleName]; ok {
	// 		// Lazily emit package information vertex
	// 		packageInformationID := i.ensurePackageInformation(module.Name, module.Version)
	//
	// 		// Lazily emit moniker vertex
	// 		monikerID := i.ensureImportMoniker(monikerIdentifier, packageInformationID)
	//
	// 		// Monikers will be linked during Indexer.linkImportMonikersToRanges
	// 		i.addImportMonikerReference(monikerID, rangeID, document.DocumentID)
	//
	// 		return true
	// 	}
	// }

	return false
}

// packagePrefixes returns all prefix of the go package path. For example, the package
// `foo/bar/baz` will return the slice containing `foo/bar/baz`, `foo/bar`, and `foo`.
func packagePrefixes(packageName string) []string {
	parts := strings.Split(packageName, "/")
	prefixes := make([]string, len(parts))

	for i := 1; i <= len(parts); i++ {
		prefixes[len(parts)-i] = strings.Join(parts[:i], "/")
	}

	return prefixes
}

// A Visitor's Visit method is invoked for each node encountered by Walk.
// If the result visitor w is not nil, Walk visits each of the children
// of node with the visitor w, followed by a call of w.Visit(nil).
// type Visitor interface { Visit(node Node) (w Visitor) }

type FileVisitor struct {
	deps map[string]gomod.GoModule
	pkg  *PackageInfo
	doc  *scip.Document
	file *ast.File
}

func (f FileVisitor) Visit(n ast.Node) (w ast.Visitor) {
	if n == nil {
		return nil
	}

	switch node := n.(type) {
	case *ast.File:
		// explicit pass
	case *ast.FuncDecl:
		// node.Name
		fmt.Println(node.Name.Name)
	case *ast.Ident:
		fmt.Println("=======")
		info := f.pkg.TypesInfo

		def := info.Defs[node]
		ref := info.Uses[node]

		// Don't think anything can be a def and a ref
		if def != nil && ref != nil {
			panic("Didn't think this was possible")
		}

		pos := node.NamePos
		position := f.pkg.Fset.Position(pos)

		// Append definition
		if def != nil {
			f.doc.Occurrences = append(f.doc.Occurrences, &scip.Occurrence{
				Range:       scipRange(position, def),
				Symbol:      scipSymbol(findModuleForObj(f.deps, def), def),
				SymbolRoles: int32(scip.SymbolRole_Definition),
			})
		} else if ref != nil {
			f.doc.Occurrences = append(f.doc.Occurrences, &scip.Occurrence{
				Range:       scipRange(position, ref),
				Symbol:      scipSymbol(findModuleForObj(f.deps, ref), ref),
				SymbolRoles: int32(scip.SymbolRole_ReadAccess),
			})
		}

		fmt.Println("Definition: ", def)
		fmt.Println("References: ", ref)
	default:
		fmt.Printf("unhandled: %T %v\n", n, n)
	}

	return f
}

func scipSymbol(dep *gomod.GoModule, obj ObjectLike) string {
	desc := []*scip.Descriptor{
		{Name: makeMonikerPackage(obj), Suffix: scip.Descriptor_Package},
	}

	desc = append(desc, scipDescriptors(obj)...)

	return scip.VerboseSymbolFormatter.FormatSymbol(&scip.Symbol{
		Scheme: "scip-go",
		Package: &scip.Package{
			Manager: "gomod",
			// TODO: We might not have a dep, so we should handle that
			Name:    dep.Name,
			Version: dep.Version,
		},
		Descriptors: desc,
	})
}

func scipDescriptors(obj ObjectLike) []*scip.Descriptor {
	switch obj := obj.(type) {
	case *types.Func:
		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Method},
		}
	case *types.Var:
		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Term},
		}
	}

	fmt.Printf("TYPE OF OBJ: %T\n", obj)

	return []*scip.Descriptor{}
}
