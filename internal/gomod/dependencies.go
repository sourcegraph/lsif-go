package gomod

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/sourcegraph/lsif-go/internal/command"
	"golang.org/x/tools/go/vcs"
)

type Module struct {
	Name    string
	Version string
}

// ListDependencies returns a map from dependency import paths to the imported module's name
// and version as declared by the go.mod file in the current directory. The given root module
// and version are used to resolve replace directives with local file paths. The root module
// is expected to be a resolved import path (a valid URL, including a scheme).
func ListDependencies(dir, rootModule, rootVersion string) (map[string]Module, error) {
	if !isModule(dir) {
		log.Println("WARNING: No go.mod file found in current directory.")
		return nil, nil
	}

	output, err := command.Run(dir, "go", "list", "-mod=readonly", "-m", "-json", "all")
	if err != nil {
		return nil, fmt.Errorf("failed to list modules: %v\n%s", err, output)
	}

	dependencies, err := parseGoListOutput(output, rootVersion)
	if err != nil {
		return nil, err
	}

	modules := make([]string, 0, len(dependencies))
	for _, module := range dependencies {
		modules = append(modules, module.Name)
	}

	resolvedImportPaths := resolveImportPaths(rootModule, modules)
	mapImportPaths(dependencies, resolvedImportPaths)
	return dependencies, nil
}

type jsonModule struct {
	Name    string      `json:"Path"`
	Version string      `json:"Version"`
	Replace *jsonModule `json:"Replace"`
}

// parseGoListOutput parse the JSON output of `go list -m`. This method returns a map from
// import paths to pairs of declared (unresolved) module names and version pairs that respect
// replacement directives specified in go.mod. Replace directives indicating a local file path
// will create a module with the given root version, which is expected to be the same version
// as the module being indexed.
func parseGoListOutput(output, rootVersion string) (map[string]Module, error) {
	dependencies := map[string]Module{}
	decoder := json.NewDecoder(strings.NewReader(output))

	for {
		var module jsonModule
		if err := decoder.Decode(&module); err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		// Stash original name before applying replacement
		importPath := module.Name

		// If there's a replace directive, use that module instead
		if module.Replace != nil {
			module = *module.Replace
		}

		// Local file paths and root modules
		if module.Version == "" {
			module.Version = rootVersion
		}

		dependencies[importPath] = Module{
			Name:    module.Name,
			Version: cleanVersion(module.Version),
		}
	}

	return dependencies, nil
}

// versionPattern matches a versioning ending in a 12-digit sha, e.g., vX.Y.Z.-yyyymmddhhmmss-abcdefabcdef
var versionPattern = regexp.MustCompile(`^.*-([a-f0-9]{12})$`)

// cleanVersion normalizes a module version string.
func cleanVersion(version string) string {
	version = strings.TrimSpace(strings.TrimSuffix(version, "// indirect"))
	version = strings.TrimSpace(strings.TrimSuffix(version, "+incompatible"))

	if matches := versionPattern.FindStringSubmatch(version); len(matches) > 0 {
		return matches[1]
	}

	return version
}

// resolveImportPaths returns a map of import paths to resolved code host and path
// suffix usable for moniker identifiers. The given root module is used to resolve
// replace directives with local file paths and is expected to be a resolved import
// path (a valid URL, including a scheme).
func resolveImportPaths(rootModule string, modules []string) map[string]string {
	ch := make(chan string, len(modules))
	for _, module := range modules {
		ch <- module
	}
	close(ch)

	var m sync.Mutex
	namesToResolve := map[string]string{}
	var wg sync.WaitGroup

	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for name := range ch {
				// Stash original name before applying replacement
				originalName := name

				// Try to resolve the import path if it looks like a local path
				name, err := resolveLocalPath(name, rootModule)
				if err != nil {
					log.Println(fmt.Sprintf("WARNING: Failed to resolve %s (%s).", name, err))
					continue
				}

				// Determine path suffix relative to the import path
				repoRoot, err := vcs.RepoRootForImportPath(name, false)
				if err != nil {
					log.Println(fmt.Sprintf("WARNING: Failed to resolve %s (%s).", name, err))
					continue
				}
				suffix := strings.TrimPrefix(name, repoRoot.Root)

				m.Lock()
				namesToResolve[originalName] = repoRoot.Repo + suffix
				m.Unlock()
			}
		}()
	}

	wg.Wait()
	return namesToResolve
}

// resolveLocalPath converts the given name to an import path if it looks like a local path based on
// the given root module. The root module, if non-empty, is expected to be a resolved import path
// (a valid URL, including a scheme). If the name does not look like a local path, it will be returned
// unchanged.
func resolveLocalPath(name, rootModule string) (string, error) {
	if rootModule == "" || !strings.HasPrefix(name, ".") {
		return name, nil
	}

	parsedRootModule, err := url.Parse(rootModule)
	if err != nil {
		return "", err
	}

	// Join path relative to the root to the parsed module
	parsedRootModule.Path = path.Join(parsedRootModule.Path, name)

	// Remove scheme so it's resolvable again as an import path
	return strings.TrimPrefix(parsedRootModule.String(), parsedRootModule.Scheme+"://"), nil
}

// mapImportPaths replace each module name with the value in the given resolved import paths
// map. If the module name is not present in the map, no change is made to the module value.
func mapImportPaths(dependencies map[string]Module, resolvedImportPaths map[string]string) {
	for importPath, module := range dependencies {
		if name, ok := resolvedImportPaths[module.Name]; ok {
			dependencies[importPath] = Module{
				Name:    name,
				Version: module.Version,
			}
		}
	}
}
