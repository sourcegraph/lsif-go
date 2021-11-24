package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sourcegraph/lsif-go/internal/git"
	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/indexer"
	"github.com/sourcegraph/lsif-go/internal/output"
)

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(os.Stdout)
}

func main() {
	if err := mainErr(); err != nil {
		fmt.Fprint(os.Stderr, fmt.Sprintf("error: %v\n", err))
		os.Exit(1)
	}
}

func mainErr() (err error) {
	if err := parseArgs(os.Args[1:]); err != nil {
		return err
	}

	if !git.Check(moduleRoot) {
		return fmt.Errorf("module root is not a git repository")
	}

	defer func() {
		if err != nil {
			// Add a new line to all errors except for ones that
			// come from parsing invalid command line arguments
			// and basic environment sanity checks.
			//
			// We will print progress unconditionally after this
			// point and we want the error text to be clearly
			// visible.
			fmt.Fprintf(os.Stderr, "\n")
		}
	}()

	outputOptions := output.Options{
		Verbosity:      getVerbosity(),
		ShowAnimations: !noAnimation,
	}

	moduleName, err := gomod.ModuleName(moduleRoot, repositoryRemote, outputOptions)
	if err != nil {
		return fmt.Errorf("failed to infer module name: %v", err)
	}

	dependencies, err := gomod.ListDependencies(moduleRoot, moduleName, moduleVersion, outputOptions)
	if err != nil {
		return fmt.Errorf("failed to list dependencies: %v", err)
	}

	projectDependencies, err := gomod.ListProjectDependencies(moduleRoot)
	if err != nil {
		return fmt.Errorf("failed to list project dependencies: %v", err)
	}

	generationOptions := indexer.NewGenerationOptions()
	generationOptions.EnableApiDocs = enableApiDocs
	generationOptions.EnableImplementations = enableImplementations
	generationOptions.DepBatchSize = depBatchSize

	if err := writeIndex(
		repositoryRoot,
		repositoryRemote,
		projectRoot,
		moduleName,
		moduleVersion,
		dependencies,
		projectDependencies,
		outFile,
		outputOptions,
		generationOptions,
	); err != nil {
		return fmt.Errorf("failed to index: %v", err)
	}

	return nil
}
