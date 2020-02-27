// The program lsif-go is an LSIF indexer for Go.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/index"
	"github.com/sourcegraph/lsif-go/protocol"
)

const version = "0.4.1"
const versionString = version + ", protocol version " + protocol.Version

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(os.Stdout)
}

func main() {
	if err := realMain(); err != nil {
		fmt.Fprint(os.Stderr, fmt.Sprintf("error: %v\n", err))
		os.Exit(1)
	}
}

func realMain() error {
	var (
		outFile        string
		moduleVersion  string
		repositoryRoot string
		addContents    bool
	)

	app := kingpin.New("lsif-go", "lsif-go is an LSIF indexer for Go.").Version(versionString)
	app.Flag("out", "The output file").Default("dump.lsif").StringVar(&outFile)
	app.Flag("moduleVersion", "Specifies the version of the module defined by this project.").PlaceHolder("version").StringVar(&moduleVersion)
	app.Flag("repositoryRoot", "Specifies the path of the current repository (inferred automatically via git).").PlaceHolder("root").StringVar(&repositoryRoot)
	app.Flag("addContents", "Embedded file contents into the dump.").Default("false").BoolVar(&addContents)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if repositoryRoot == "" {
		toplevel, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return fmt.Errorf("get git root: %v", err)
		}
		repositoryRoot = string(toplevel)
	}

	projectRoot, err := filepath.Abs(".")
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
		addContents,
		toolInfo,
		out,
	)

	start := time.Now()
	s, err := indexer.Index()

	// End progress line before printing summary or error
	fmt.Println()

	if err != nil {
		return fmt.Errorf("index: %v", err)
	}

	fmt.Printf("%d package(s), %d file(s), %d def(s), %d element(s)\n", s.NumPkgs, s.NumFiles, s.NumDefs, s.NumElements)
	fmt.Println("Processed in", time.Since(start))
	return nil
}
