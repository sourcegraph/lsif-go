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
	moduleVersion  string
	repositoryRoot string
	moduleRoot     string
	projectRoot    string
	noProgress     bool
	noOutput       bool
	verboseOutput  bool
)

func init() {
	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')
	app.HelpFlag.Hidden()

	app.Flag("out", "The output file.").Short('o').Default("dump.lsif").StringVar(&outFile)
	app.Flag("moduleVersion", "Specifies the version of the module defined by this project.").PlaceHolder("version").StringVar(&moduleVersion)
	app.Flag("repositoryRoot", "Specifies the path of the current repository (inferred automatically via git).").PlaceHolder("root").StringVar(&repositoryRoot)
	app.Flag("moduleRoot", "Specifies the directory containing the go.mod file.").Default(".").StringVar(&moduleRoot)
	app.Flag("projectRoot", "Specifies the root directory to index.").Default(".").StringVar(&projectRoot)
	app.Flag("noProgress", "Do not output verbose progress.").Default("false").BoolVar(&noProgress)
	app.Flag("noOutput", "Do not output progress.").Default("false").BoolVar(&noOutput)
	app.Flag("verbose", "Display timings and stats.").Default("false").BoolVar(&verboseOutput)
}

func parseArgs(args []string) (err error) {
	if _, err := app.Parse(args); err != nil {
		return err
	}

	if repositoryRoot == "" {
		toplevel, err := git.TopLevel(repositoryRoot)
		if err != nil {
			return fmt.Errorf("get git root: %v", err)
		}

		repositoryRoot = strings.TrimSpace(string(toplevel))
	} else {
		repositoryRoot, err = filepath.Abs(repositoryRoot)
		if err != nil {
			return fmt.Errorf("get abspath of repository root: %v", err)
		}
	}

	moduleRoot, err = filepath.Abs(moduleRoot)
	if err != nil {
		return fmt.Errorf("get abspath of module root: %v", err)
	}

	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("get abspath of project root: %v", err)
	}

	// Ensure the module root is inside the repository
	if !strings.HasPrefix(moduleRoot, repositoryRoot) {
		return errors.New("module root is outside the repository")
	}

	// Ensure the project root is inside the repository
	if !strings.HasPrefix(projectRoot, repositoryRoot) {
		return errors.New("project root is outside the repository")
	}

	return nil
}
