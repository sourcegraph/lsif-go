// The program lsif-gomod adds gomod moniker support to lsif-go output.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/sourcegraph/lsif-go/gomod"
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
		projectRoot string
		inFile      string
		stdin       bool
		outFile     string
		stdout      bool
	)

	app := kingpin.New("lsif-gomod", "lsif-gomod decorates lsif-go output with gomod monikers.").Version(versionString)
	app.Flag("projectRoot", "Specifies the project root. Defaults to the current working directory.").Default(".").StringVar(&projectRoot)
	app.Flag("in", "Specifies the file that contains a LSIF dump.").StringVar(&inFile)
	app.Flag("stdin", "Reads the dump from stdin.").Default("false").BoolVar(&stdin)
	app.Flag("out", "The output file the converted dump is saved to.").StringVar(&outFile)
	app.Flag("stdout", "Writes the dump to stdout.").Default("false").BoolVar(&stdout)

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if inFile == "" && !stdin {
		return fmt.Errorf("either an input file using --in or --stdin must be specified")
	}

	if outFile == "" && !stdout {
		return fmt.Errorf("either an output file using --out or --stdout must be specified")
	}

	var in io.Reader
	if stdin {
		in = os.Stdin
	} else {
		file, err := os.Open(inFile)
		if err != nil {
			return fmt.Errorf("open dump file: %v", err)
		}

		defer file.Close()
		in = file
	}

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

	return gomod.Decorate(in, out, projectRoot)
}
