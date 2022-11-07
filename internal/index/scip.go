package index

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"github.com/sourcegraph/scip/bindings/go/scip/testutil"
	"golang.org/x/tools/go/packages"
)

const SymbolDefinition = int32(scip.SymbolRole_Definition)

func Parse() {
	// root := "/home/tjdevries/sourcegraph/sourcegraph.git/main/"
	// root := "/home/tjdevries/build/vhs/"
	root := "/home/tjdevries/git/smol_go/"

	index, _ := IndexProject(IndexOpts{
		ModuleRoot:    root,
		ModuleVersion: "0.0.1",
	})

	for _, doc := range index.Documents {
		if root == "/home/tjdevries/build/vhs" && doc.RelativePath != "command.go" {
			continue
		}

		fmt.Println("\nSnapshot:", doc.RelativePath)
		if true {
			formatted, _ := testutil.FormatSnapshot(doc, index, "//", scip.VerboseSymbolFormatter)
			fmt.Println(formatted)
		}
	}
}

type IndexOpts struct {
	ModuleRoot    string
	ModuleVersion string
}

var loadMode = packages.NeedDeps |
	packages.NeedFiles |
	packages.NeedImports |
	packages.NeedSyntax |
	packages.NeedTypes |
	packages.NeedTypesInfo |
	packages.NeedModule |
	packages.NeedName

func IndexProject(opts IndexOpts) (*scip.Index, error) {
	opts.ModuleRoot, _ = filepath.Abs(opts.ModuleRoot)

	moduleRoot := opts.ModuleRoot

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

	cfg := &packages.Config{
		Mode: loadMode,
		Dir:  moduleRoot,
		Logf: nil,

		// Only load tests for the current project.
		// This greatly reduces memory usage when loading dependencies
		Tests: true,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		panic(err)
	}

	normalizeThisPackage(opts, pkgs)

	// TODO: Normalize the std library packages so that
	// we don't have do any special handling later on.
	//
	// This will make our lives a lot easier when reasoning
	// about packages (they will just all be loaded)
	pkgLookup := map[string]*packages.Module{
		"builtin": {
			Path:    "builtin/builtin",
			Version: "go1.19",
		},
	}

	for _, pkg := range pkgs {
		fmt.Println("LOOKING UP", pkg)
		ensureVersionForPackage(pkg)
		pkgLookup[pkg.Name] = pkg.Module

		for name, imp := range pkg.Imports {
			ensureVersionForPackage(imp)
			pkgLookup[name] = imp.Module
		}
	}

	fieldPositionsToSymbol := map[token.Pos]string{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			visitor := StructVisitor{
				mod:    pkg.Module,
				Fields: fieldPositionsToSymbol,
				curScope: []*scip.Descriptor{
					{
						Name:   pkg.PkgPath,
						Suffix: scip.Descriptor_Namespace,
					},
				},
			}

			ast.Walk(visitor, f)
		}
	}

	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			relative, _ := filepath.Rel(moduleRoot, pkg.Fset.File(f.Package).Name())
			doc := scip.Document{
				Language:     "go",
				RelativePath: relative,
				Occurrences:  []*scip.Occurrence{},
				Symbols:      []*scip.SymbolInformation{},
			}

			// Generate import references
			for _, spec := range f.Imports {
				importedPackage := pkg.Imports[strings.Trim(spec.Path.Value, `"`)]
				if importedPackage == nil {
					fmt.Println("Could not find: ", spec.Path)
					continue
				}

				position := pkg.Fset.Position(spec.Pos())
				emitImportReference(&doc, position, importedPackage)
			}

			visitor := FileVisitor{
				doc:       &doc,
				pkg:       pkg,
				file:      f,
				pkgLookup: pkgLookup,
				locals:    map[token.Pos]string{},
				fields:    fieldPositionsToSymbol,
			}

			pkgDescriptor := &scip.Descriptor{
				Name:   pkg.PkgPath,
				Suffix: scip.Descriptor_Namespace,
			}

			for _, decl := range f.Decls {
				switch decl := decl.(type) {
				case *ast.BadDecl:
					continue
				case *ast.GenDecl:
					switch decl.Tok {
					case token.IMPORT:
						// Already handled imports above
						continue
					case token.VAR, token.CONST:
						ast.Walk(VarVisitor{
							doc: &doc,
							pkg: pkg,
							vis: &visitor,
						}, decl)

						continue
					case token.TYPE:
						ast.Walk(TypeVisitor{
							doc:    &doc,
							pkg:    pkg,
							vis:    &visitor,
							fields: fieldPositionsToSymbol,
						}, decl)
					default:
						panic("Unhandled general declaration")
					}

					continue
				case *ast.FuncDecl:
					if decl.Recv != nil {
						ast.Walk(visitor, decl.Recv)
					}

					pos := decl.Name.Pos()
					position := pkg.Fset.Position(pos)

					tPackage := types.NewPackage(pkg.Module.Path, pkg.Name)
					obj := types.NewFunc(decl.Pos(), tPackage, decl.Name.Name, nil)

					desciptors := []*scip.Descriptor{
						pkgDescriptor,
					}

					if recv, has := receiverTypeName(decl); has {
						desciptors = append(desciptors, descriptorType(recv))
					}

					desciptors = append(desciptors, descriptorMethod(decl.Name.Name))

					symbol := scipSymbolFromDescriptors(pkg.Module, desciptors)

					doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
						Range:       scipRange(position, obj),
						Symbol:      symbol,
						SymbolRoles: SymbolDefinition,
					})

					ast.Walk(visitor, decl.Type.Params)
					ast.Walk(visitor, decl.Body)

					if decl.Type.Results != nil {
						ast.Walk(visitor, decl.Type.Results)
					}

					continue
				}

				panic("unreachable")
			}

			index.Documents = append(index.Documents, &doc)
		}
	}

	return &index, nil
}

