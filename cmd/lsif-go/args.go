package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/sourcegraph/lsif-go/internal/git"
	protocol "github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
)

var app = kingpin.New(
	"lsif-go",
	"lsif-go is an LSIF indexer for Go.",
).Version(version + ", protocol version " + protocol.Version)

var (
	outFile          string
	projectRoot      string
	moduleRoot       string
	repositoryRoot   string
	repositoryRemote string
	moduleVersion    string
	verbosity        int
	noOutput         bool
	noAnimation      bool
)

func init() {
	app.HelpFlag.Short('h')
	app.VersionFlag.Short('V')

	// Outfile options
	app.Flag("output", "The output file.").Short('o').Default("dump.lsif").StringVar(&outFile)

	// Path options (inferred by presence of go.mod; git)
	app.Flag("project-root", "Specifies the directory to index.").Default(".").StringVar(&projectRoot)
	app.Flag("module-root", "Specifies the directory containing the go.mod file.").Default(defaultModuleRoot.Value()).StringVar(&moduleRoot)
	app.Flag("repository-root", "Specifies the top-level directory of the git repository.").Default(defaultRepositoryRoot.Value()).StringVar(&repositoryRoot)

	// Repository remote and tag options (inferred by git)
	app.Flag("repository-remote", "Specifies the canonical name of the repository remote.").Default(defaultRepositoryRemote.Value()).StringVar(&repositoryRemote)
	app.Flag("module-version", "Specifies the version of the module defined by module-root.").Default(defaultModuleVersion.Value()).StringVar(&moduleVersion)

	// Verbosity options
	app.Flag("quiet", "Do not output to stdout or stderr.").Short('q').Default("false").BoolVar(&noOutput)
	app.Flag("verbose", "Output debug logs.").Short('v').CounterVar(&verbosity)
	app.Flag("no-animation", "Do not animate output.").Default("false").BoolVar(&noAnimation)
}

func parseArgs(args []string) (err error) {
	if _, err := app.Parse(args); err != nil {
		return fmt.Errorf("failed to parse args: %v", err)
	}

	sanitizers := []func() error{sanitizeProjectRoot, sanitizeModuleRoot, sanitizeRepositoryRoot}
	validators := []func() error{validatePaths}

	for _, f := range append(sanitizers, validators...) {
		if err := f(); err != nil {
			return fmt.Errorf("failed to parse args: %v", err)
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

var defaultModuleRoot = newCachedString(func() string {
	return searchForGoMod(wd.Value(), toplevel.Value())
})

var defaultRepositoryRoot = newCachedString(func() string {
	return rel(toplevel.Value())
})

var defaultRepositoryRemote = newCachedString(func() string {
	if repo, err := git.InferRepo(defaultModuleRoot.Value()); err == nil {
		return repo
	}

	return ""
})

var defaultModuleVersion = newCachedString(func() string {
	if version, err := git.InferModuleVersion(defaultModuleRoot.Value()); err == nil {
		return version
	}

	return ""
})
