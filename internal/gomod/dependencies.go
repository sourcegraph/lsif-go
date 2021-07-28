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
	Name      string      `json:"Path"`
	Version   string      `json:"Version"`
	GoVersion string      `json:"GoVersion"`
	Replace   *jsonModule `json:"Replace"`
}

var stdlibName = "github.com/golang/go"
var validGoVersion = regexp.MustCompile(`(\d+)\.(\d+)`)

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

		if validGoVersion.MatchString(module.GoVersion) {
			currentVersion, err := getGoVersion(module.GoVersion)
			if err != nil {
				continue
			}

			// Pick the latest go version.
			//    This probably should be the first one, but this just ensures that we pick
			//    the latest go version and only a go version that we can understand
			if goVersion.Version == "" {
				goVersion = currentVersion
			} else if goVersion.Major < currentVersion.Major && goVersion.Minor < currentVersion.Minor {
				goVersion = currentVersion
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
	dependencies[stdlibName] = GoModule{
		Name:    stdlibName,
		Version: "go" + goVersion.Version,
	}

}

func GetGolangDependency(dependencies map[string]GoModule) GoModule {
	return dependencies[stdlibName]
}

func IsStandardlibPackge(pkg string) bool {
	// TODO: Any other considerations?
	// Could also hardcode the result

	if strings.Contains(pkg, ".") {
		return false
	}

	return true
}

func NormalizeMonikerPackage(path string) string {
	if !IsStandardlibPackge(path) {
		return path
	}

	var stdPrefix string
	if !strings.HasPrefix(path, "std/") {
		stdPrefix = "std/"
	}

	return fmt.Sprintf("%s/%s%s", stdlibName, stdPrefix, path)
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

				var finalName string
				if name == "std" {
					// fmt.Println("STD CHECK HERE", rootModule, modules)
					finalName = name
				} else {
					// Determine path suffix relative to the import path
					repoRoot, err := vcs.RepoRootForImportPath(name, false)
					if err != nil {
						log.Println(fmt.Sprintf("WARNING: Failed to resolve repo %s (%s).", name, err))
						continue
					}
					suffix := strings.TrimPrefix(name, repoRoot.Root)
					finalName = repoRoot.Repo + suffix
				}

				m.Lock()
				namesToResolve[originalName] = finalName
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
