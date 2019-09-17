// The program lsif-go is an LSIF indexer for Go.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alecthomas/kingpin"
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
		debug       bool
		verbose     bool
		projectRoot string
		noContents  bool
		outFile     string
		stdout      bool
	)

	app := kingpin.New("lsif-go", "lsif-go is an LSIF indexer for Go.").Version(versionString)
	app.Flag("debug", "Display debug information.").Default("false").BoolVar(&debug)
	app.Flag("verbose", "Display verbose information.").Short('v').Default("false").BoolVar(&verbose)
	app.Flag("projectRoot", "Specifies the project root. Defaults to the current working directory.").Default(".").StringVar(&projectRoot)
	app.Flag("noContents", "File contents will not be embedded into the dump.").Default("false").BoolVar(&noContents)
	app.Flag("out", "The output file the dump is save to.").StringVar(&outFile)
	app.Flag("stdout", "Writes the dump to stdout.").Default("false").BoolVar(&stdout)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if outFile == "" && !stdout {
		return fmt.Errorf("either an output file using --out or --stdout must be specified")
	}

	if debug {
		log.SetLevel(log.Debug)
	}

	if verbose {
		log.SetLevel(log.Info)
	}

	if stdout && (verbose || debug) {
		return fmt.Errorf("debug and verbose options cannot be enabled with --stdout")
	}

	start := time.Now()

	var out io.Writer
	if stdout {
		out = os.Stdout
	} else {
		file, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("create dump file: %v", err)
		}

		defer file.Close()
		out = file
	}

	s, err := index.Index(
		projectRoot,
		noContents,
		out,
		protocol.ToolInfo{
			Name:    "lsif-go",
			Version: version,
			Args:    os.Args[1:],
		},
	)
	if err != nil {
		return fmt.Errorf("index: %v", err)
	}

	if !stdout {
		log.Printf("%d package(s), %d file(s), %d def(s), %d element(s)", s.NumPkgs, s.NumFiles, s.NumDefs, s.NumElements)
		log.Println("Processed in", time.Since(start))
	}

	return nil
}
