package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/sourcegraph/lsif-go/internal/git"
	"github.com/sourcegraph/lsif-go/protocol"
)

var app = kingpin.New(
	"lsif-go",
	"lsif-go is an LSIF indexer for Go.",
).Version(version + ", protocol version " + protocol.Version)

var (
	outFile        string
	projectRoot    string
	moduleRoot     string
	repositoryRoot string
	moduleVersion  string
	noOutput       bool
	verboseOutput  bool
	noProgress     bool
)

func init() {
	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')
	app.HelpFlag.Hidden()

	// Outfile options
	app.Flag("out", "The output file.").Short('o').Default("dump.lsif").StringVar(&outFile)

	// Module version options (inferred by git)
	app.Flag("moduleVersion", "Specifies the version of the module defined by this project.").Default(defaultModuleVersion.Value()).StringVar(&moduleVersion)

	// Path options (inferred by presence of go.mod; git)
	app.Flag("projectRoot", "Specifies the root directory to index.").Default(".").StringVar(&projectRoot)
	app.Flag("moduleRoot", "Specifies the directory containing the go.mod file.").Default(defaultModuleRoot.Value()).StringVar(&moduleRoot)
	app.Flag("repositoryRoot", "Specifies the path to the root of the current repository.").Default(defaultRepositoryRoot.Value()).StringVar(&repositoryRoot)

	// Verbosity options
	app.Flag("noOutput", "Do not output progress.").Default("false").BoolVar(&noOutput)
	app.Flag("verbose", "Display timings and stats.").Default("false").BoolVar(&verboseOutput)
	app.Flag("noProgress", "Do not output verbose progress.").Default("false").BoolVar(&noProgress)
}

func parseArgs(args []string) (err error) {
	if _, err := app.Parse(args); err != nil {
		return err
	}

	sanitizers := []func() error{sanitizeProjectRoot, sanitizeModuleRoot, sanitizeRepositoryRoot}
	validators := []func() error{validatePaths}

	for _, f := range append(sanitizers, validators...) {
		if err := f(); err != nil {
			return err
		}
	}

	return nil
}

//
// Sanitizers

func sanitizeProjectRoot() (err error) {
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("get abspath of project root: %v", err)
	}

	return nil
}

func sanitizeModuleRoot() (err error) {
	moduleRoot, err = filepath.Abs(moduleRoot)
	if err != nil {
		return fmt.Errorf("get abspath of module root: %v", err)
	}

	return nil
}

func sanitizeRepositoryRoot() (err error) {
	repositoryRoot, err = filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("get abspath of repository root: %v", err)
	}

	return nil
}

//
// Validators

func validatePaths() error {
	if !strings.HasPrefix(projectRoot, repositoryRoot) {
		return errors.New("project root is outside the repository")
	}

	if !strings.HasPrefix(moduleRoot, repositoryRoot) {
		return errors.New("module root is outside the repository")
	}

	return nil
}

//
// Defaults

var defaultProjectRoot = newCachedString(func() string {
	return rel(wd.Value())
})

var defaultModuleRoot = newCachedString(func() string {
	return searchForGoMod(wd.Value(), toplevel.Value())
})

var defaultRepositoryRoot = newCachedString(func() string {
	return rel(toplevel.Value())
})

var defaultModuleVersion = newCachedString(func() string {
	if version, err := git.InferModuleVersion(defaultModuleRoot.Value()); err == nil {
		return version
	}

	return ""
})
