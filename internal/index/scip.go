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

	// => Walk all structs and generate lookup table from position -> descriptors
	//    TODO: I think it should be possible to have Fields just be the SymbolString
	posToFields := map[token.Pos][]*scip.Descriptor{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			visitor := StructVisitor{
				fields: posToFields,
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
				fields:    posToFields,
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
					case token.VAR:
					case token.CONST:
						fmt.Println(":: Variable", decl.Tok)
					case token.TYPE:
						for _, spec := range decl.Specs {
							// token.TYPE ensures only ast.TypeSpec, as far as I can tell
							typespec := spec.(*ast.TypeSpec)

							structDescriptors := []*scip.Descriptor{
								pkgDescriptor,
								{
									Name:   typespec.Name.Name,
									Suffix: scip.Descriptor_Type,
								},
							}
							symbol := scipSymbolFromDescriptors(pkg.Module, structDescriptors)

							position := pkg.Fset.Position(typespec.Name.NamePos)
							doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
								Range:       scipRangeFromName(position, typespec.Name.Name, false),
								Symbol:      symbol,
								SymbolRoles: SymbolDefinition,
							})

							switch typ := typespec.Type.(type) {
							case *ast.StructType:
								for _, field := range typ.Fields.List {
									for _, name := range field.Names {
										if descriptors, ok := posToFields[name.NamePos]; ok {
											namePosition := pkg.Fset.Position(name.NamePos)
											doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
												Range:       scipRangeFromName(namePosition, name.Name, false),
												Symbol:      scipSymbolFromDescriptors(pkg.Module, descriptors),
												SymbolRoles: SymbolDefinition,
											})
										}

										// Otherwise?
										// append(structDescriptors, &scip.Descriptor{ Name:   name.Name, Suffix: scip.Descriptor_Term, }
									}

									ast.Walk(visitor, field.Type)
								}
							case *ast.Ident:
								// explicit pass
							case *ast.FuncType:
								fmt.Println("TODO: FuncType", typ)
							case *ast.InterfaceType:
								fmt.Println("TODO: InterfaceType", typ)
							case *ast.SelectorExpr:
								fmt.Println("TODO: SelectorExpr", typ)
							default:
								panic(fmt.Sprintf("unhandled typespec.Type: %T %s", typ, typ))
							}

							if typespec.TypeParams != nil {
								for _, param := range typespec.TypeParams.List {
									for _, name := range param.Names {
										namePosition := pkg.Fset.Position(name.NamePos)
										doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
											Range: scipRangeFromName(namePosition, name.Name, false),
											Symbol: scipSymbolFromDescriptors(pkg.Module, append(structDescriptors, &scip.Descriptor{
												Name:   name.Name,
												Suffix: scip.Descriptor_Term,
											})),
											SymbolRoles: SymbolDefinition,
										})
									}
								}

								panic("TypeParams")
							}
						}
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
	// TODO: Remove once we do this at the start
	ensureVersionForPackage(importedPackage)

	pkgPath := importedPackage.PkgPath
	scipRange := scipRangeFromName(position, pkgPath, true)
	symbol := scip.Symbol{
		Scheme: "scip-go",
		Package: &scip.Package{
			Manager: "gomod",
			Name:    importedPackage.Name,
			Version: importedPackage.Module.Version,
		},
		Descriptors: []*scip.Descriptor{{Name: pkgPath, Suffix: scip.Descriptor_Package}},
	}

	doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
		Range:       scipRange,
		Symbol:      scip.VerboseSymbolFormatter.FormatSymbol(&symbol),
		SymbolRoles: int32(scip.SymbolRole_ReadAccess),
	})
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
	if pkg == "main" || pkg == "" {
		// Special case...
		x := dependencies["smol_go"]
		return &x
	}

	for _, moduleName := range packagePrefixes(pkg) {
		if module, ok := dependencies[moduleName]; ok {
			return &module
		}
	}

	fmt.Printf("Unhandled module: %T %+v || %s\n", obj, obj, makeMonikerPackage(obj))
	panic("OH NO")
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
	// Document to append occurrences to
	doc *scip.Document

	// Current file information
	pkg  *packages.Package
	file *ast.File

	// soething
	pkgLookup map[string]*packages.Module

	// locals holds the references for a DEFINITION identifier
	locals map[token.Pos]string

	fields map[token.Pos][]*scip.Descriptor
}

func (f *FileVisitor) createNewLocalSymbol(pos token.Pos) string {
	if _, ok := f.locals[pos]; ok {
		panic("Cannot create a new local symbol for an ident that has already been created")
	}

	f.locals[pos] = fmt.Sprintf("local %d", len(f.locals))
	return f.locals[pos]
}

