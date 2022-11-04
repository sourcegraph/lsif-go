package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os/exec"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

var parsedPackages = map[string]*PackageInfo{}
var parsedFiles = map[string]*ast.File{}
var fset = token.NewFileSet()

func getListPkg(pkgName string, dir string, isCurrentProject bool) ([]*PackageInfo, error) {
	if pkgName == "std" {
		panic("It is not possible to depend on 'std'. Dependencies should be via import path")
	}

	if _, ok := parsedPackages[pkgName]; ok {
		return []*PackageInfo{parsedPackages[pkgName]}, nil
	}

	arguments := []string{"list", "-e"}
	if isCurrentProject {
		// TODO: See if we can do this better, cause it seems like maybe it's wrong.
		// There is a possibility though that we might be able to do something with the TestFiles or something
		// instead of doing this. I don't think we actually want these "test" files.
		arguments = append(arguments, "-test=false")
	} else {
		arguments = append(arguments, "-test=false")
	}
	arguments = append(arguments, "-deps=false", "-json", pkgName)

	cmd := exec.Command("go", arguments...)
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()

	rawPkgs := []*GoListOutput{}

	decoder := json.NewDecoder(bytes.NewReader(out))
	for decoder.More() {
		pkg := GoListOutput{}
		err := decoder.Decode(&pkg)
		if err != nil {
			return nil, err
		}

		fmt.Println("MODULE:", pkg.Name, pkg.Module)
		rawPkgs = append(rawPkgs, &pkg)
	}

	currentPkgs := []*PackageInfo{}
	for _, rawPkg := range rawPkgs {
		pkg, err := parseRawPackage(fset, rawPkg, isCurrentProject)
		if err != nil {
			return nil, err
		}

		currentPkgs = append(currentPkgs, &pkg)
	}

	if pkgName != "./..." {
		if len(currentPkgs) > 1 {
			panic(fmt.Sprintf("It is not possible to have more than one package for specific import paths: %s", pkgName))
		}
	}
	return currentPkgs, nil
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

func parseRawPackage(fset *token.FileSet, rawPkg *GoListOutput, isCurrentProject bool) (PackageInfo, error) {
	var err error

	if parsedPackages[rawPkg.ImportPath] != nil {
		return *parsedPackages[rawPkg.ImportPath], nil
	}

	pkg := NewPackageInfo(rawPkg, &packages.Package{
		PkgPath: rawPkg.ImportPath,

		// packages.Package fieldsd
		ID:     rawPkg.ImportPath, // TODO: Is this good or not?
		Syntax: []*ast.File{},
		Fset:   fset,

		// TODO: Check if this is right
		Types: types.NewPackage(rawPkg.ImportPath, rawPkg.Name),
	})

	if isCurrentProject {
		for _, imp := range pkg.ImportedPaths {
			if parsedPackages[imp] != nil {
				continue
			}

			impPackage, err := getListPkg(imp, rawPkg.Dir, false)
			if err != nil {
				return PackageInfo{}, err
			}

			if len(impPackage) != 1 {
				panic(fmt.Sprintf("Yo, we cannot have packages like this: %s", imp))
			}

			pkg.Imports[imp] = impPackage[0]
		}
	}

	// Collect our files
	filesToParse := []string{}
	filesToParse = append(filesToParse, pkg.GoFiles...)
	if isCurrentProject {
		filesToParse = append(filesToParse, pkg.TestGoFiles...)
	}

	for _, gofile := range pkg.GoFiles {
		gofilePath := gofile
		if !filepath.IsAbs(gofilePath) {
			gofilePath = filepath.Join(pkg.Dir, gofile)
		}

		f := parsedFiles[gofilePath]
		if f == nil {
			f, err = parser.ParseFile(fset, gofilePath, nil, parser.AllErrors|parser.ParseComments)
			if err != nil {
				return PackageInfo{}, err
			}

			parsedFiles[gofilePath] = f
		} else {
			panic("==== Already Parsed ====")
		}

		pkg.Syntax = append(pkg.Syntax, f)
	}

	pkg.TypesInfo = &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	importer := importerFunc(func(path string) (*types.Package, error) {
		if path == "unsafe" {
			return types.Unsafe, nil
		}

		// TODO: Make sure these are _only_ the ones that I want to use.
		if importedPkg, ok := pkg.Imports[path]; ok {
			return importedPkg.Types, nil
		}

		return nil, fmt.Errorf("I haven't seen this import: %s", path)
		// The imports map is keyed by import path.
		// ipkg := pkg.Imports[path]
		// if ipkg == nil {
		// 	if err := lpkg.importErrors[path]; err != nil {
		// 		return nil, err
		// 	}
		// 	// There was skew between the metadata and the
		// 	// import declarations, likely due to an edit
		// 	// race, or because the ParseFile feature was
		// 	// used to supply alternative file contents.
		// 	return nil, fmt.Errorf("no metadata for %s", path)
		// }
		//
		// if ipkg.Types != nil && ipkg.Types.Complete() {
		// 	return ipkg.Types, nil
		// }
		// log.Fatalf("internal error: package %q without types was imported from %q", path, lpkg)
		// panic("unreachable")
	})

	// type-check
	tc := &types.Config{
		Importer: importer,

		// Type-check bodies of functions only in non-initial packages.
		// Example: for import graph A->B->C and initial packages {A,C},
		// we can ignore function bodies in B.
		IgnoreFuncBodies: !isCurrentProject,

		// FakeImportC: true,

		Error: func(err error) {},
		// Sizes: ld.sizes,

		DisableUnusedImportCheck: true,
	}

	// discard the error for this... we don't care if it fails
	checker := types.NewChecker(tc, pkg.Fset, pkg.Types, pkg.TypesInfo)
	checker.Files(pkg.Syntax)

	parsedPackages[pkg.ImportPath] = &pkg

	return pkg, nil
}
