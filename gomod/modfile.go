package gomod

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var (
	// TODO - also get version
	modPattern = regexp.MustCompile("^module (.*)$")

	// TODO - read docs to get actual pattern
	sumPattern = regexp.MustCompile("^([^ ]+) v([^/]+)/go.mod")
)

func readModFile(projectRoot string) (string, string, error) {
	reader, err := os.Open(filepath.Join(projectRoot, "go.mod"))
	if err != nil {
		return "", "", fmt.Errorf("open dump file: %v", err)
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if matches := modPattern.FindStringSubmatch(scanner.Text()); len(matches) > 0 {
			return matches[1], "0.1.0", nil
		}
	}

	return "", "", nil
}

func readSumFile(projectRoot string) (map[string]string, error) {
	reader, err := os.Open(filepath.Join(projectRoot, "go.sum"))
	if err != nil {
		return nil, fmt.Errorf("open dump file: %v", err)
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
