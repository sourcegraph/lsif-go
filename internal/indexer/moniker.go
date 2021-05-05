package indexer

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// emitExportMoniker emits an export moniker for the given object linked to the given source
// identifier (either a range or a result set identifier). This will also emit links between
// the moniker vertex and the package information vertex representing the current module.
func (i *Indexer) emitExportMoniker(sourceID uint64, p *packages.Package, obj types.Object) {
	if i.moduleName == "" {
		// Unknown dependencies, skip export monikers
		return
	}

	// If this is a package reference, remove the trailing `:`
	monikerIdentifier := fmt.Sprintf("%s:%s", monikerPackage(obj), monikerIdentifier(i.packageDataCache, p, obj))
	monikerIdentifier = strings.Trim(monikerIdentifier, ":")

	// Lazily emit package information vertex
	packageInformationID := i.ensurePackageInformation(i.moduleName, i.moduleVersion)

	// Emit moniker
	monikerID := i.emitter.EmitMoniker("export", "gomod", monikerIdentifier)

	// Attach package information to moniker, and moniker to source element
	_ = i.emitter.EmitPackageInformationEdge(monikerID, packageInformationID)
	_ = i.emitter.EmitMonikerEdge(sourceID, monikerID)
}

// emitImportMoniker emits an import moniker for the given object linked to the given source
// identifier (either a range or a result set identifier). This will also emit links between
// the moniker vertex and the package information vertex representing the dependency containing
// the identifier.
func (i *Indexer) emitImportMoniker(sourceID uint64, p *packages.Package, obj types.Object) {
	pkg := monikerPackage(obj)

	for _, moduleName := range packagePrefixes(pkg) {
		version, ok := i.dependencies[moduleName]
		if !ok {
			continue
		}

		// If this is a package reference, remove the trailing `:`
		monikerIdentifier := fmt.Sprintf("%s:%s", pkg, monikerIdentifier(i.packageDataCache, p, obj))
		monikerIdentifier = strings.Trim(monikerIdentifier, ":")

		// Lazily emit package information and moniker vertices
		packageInformationID := i.ensurePackageInformation(moduleName, version)
		monikerID := i.ensureImportMoniker(monikerIdentifier, packageInformationID)

		// Attach moniker to source element
		_ = i.emitter.EmitMonikerEdge(sourceID, monikerID)

		// Stop after first match
		break
	}
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

// ensurePackageInformation returns the identifier of a package information vertex with the
// give name and version. A vertex will be emitted only if one with the same name has not
// yet been emitted.
func (i *Indexer) ensurePackageInformation(name, version string) uint64 {
	i.packageInformationIDsMutex.RLock()
	packageInformationID, ok := i.packageInformationIDs[name]
	i.packageInformationIDsMutex.RUnlock()
	if ok {
		return packageInformationID
	}

	i.packageInformationIDsMutex.Lock()
	defer i.packageInformationIDsMutex.Unlock()

	if packageInformationID, ok := i.packageInformationIDs[name]; ok {
		return packageInformationID
	}

	packageInformationID = i.emitter.EmitPackageInformation(name, "gomod", version)
	i.packageInformationIDs[name] = packageInformationID
	return packageInformationID
}

// ensureImportMoniker returns the identifier of a moniker vertex with the give identifier
// attached to teh given package information identifier. A vertex will be emitted only if
// one with the same key has not yet been emitted.
func (i *Indexer) ensureImportMoniker(identifier string, packageInformationID uint64) uint64 {
	key := fmt.Sprintf("%s:%d", identifier, packageInformationID)

	i.importMonikerIDsMutex.RLock()
	monikerID, ok := i.importMonikerIDs[key]
	i.importMonikerIDsMutex.RUnlock()
	if ok {
		return monikerID
	}

	i.importMonikerIDsMutex.Lock()
	defer i.importMonikerIDsMutex.Unlock()

	if monikerID, ok := i.importMonikerIDs[key]; ok {
		return monikerID
	}

	monikerID = i.emitter.EmitMoniker("import", "gomod", identifier)
	_ = i.emitter.EmitPackageInformationEdge(monikerID, packageInformationID)
	i.importMonikerIDs[key] = monikerID
	return monikerID
}

// monikerPackage returns the package prefix used to construct a unique moniker for the given object.
// A full moniker has the form `{package prefix}:{identifier suffix}`.
func monikerPackage(obj types.Object) string {
	if v, ok := obj.(*types.PkgName); ok {
		return strings.Trim(v.Name(), `"`)
	}

	return obj.Pkg().Path()
}

// monikerIdentifier returns the identifier suffix used to construct a unique moniker for the given object.
// A full moniker has the form `{package prefix}:{identifier suffix}`. The identifier is meant to act as a
// qualified type path to the given object (e.g. `StructName.FieldName` or `StructName.MethodName`).
func monikerIdentifier(packageDataCache *PackageDataCache, p *packages.Package, obj types.Object) string {
	if _, ok := obj.(*types.PkgName); ok {
		// Packages are identified uniquely by their package prefix
		return ""
	}

	if v, ok := obj.(*types.Var); ok && v.IsField() {
		// Qualifiers for fields were populated as pre-load step so we do not need to traverse
		// the AST path back up to the root to find the enclosing type specs and fields with an
		// anonymous struct type.
		return strings.Join(packageDataCache.MonikerPath(p, obj.Pos()), ".")
	}

	if signature, ok := obj.Type().(*types.Signature); ok {
		if recv := signature.Recv(); recv != nil {
			return strings.Join([]string{
				// Qualify function with receiver stripped of a pointer indicator `*` and its package path
				strings.TrimPrefix(strings.TrimPrefix(recv.Type().String(), "*"), obj.Pkg().Path()+"."),
				obj.Name(),
			}, ".")
		}
	}

	return obj.Name()
}
