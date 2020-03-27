package gomod

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// InferModuleVersion determines the go module version that is supplied by the
// code in the given project root.
func InferModuleVersion(projectRoot string) (string, error) {
	// Step 1: see if the current commit is tagged. If it is, we return
	// just the tag without a commit attached to it.
	tag, err := run(projectRoot, "git", "tag", "-l", "--points-at", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to tags for current commit: %v", err)
	}
	if tag != "" {
		return tag, nil
	}

	// Step 2: Find the most recent tag of the current commit. We need to
	// describe the tags, but this exits with status 128 when there are no
	// tags known to the current clone. Ensure that there are tags prior to
	// running this command.
	var mostRecentTag string
	tags, err := run(projectRoot, "git", "tag")
	if err != nil {
		return "", fmt.Errorf("failed to list tags: %v", err)
	}
	if tags != "" {
		if mostRecentTag, err = run(projectRoot, "git", "describe", "--tags", "--abbrev=0"); err != nil {
			return "", fmt.Errorf("failed to get most recent tag: %v", err)
		}
	} else {
		// canned tag
		mostRecentTag = "v0.0.0"
	}

	// Step 3: Determine the current commit and suffix it with the most
	// recent tag.
	commit, err := run(projectRoot, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current commit: %v", err)
	}
	return fmt.Sprintf("%s-%s", mostRecentTag, commit[:12]), nil
}

// ListModules returns the module name provided by the project root as well
// as a map from names to versions of each module the project depends on.
func ListModules(projectRoot string) (string, map[string]string, error) {
	_, err := os.Stat(filepath.Join(projectRoot, "go.mod"))
	if os.IsNotExist(err) {
		log.Println("WARNING: No go.mod file found in current directory.")
		return "", nil, nil
	}

	out, err := run(projectRoot, "go", "list", "-mod=readonly", "-m", "all")
	if err != nil {
		return "", nil, fmt.Errorf("failed to list modules: %v", err)
	}

	// go list -m all output:
	//   - first line is the versionless module name as supplied by go.mod
	//   - each remaining line has a versioned dependency `{name} {version}`
	lines := strings.Split(out, "\n")

	dependencies := map[string]string{}
	for _, line := range lines {
		parts := strings.Split(line, " ")
		if len(parts) == 2 {
			dependencies[parts[0]] = cleanVersion(parts[1])
		}
	}

	return lines[0], dependencies, nil
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

// run executes the command in the given directory and returns the output.
// Whitespace is trimmed from the output to get rid of trailing newlines.
func run(dir, command string, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}
