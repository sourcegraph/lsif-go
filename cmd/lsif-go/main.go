// The program lsif-go is an LSIF indexer for Go.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sourcegraph/lsif-go/internal/git"
	"github.com/sourcegraph/lsif-go/internal/gomod"
)

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(os.Stdout)
}

func main() {
	if err := mainErr(); err != nil {
		fmt.Fprint(os.Stderr, fmt.Sprintf("\nerror: %v\n", err))
		os.Exit(1)
	}
}

func mainErr() error {
	if err := parseArgs(os.Args[1:]); err != nil {
		return err
	}

	// Ensure all the dependencies of the specified module are cached
	if err := gomod.Download(projectRoot); err != nil {
		return fmt.Errorf("fetching dependencies: %v", err)
	}

	moduleName, dependencies, err := gomod.ListModules(projectRoot)
	if err != nil {
		return err
	}

	if moduleVersion == "" {
		// Infer module version from git data if one is not explicitly supplied
		if moduleVersion, err = git.InferModuleVersion(projectRoot); err != nil {
			return err
		}
	}

	var fileSet map[string]struct{}
	if len(filesToIndex) > 0 {
		fileSet = make(map[string]struct{})
		for _, file := range filesToIndex {
			fileSet[file] = struct{}{}
		}
	}

	return writeIndex(
		repositoryRoot,
		projectRoot,
		moduleName,
		moduleVersion,
		fileSet,
		dependencies,
		outFile,
	)
}
