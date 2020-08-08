package gomod

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/sourcegraph/lsif-go/internal/command"
)

// ListModules returns the name of the module and a map from dependency names to their
// versions as declared by the go.mod file in the current directory.
func ListModules(dir string) (module string, dependencies map[string]string, err error) {
	if !isModule(dir) {
		log.Println("WARNING: No go.mod file found in current directory.")
		return "", nil, nil
	}

	output, err := command.Run(dir, "go", "list", "-mod=readonly", "-m", "all")
	if err != nil {
		return "", nil, fmt.Errorf("failed to list modules: %v\n%s", err, output)
	}

	module, dependencies = parseGoListOutput(output)
	return module, dependencies, nil
}

// parseGoListOutput parses the output from the `go list -m all` command. The first line
// is the versionless module name (as supplied by go.mod). The remaining lines consist of
// a dependency name and its version (separated by a space).
func parseGoListOutput(output string) (module string, dependencies map[string]string) {
	lines := strings.Split(output, "\n")

	dependencies = map[string]string{}
	for _, line := range lines {
		if parts := strings.Split(line, " "); len(parts) == 2 {
			dependencies[parts[0]] = cleanVersion(parts[1])
		}
	}

	return lines[0], dependencies

}

// versionPattern matches the form vX.Y.Z.-yyyymmddhhmmss-abcdefabcdef
var versionPattern = regexp.MustCompile(`^(.*)-(\d{14})-([a-f0-9]{12})$`)

// cleanVersion removes the date segment from a module version.
func cleanVersion(version string) string {
	if matches := versionPattern.FindStringSubmatch(version); len(matches) > 0 {
		return fmt.Sprintf("%s-%s", matches[1], matches[3])
	}

	return version
}
