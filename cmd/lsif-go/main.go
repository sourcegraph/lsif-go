// The program lsif-go is an LSIF indexer for Go.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/index"
	"github.com/sourcegraph/lsif-go/protocol"
)

const version = "0.5.0"
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
		moduleRoot     string
		addContents    bool
	)

	app := kingpin.New("lsif-go", "lsif-go is an LSIF indexer for Go.").Version(versionString)
	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')
	app.HelpFlag.Hidden()

	app.Flag("out", "The output file.").Short('o').Default("dump.lsif").StringVar(&outFile)
	app.Flag("moduleVersion", "Specifies the version of the module defined by this project.").PlaceHolder("version").StringVar(&moduleVersion)
	app.Flag("repositoryRoot", "Specifies the path of the current repository (inferred automatically via git).").PlaceHolder("root").StringVar(&repositoryRoot)
	app.Flag("moduleRoot", "Specifies the module root directory relative to the repository").Default(".").StringVar(&moduleRoot)
	app.Flag("addContents", "Embed file contents into the dump.").Default("false").BoolVar(&addContents)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if repositoryRoot == "" {
		toplevel, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return fmt.Errorf("get git root: %v", err)
		}
		repositoryRoot = strings.TrimSpace(string(toplevel))
	}

	projectRoot, err := filepath.Abs(moduleRoot)
	if err != nil {
		return fmt.Errorf("get abspath of project root: %v", err)
	}

	repositoryRoot, err = filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("get abspath of repository root: %v", err)
	}

	// Sanity check: Ensure the module root is inside the repository.
	if !strings.HasPrefix(projectRoot, repositoryRoot) {
		return errors.New("module root is outside the repository")
	}

	// Ensure all the dependencies of the specified module are cached.
	if err := gomod.Download(projectRoot); err != nil {
		return fmt.Errorf("fetching dependencies: %v", err)
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

	// TODO(@creachadair): With cgo enabled, the indexer cannot handle packages
	// that include assembly (.s) files. To index such a package you need to
	// set CGO_ENABLED=0. Consider maybe doing this explicitly, always.
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
