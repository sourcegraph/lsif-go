// The program lsif-go is an LSIF indexer for Go.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/index"
	"github.com/sourcegraph/lsif-go/internal/log"
	"github.com/sourcegraph/lsif-go/protocol"
)

const version = "0.4.1"
const versionString = version + ", protocol version " + protocol.Version

func main() {
	if err := realMain(); err != nil {
		fmt.Fprint(os.Stderr, fmt.Sprintf("error: %v\n", err))
		os.Exit(1)
	}
}

func realMain() error {
	var (
		debug          bool
		verbose        bool
		projectRoot    string
		repositoryRoot string
		moduleVersion  string
		noContents     bool
		outFile        string
	)

	app := kingpin.New("lsif-go", "lsif-go is an LSIF indexer for Go.").Version(versionString)
	app.Flag("debug", "Display debug information.").Default("false").BoolVar(&debug)
	app.Flag("verbose", "Display verbose information.").Short('v').Default("false").BoolVar(&verbose)
	app.Flag("projectRoot", "Specifies the project root. Defaults to the current working directory.").Default(".").StringVar(&projectRoot)
	app.Flag("repositoryRoot", "Specifies the repository root.").StringVar(&repositoryRoot)
	app.Flag("moduleVersion", "Specifies the version of the module defined by this project.").StringVar(&moduleVersion)
	app.Flag("noContents", "File contents will not be embedded into the dump.").Default("false").BoolVar(&noContents)
	app.Flag("out", "The output file the dump is saved to.").Default("dump.lsif").StringVar(&outFile)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if verbose {
		log.SetLevel(log.Info)
	}

	if debug {
		log.SetLevel(log.Debug)
	}

	// Print progress dots if we have no other output
	printProgressDots := !verbose && !debug

	if repositoryRoot == "" {
		toplevel, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return fmt.Errorf("get git root: %v", err)
		}
		repositoryRoot = string(toplevel)
	}

	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("get abspath of project root: %v", err)
	}

	repositoryRoot, err = filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("get abspath of repository root: %v", err)
	}

	moduleName, dependencies, err := gomod.ListModules(projectRoot)
	if err != nil {
		return err
	}

	if moduleVersion == "" {
		if moduleVersion, err = gomod.InferModuleVersion(projectRoot); err != nil {
			return err
		}
	}

	out, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("create dump file: %v", err)
	}

	defer out.Close()

	toolInfo := protocol.ToolInfo{
		Name:    "lsif-go",
		Version: version,
		Args:    os.Args[1:],
	}

	indexer := index.NewIndexer(
		projectRoot,
		repositoryRoot,
		moduleName,
		moduleVersion,
		dependencies,
		noContents,
		printProgressDots,
		toolInfo,
		out,
	)

	start := time.Now()
	s, err := indexer.Index()
	if printProgressDots {
		// End progress line before printing summary or error
		log.Println()
		log.Println()
	}

	if err != nil {
		return fmt.Errorf("index: %v", err)
	}

	log.Printf("%d package(s), %d file(s), %d def(s), %d element(s)", s.NumPkgs, s.NumFiles, s.NumDefs, s.NumElements)
	log.Println("Processed in", time.Since(start))
	return nil
}