func normalizeThisPackage(opts IndexOpts, pkgs []*packages.Package) {
	for _, pkg := range pkgs {
		if pkg.Module.Dir == opts.ModuleRoot {
			if pkg.Module.Version == "" {
				pkg.Module.Version = opts.ModuleVersion
			}

			if pkg.Module.Path == "" {
				pkg.Module.Path = opts.ModuleRoot
			}
		}
	}
}

func ensureVersionForPackage(pkg *packages.Package) {
	if pkg.Module != nil {
		return
	}

	fmt.Printf("Ensuring Version for Package: %s | %+v\n", pkg.PkgPath, pkg)

	// TODO: Just use the current stuff for version
	if gomod.IsStandardlibPackge(pkg.PkgPath) {
		pkg.Module = &packages.Module{
			Path:    "github.com/golang/go",
			Version: "v1.19",
			// Main:      false,
			// Indirect:  false,
			// Dir:       "",
			// GoMod:     "",
			// GoVersion: "",
			// Error:     &packages.ModuleError{},
		}

		return
	}

}

func emitImportReference(
	doc *scip.Document,
	position token.Position,
	importedPackage *packages.Package,
) {
	pkgPath := importedPackage.PkgPath
	scipRange := scipRangeFromName(position, pkgPath, true)

	symbol := scipSymbolFromDescriptors(importedPackage.Module, []*scip.Descriptor{descriptorPackage(pkgPath)})
	// symbol := scip.Symbol{
	// 	Scheme: "scip-go",
	// 	Package: &scip.Package{
	// 		Manager: "gomod",
	// 		Name:    importedPackage.Name,
	// 		Version: importedPackage.Module.Version,
	// 	},
	// 	Descriptors: []*scip.Descriptor{{Name: pkgPath, Suffix: scip.Descriptor_Package}},
	// }

	doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
		Range:       scipRange,
		Symbol:      symbol,
		SymbolRoles: int32(scip.SymbolRole_ReadAccess),
	})
}

func makeMonikerPackage(obj types.Object) string {
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

func pkgPath(obj types.Object) string {
	pkg := obj.Pkg()

	// Handle Universe Scoped objs.
	if pkg == nil {
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
		case *types.TypeName:
			universeObj := types.Universe.Lookup(v.Type().String())
			if universeObj != nil {
				return "builtin"
			}
		case *types.Builtin:
			return "builtin"
		case *types.Nil:
			return "builtin"
		case *types.Const:
			universeObj := types.Universe.Lookup(v.Type().String())
			if universeObj != nil {
				return "builtin"
			}
		}

		// Do not allow to fall through to returning pkg.Path()
		//
		// If this becomes a problem more in the future, we can just default to
		// returning "builtin" but as of now this handles all the cases that I
		// know of.
		fmt.Printf("%T %+v (pkg: %s)\n", obj, obj, obj.Pkg())
		// panic("Unhandled nil obj.Pkg()")
		return "builtin"
	}

	return pkg.Path()
}

func scipRangeFromName(position token.Position, name string, adjust bool) []int32 {
	var adjustment int32 = 0
	if adjust {
		adjustment = 1
	}

	line := int32(position.Line - 1)
	column := int32(position.Column - 1)
	n := int32(len(name))

	return []int32{line, column + adjustment, column + n + adjustment}
}

