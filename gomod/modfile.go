package gomod

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var sumPattern = regexp.MustCompile("^([^ ]+) v([^/]+)( |/)")

// readModFile returns the module name extracted from the go.sum
// file at the given project root.
func readModFile(projectRoot string) (string, error) {
	reader, err := os.Open(filepath.Join(projectRoot, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("open go.mod file: %v", err)
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(line[7:]), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod file: %v", err)
	}

	return "", fmt.Errorf("failed to extract module")
}

// readSumFile returns a map of dependencies to their versions
// extracted from the content of the go.sum file at the given project
// root.
func readSumFile(projectRoot string) (map[string]string, error) {
	reader, err := os.Open(filepath.Join(projectRoot, "go.sum"))
	if err != nil {
		return nil, fmt.Errorf("open go.sum file: %v", err)
	}
	defer reader.Close()

	dependencies := map[string]string{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if matches := sumPattern.FindStringSubmatch(scanner.Text()); len(matches) > 0 {
			dependencies[matches[1]] = matches[2]
		}
	}

	return dependencies, nil
}
