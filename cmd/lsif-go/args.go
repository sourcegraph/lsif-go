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
	filesToIndex   *[]string
)

func init() {
	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')
	app.HelpFlag.Hidden()

	app.Flag("out", "The output file.").Short('o').Default("dump.lsif").StringVar(&outFile)
	app.Flag("moduleVersion", "Specifies the version of the module defined by this project.").PlaceHolder("version").StringVar(&moduleVersion)
	app.Flag("repositoryRoot", "Specifies the path of the current repository (inferred automatically via git).").PlaceHolder("root").StringVar(&repositoryRoot)
	app.Flag("moduleRoot", "Specifies the module root directory relative to the repository").Default(".").StringVar(&moduleRoot)
	app.Flag("noProgress", "Do not output verbose progress.").Default("false").BoolVar(&noProgress)
	app.Flag("noOutput", "Do not output progress.").Default("false").BoolVar(&noOutput)
	app.Flag("verbose", "Display timings and stats.").Default("false").BoolVar(&verboseOutput)
	filesToIndex = app.Flag("file", "Repeatable flag for specific files to index. The resulting LSIF dump will contain only information necessary to resolve definition, reference, and hover requests for within these files.").Strings()
}

func parseArgs(args []string) (err error) {
	if _, err := app.Parse(args); err != nil {
		return err
	}

	repositoryRoot, err = filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("get abspath of repository root: %v", err)
	}

	if repositoryRoot == "" {
		toplevel, err := git.TopLevel(repositoryRoot)
		if err != nil {
			return fmt.Errorf("get git root: %v", err)
		}

		repositoryRoot = strings.TrimSpace(string(toplevel))
	}

	projectRoot, err = filepath.Abs(moduleRoot)
	if err != nil {
		return fmt.Errorf("get abspath of project root: %v", err)
	}

	// Ensure the module root is inside the repository
	if !strings.HasPrefix(projectRoot, repositoryRoot) {
		return errors.New("module root is outside the repository")
	}

	if len(*filesToIndex) == 0 {
		filesToIndex = nil
	} else {
		for idx := range *filesToIndex {
			(*filesToIndex)[idx], err = filepath.Abs((*filesToIndex)[idx])
			if err != nil {
				return fmt.Errorf("get abspath of manually specified file to index: %v", err)
			}
			if !strings.HasPrefix((*filesToIndex)[idx], repositoryRoot) {
				return errors.New("manually specified file to index is outside the repository")
			}
		}
	}

	return nil
}