func scipRange(position token.Position, obj types.Object) []int32 {
	var adjustment int32 = 0
	if pkgName, ok := obj.(*types.PkgName); ok && strings.HasPrefix(pkgName.Name(), `"`) {
		adjustment = 1
	}

	line := int32(position.Line - 1)
	column := int32(position.Column - 1)
	n := int32(len(obj.Name()))

	return []int32{line, column + adjustment, column + n - adjustment}
}

func findModuleForObj(dependencies map[string]gomod.GoModule, obj types.Object) *gomod.GoModule {
	pkg := makeMonikerPackage(obj)
	if pkg == "main" || pkg == "" {
		// TODO(modules): Special case...
		x := dependencies["smol_go"]
		return &x
	}

	for _, moduleName := range packagePrefixes(pkg) {
		if module, ok := dependencies[moduleName]; ok {
			return &module
		}
	}

	panic(fmt.Sprintf("Unhandled module: %T %+v || %s\n", obj, obj, makeMonikerPackage(obj)))
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

func scipSymbolFromDescriptors(mod *packages.Module, descriptors []*scip.Descriptor) string {
	return scip.VerboseSymbolFormatter.FormatSymbol(&scip.Symbol{
		Scheme: "scip-go",
		Package: &scip.Package{
			Manager: "gomod",
			// TODO: We might not have a dep, so we should handle that
			Name:    mod.Path,
			Version: mod.Version,
		},
		Descriptors: descriptors,
	})
}

func scipSymbolFromObject(mod *packages.Module, obj types.Object) string {
	if mod == nil {
		panic("Somehow dep was nil...")
	}

	desc := []*scip.Descriptor{
		{Name: makeMonikerPackage(obj), Suffix: scip.Descriptor_Package},
	}
	return scipSymbolFromDescriptors(mod, append(desc, scipDescriptors(obj)...))
}

func scipDescriptors(obj types.Object) []*scip.Descriptor {
	switch obj := obj.(type) {
	case *types.Func:
		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Method},
		}
	case *types.Var:
		if obj.IsField() {
			fmt.Println("OBJ IS FIELD:", obj)

			// inner := obj.Pkg().Scope().Innermost(obj.Pos())
			fmt.Printf("  %T %+v\n", obj.Parent(), obj.Type())
		}

		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Term},
		}
	case *types.Const:
		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Term},
		}
	case *types.TypeName:
		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Type},
		}
	case *types.PkgName:
		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Namespace},
		}
	default:
		fmt.Printf("unknown scip descriptor for type: %T\n", obj)
	}

	return []*scip.Descriptor{}
}

func descriptorType(name string) *scip.Descriptor {
	return &scip.Descriptor{
		Name:   name,
		Suffix: scip.Descriptor_Type,
	}
}

func descriptorMethod(name string) *scip.Descriptor {
	return &scip.Descriptor{
		Name:   name,
		Suffix: scip.Descriptor_Method,
	}
}

func descriptorPackage(name string) *scip.Descriptor {
	return &scip.Descriptor{
		Name:   name,
		Suffix: scip.Descriptor_Package,
	}
}

func descriptorTerm(name string) *scip.Descriptor {
	return &scip.Descriptor{
		Name:   name,
		Suffix: scip.Descriptor_Term,
	}
}

// func nameOf(f *FuncDecl) string {
// 	if r := f.Recv; r != nil && len(r.List) == 1 {
// 		// looks like a correct receiver declaration
// 		t := r.List[0].Type
// 		// dereference pointer receiver types
// 		if p, _ := t.(*StarExpr); p != nil {
// 			t = p.X
// 		}
// 		// the receiver type must be a type name
// 		if p, _ := t.(*Ident); p != nil {
// 			return p.Name + "." + f.Name.Name
// 		}
// 		// otherwise assume a function instead
// 	}
// 	return f.Name.Name
// }

func receiverTypeName(f *ast.FuncDecl) (string, bool) {
	recv := f.Recv
	if recv == nil {
		return "", false
	}

	if len(recv.List) > 1 {
		panic("I don't understand what this would look like")
	} else if len(recv.List) == 0 {
		return "", false
	}

	field := recv.List[0]
	if field.Type == nil {
		return "", false
	}

	// Dereference pointer receiver types
	typ := field.Type
	if p, _ := typ.(*ast.StarExpr); p != nil {
		typ = p.X
	}

	// If we have an identifier, then we have a receiver
	if p, _ := typ.(*ast.Ident); p != nil {
		return p.Name, true
	}

	return "", false
}
