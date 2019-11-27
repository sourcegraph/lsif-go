// The program lsif-go is an LSIF indexer for Go.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/sourcegraph/lsif-go/gomod"
	"github.com/sourcegraph/lsif-go/index"
	"github.com/sourcegraph/lsif-go/log"
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
		debug         bool
		verbose       bool
		projectRoot   string
		moduleVersion string
		noContents    bool
		outFile       string
	)

	app := kingpin.New("lsif-go", "lsif-go is an LSIF indexer for Go.").Version(versionString)
	app.Flag("debug", "Display debug information.").Default("false").BoolVar(&debug)
	app.Flag("verbose", "Display verbose information.").Short('v').Default("false").BoolVar(&verbose)
	app.Flag("projectRoot", "Specifies the project root. Defaults to the current working directory.").Default(".").StringVar(&projectRoot)
	app.Flag("moduleVersion", "Specifies the version of the module defined by this project.").StringVar(&moduleVersion)
	app.Flag("noContents", "File contents will not be embedded into the dump.").Default("false").BoolVar(&noContents)
	app.Flag("out", "The output file the dump is saved to.").StringVar(&outFile)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if outFile == "" {
		outFile = "dump.lsif"
	}

	if verbose {
		log.SetLevel(log.Info)
	}

	if debug {
		log.SetLevel(log.Debug)
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

	start := time.Now()

	s, err := index.Index(
		projectRoot,
		noContents,
		out,
		moduleName,
		moduleVersion,
		dependencies,
		protocol.ToolInfo{
			Name:    "lsif-go",
			Version: version,
			Args:    os.Args[1:],
		},
	)

	if err != nil {
		return fmt.Errorf("index: %v", err)
	}

	log.Printf("%d package(s), %d file(s), %d def(s), %d element(s)", s.NumPkgs, s.NumFiles, s.NumDefs, s.NumElements)
	log.Println("Processed in", time.Since(start))
	return nil
}
