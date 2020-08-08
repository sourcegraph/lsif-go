package indexer

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// emitExportMoniker emits an export moniker for the given object linked to the given source
// identifier (either a range or a result set identifier). This will also emit links between
// the moniker vertex and the package information vertex representing the current module.
func (i *Indexer) emitExportMoniker(sourceID uint64, o ObjectInfo) {
	if i.moduleName == "" {
		// Unknown dependencies, skip export monikers
		return
	}

	i.addMonikers(
		"export",
		strings.Trim(fmt.Sprintf("%s:%s", monikerPackage(o), monikerIdentifier(o)), ":"),
		sourceID,
		i.ensurePackageInformation(i.moduleName, i.moduleVersion),
	)
}

// emitImportMoniker emits an import moniker for the given object linked to the given source
// identifier (either a range or a result set identifier). This will also emit links between
// the moniker vertex and the package information vertex representing the dependency containing
// the identifier.
func (i *Indexer) emitImportMoniker(sourceID uint64, o ObjectInfo) {
	pkg := monikerPackage(o)

	for _, moduleName := range packagePrefixes(pkg) {
		if moduleVersion, ok := i.dependencies[moduleName]; ok {
			i.addMonikers(
				"import",
				strings.Trim(fmt.Sprintf("%s:%s", pkg, monikerIdentifier(o)), ":"),
				sourceID,
				i.ensurePackageInformation(moduleName, moduleVersion),
			)

			break
		}
	}
}

// packagePrefixes return sall prefix of the go package path. For example, the package
// `foo/bar/baz` will return the slice containing `foo/bar/baz`, `foo/bar`, and `foo`.
func packagePrefixes(packageName string) []string {
	parts := strings.Split(packageName, "/")
	prefixes := make([]string, len(parts))

	for i := 1; i <= len(parts); i++ {
		prefixes[len(parts)-i] = strings.Join(parts[:i], "/")
	}

	return prefixes
}

// ensurePackageInformation returns the identifier of a package information vertex with the
// give name and version. A vertex will be emitted only if one with the same name not yet
// been emitted.
func (i *Indexer) ensurePackageInformation(name, version string) uint64 {
	if packageInformationID, ok := i.packageInformationIDs[name]; ok {
		return packageInformationID
	}

	packageInformationID := i.emitter.EmitPackageInformation(name, "gomod", version)
	i.packageInformationIDs[name] = packageInformationID
	return packageInformationID
}

// addMonikers emits a moniker vertex with the given identifier, an edge from the moniker
// to the given package information vertex identifier, and an edge from the given source
// identifier to the moniker vertex identifier.
func (i *Indexer) addMonikers(kind, identifier string, sourceID, packageID uint64) {
	monikerID := i.emitter.EmitMoniker(kind, "gomod", identifier)
	_ = i.emitter.EmitPackageInformationEdge(monikerID, packageID)
	_ = i.emitter.EmitMonikerEdge(sourceID, monikerID)
}

// monikerPackage returns the package prefix used to construct a unique moniker for the given object.
// A full moniker has the form `{package prefix}:{identifier suffix}`.
func monikerPackage(o ObjectInfo) string {
	if v, ok := o.Object.(*types.PkgName); ok {
		return strings.Trim(v.Name(), `"`)
	}

	return o.Object.Pkg().Path()
}

// monikerIdentifier returns the identifier suffix used to construct a unique moniker for the given object.
// A full moniker has the form `{package prefix}:{identifier suffix}`. The identifier is meant to act as a
// qualified type path to the given object (e.g. `StructName.FieldName` or `StructName.MethodName`).
func monikerIdentifier(o ObjectInfo) string {
	if _, ok := o.Object.(*types.PkgName); ok {
		// Packages are identified uniquely by their package prefix
		return ""
	}

	return strings.Join(append(monikerIdentifierQualifiers(o), o.Ident.String()), ".")
}

// monikerIdentifierQualifiers returns a slice of container names used to construct the moniker identifier
// uniquely defining the given object. This will include the names of structs, interfaces, and receivers
// enclosing the target field or signature.
func monikerIdentifierQualifiers(o ObjectInfo) (qualifiers []string) {
	if v, ok := o.Object.(*types.Var); ok && v.IsField() {
		// TODO(efritz) - investigate performance of this function
		// Get path of nodes from the file root to the var identifier
		path, _ := astutil.PathEnclosingInterval(o.File, o.Ident.Pos(), o.Ident.Pos())

		// walk the nodes inside-out (from target to file root) and add the name of
		// each container to the list of qualifiers.
		for i := len(path) - 1; i >= 0; i-- {
			switch q := path[i].(type) {
			case *ast.Field:
				if q.Pos() != v.Pos() {
					// Add names of distinct fields whose type is an anonymous struct type
					// containing the target field (e.g. `X struct { target string }`).
					qualifiers = append(qualifiers, q.Names[0].String())
				}

			case *ast.TypeSpec:
				// Add the top-level type spec (e.g. `type X struct` and `type Y interface`)
				qualifiers = append(qualifiers, q.Name.String())
			}
		}

	}

	if signature, ok := o.Object.Type().(*types.Signature); ok {
		if recv := signature.Recv(); recv != nil {
			// Qualify function with receiver stripped of a pointer indicator `*` and its package path
			qualifiers = append(qualifiers, strings.TrimPrefix(strings.TrimPrefix(recv.Type().String(), "*"), o.Object.Pkg().Path()+"."))
		}
	}

	return qualifiers
}
