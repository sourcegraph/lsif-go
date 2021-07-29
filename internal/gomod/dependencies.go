package gomod

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/sourcegraph/lsif-go/internal/command"
	"github.com/sourcegraph/lsif-go/internal/output"
	"golang.org/x/tools/go/vcs"
)

type GoModule struct {
	Name    string
	Version string
}

// ListDependencies returns a map from dependency import paths to the imported module's name
// and version as declared by the go.mod file in the current directory. The given root module
// and version are used to resolve replace directives with local file paths. The root module
// is expected to be a resolved import path (a valid URL, including a scheme).
func ListDependencies(dir, rootModule, rootVersion string, outputOptions output.Options) (dependencies map[string]GoModule, err error) {
	if !isModule(dir) {
		log.Println("WARNING: No go.mod file found in current directory.")
		return nil, nil
	}

	resolve := func() {
		output, err := command.Run(dir, "go", "list", "-mod=readonly", "-m", "-json", "all")
		if err != nil {
			err = fmt.Errorf("failed to list modules: %v\n%s", err, output)
			return
		}

		dependencies, err = parseGoListOutput(output, rootVersion)
		if err != nil {
			return
		}

		modules := make([]string, 0, len(dependencies))
		for _, module := range dependencies {
			modules = append(modules, module.Name)
		}

		resolvedImportPaths := resolveImportPaths(rootModule, modules)
		mapImportPaths(dependencies, resolvedImportPaths)
	}

	output.WithProgress("Listing dependencies", resolve, outputOptions)
	return dependencies, err
}

type jsonModule struct {
	Name    string      `json:"Path"`
	Version string      `json:"Version"`
	Replace *jsonModule `json:"Replace"`

	// The Golang version required for this module
	GoVersion string `json:"GoVersion"`
}

var golangRepository = "github.com/golang/go"
var validGolangVersion = regexp.MustCompile(`(\d+)\.(\d+)`)

// parseGoListOutput parse the JSON output of `go list -m`. This method returns a map from
// import paths to pairs of declared (unresolved) module names and version pairs that respect
// replacement directives specified in go.mod. Replace directives indicating a local file path
// will create a module with the given root version, which is expected to be the same version
// as the module being indexed.
func parseGoListOutput(output, rootVersion string) (map[string]GoModule, error) {
	dependencies := map[string]GoModule{}
	decoder := json.NewDecoder(strings.NewReader(output))

	var goVersion parsedGoVersion
	for {
		var module jsonModule
		if err := decoder.Decode(&module); err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		if validGolangVersion.MatchString(module.GoVersion) {
			moduleGoVersion, err := getGoVersion(module.GoVersion)
			if err != nil {
				continue
			}

			// Pick the latest go version.
			//    In general, the first version should be equal to the highest dependency,
			//    but this keeps us safe from picking an old go version.
			if goVersion.Version == "" {
				goVersion = moduleGoVersion
			} else if goVersion.Major < moduleGoVersion.Major && goVersion.Minor < moduleGoVersion.Minor {
				goVersion = moduleGoVersion
			}
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

		dependencies[importPath] = GoModule{
			Name:    module.Name,
			Version: cleanVersion(module.Version),
		}
	}

	if goVersion.Version == "" {
		return nil, errors.New("Must have a valid go version in go mod file")
	}
	setGolangDependency(dependencies, goVersion)

	return dependencies, nil
}

type parsedGoVersion struct {
	Version string
	Major   int
	Minor   int
}

func getGoVersion(version string) (parsedGoVersion, error) {
	splitVersion := strings.Split(version, ".")

	major, err := strconv.Atoi(splitVersion[0])
	if err != nil {
		return parsedGoVersion{}, err
	}

	minor, err := strconv.Atoi(splitVersion[1])
	if err != nil {
		return parsedGoVersion{}, err
	}

	return parsedGoVersion{
		Version: version,
		Major:   major,
		Minor:   minor,
	}, nil
}

func setGolangDependency(dependencies map[string]GoModule, goVersion parsedGoVersion) {
	dependencies[golangRepository] = GoModule{
		Name:    golangRepository,
		Version: "go" + goVersion.Version,
	}

}

func GetGolangDependency(dependencies map[string]GoModule) GoModule {
	return dependencies[golangRepository]
}

// IsStandardlibPackge checks whether a particular package is a standard
// library package.
func IsStandardlibPackge(pkg string) bool {
	if strings.Contains(pkg, ".") {
		return false
	}

	if strings.HasPrefix(pkg, "_") {
		return false
	}

	return true
}

// NormalizeMonikerPackage returns a normalized path to ensure that all
// standard library paths are handled the same. Primarily to make sure
// that both the golangRepository and "std/" paths are normalized.
func NormalizeMonikerPackage(path string) string {
	if !IsStandardlibPackge(path) {
		return path
	}

	var stdPrefix string
	if !strings.HasPrefix(path, "std/") {
		stdPrefix = "std/"
	}

	return fmt.Sprintf("%s/%s%s", golangRepository, stdPrefix, path)
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
					log.Println(fmt.Sprintf("WARNING: Failed to resolve local %s (%s).", name, err))
					continue
				}

				// Determine path suffix relative to the import path
				resolved, ok := resolveRepoRootForImportPath(name)
				if !ok {
					continue
				}

				m.Lock()
				namesToResolve[originalName] = resolved
				m.Unlock()
			}
		}()
	}

	wg.Wait()
	return namesToResolve
}

// resolveRepoRootForImportPath will get the resolved name after handling vsc RepoRoots and any
// necessary handling of the standard library
func resolveRepoRootForImportPath(name string) (string, bool) {
	// When indexining golang/go, there are some references to the package "std" itself.
	//    Generally, this not referenced directly (it is just assumed when you have "fmt" or similar
	//    in your imports), but inside of golang/go, it is directly referenced.
	//
	//    In that case, we just return it directly, there is no other resolving to do.
	if name == "std" {
		return name, true
	}

	repoRoot, err := vcs.RepoRootForImportPath(name, false)
	if err != nil {
		log.Println(fmt.Sprintf("WARNING: Failed to resolve repo %s (%s) %s.", name, err, repoRoot))
		return "", false
	}

	suffix := strings.TrimPrefix(name, repoRoot.Root)
	return repoRoot.Repo + suffix, true
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
func mapImportPaths(dependencies map[string]GoModule, resolvedImportPaths map[string]string) {
	for importPath, module := range dependencies {
		if name, ok := resolvedImportPaths[module.Name]; ok {
			dependencies[importPath] = GoModule{
				Name:    name,
				Version: module.Version,
			}
		}
	}
}
