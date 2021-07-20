package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/output"
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

	outputOptions := output.Options{
		Verbosity:      getVerbosity(),
		ShowAnimations: !noAnimation,
	}

	moduleName, err := gomod.ModuleName(moduleRoot, repositoryRemote)
	if err != nil {
		return err
	}

	dependencies, err := gomod.ListDependencies(moduleRoot, moduleName, moduleVersion)
	if err != nil {
		return err
	}

	return writeIndex(
		repositoryRoot,
		repositoryRemote,
		projectRoot,
		moduleName,
		moduleVersion,
		dependencies,
		outFile,
		outputOptions,
	)
}