func (f FileVisitor) Visit(n ast.Node) (w ast.Visitor) {
	if n == nil {
		return nil
	}

	switch node := n.(type) {
	// explicit fail
	case *ast.File:
		panic("Should not find a file. Only call from within a file")

	// explicit pass
	case *ast.FuncDecl:

	// Identifiers
	case *ast.Ident:
		info := f.pkg.TypesInfo

		def := info.Defs[node]
		ref := info.Uses[node]

		pos := node.NamePos
		position := f.pkg.Fset.Position(pos)

		// Don't think anything can be a def and a ref
		// if def != nil && ref != nil {
		// 	panic("Didn't think this was possible")
		// }

		// Append definition
		if def != nil {
			// TODO: Ensure that nothing in this type could possibly be exported.
			//       That's literally the goal of the way that we have this set up.

			f.doc.Occurrences = append(f.doc.Occurrences, &scip.Occurrence{
				Range: scipRange(position, def),
				// Symbol:      scipSymbol2(mod, def),
				Symbol:      f.createNewLocalSymbol(def.Pos()),
				SymbolRoles: int32(scip.SymbolRole_Definition),
			})
		}

		if ref != nil {
			var symbol string
			if localSymbol, ok := f.locals[ref.Pos()]; ok {
				symbol = localSymbol
			} else {
				mod, ok := f.pkgLookup[pkgPath(ref)]
				if !ok {
					if ref.Pkg() == nil {
						panic(fmt.Sprintf("Failed to find the thing for ref: %s | %+v\n", pkgPath(ref), ref))
					}

					mod = f.pkgLookup[ref.Pkg().Name()]
				}

				if mod == nil {
					panic(fmt.Sprintf("Very weird, can't figure out this reference: %s", ref))
				}

				switch ref := ref.(type) {
				case *types.Var:
					if ref.IsField() {
						fieldDescriptors := f.fields[ref.Pos()]

						// TODO: Would probably be better to save this.
						symbol = scipSymbolFromDescriptors(mod, fieldDescriptors)
					}

				}

				if symbol == "" {
					symbol = scipSymbolFromObject(mod, ref)
				}
			}

			f.doc.Occurrences = append(f.doc.Occurrences, &scip.Occurrence{
				Range:       scipRange(position, ref),
				Symbol:      symbol,
				SymbolRoles: int32(scip.SymbolRole_ReadAccess),
			})
		}
	default:
		// fmt.Printf("unhandled: %T %v\n", n, n)
	}

	return f
}

func scipSymbolFromDescriptors(dep *packages.Module, descriptors []*scip.Descriptor) string {
	return scip.VerboseSymbolFormatter.FormatSymbol(&scip.Symbol{
		Scheme: "scip-go",
		Package: &scip.Package{
			Manager: "gomod",
			// TODO: We might not have a dep, so we should handle that
			Name:    dep.Path,
			Version: dep.Version,
		},
		Descriptors: descriptors,
	})
}

func scipSymbolFromObject(dep *packages.Module, obj ObjectLike) string {
	if dep == nil {
		panic("Somehow dep was nil...")
	}

	desc := []*scip.Descriptor{
		{Name: makeMonikerPackage(obj), Suffix: scip.Descriptor_Package},
	}
	return scipSymbolFromDescriptors(dep, append(desc, scipDescriptors(obj)...))
}

func scipDescriptors(obj ObjectLike) []*scip.Descriptor {
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
	case *types.TypeName:
		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Type},
		}
	case *types.PkgName:
		return []*scip.Descriptor{
			{Name: obj.Name(), Suffix: scip.Descriptor_Namespace},
		}
	default:
		fmt.Printf("TYPE OF OBJ: %T\n", obj)
	}

	return []*scip.Descriptor{}
}

type StructVisitor struct {
	fields   map[token.Pos][]*scip.Descriptor
	curScope []*scip.Descriptor
}

func (s StructVisitor) Visit(n ast.Node) (w ast.Visitor) {
	switch node := n.(type) {
	// TODO: Could probably skip a lot more of these?
	case *ast.FuncDecl:
	case *ast.FuncLit:
		return nil

	case *ast.TypeSpec:
		s.curScope = append(s.curScope, &scip.Descriptor{
			Name:   node.Name.Name,
			Suffix: scip.Descriptor_Type,
		})

		defer func() {
			s.curScope = s.curScope[:len(s.curScope)-1]
		}()

		ast.Walk(s, node.Type)
		return nil
	case *ast.Field:
		for _, name := range node.Names {
			newFields := append([]*scip.Descriptor{}, s.curScope...)
			newFields = append(newFields, &scip.Descriptor{
				Name:   name.Name,
				Suffix: scip.Descriptor_Term,
			})

			s.fields[name.Pos()] = newFields
		}

		// ast.Walk(s, node.Type)
		return nil
	}

	return s
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
