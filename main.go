// The program lsif-go is an LSIF exporter for Go.
package main

import (
	"flag"
	"os"
)

const usageText = `lsif-go is an LSIF exporter for Go.
For more information, see https://github.com/sourcegraph/lsif-go

Usage:

	lsif-go [options] command [command options]

The options are:
	debug           display debug information
	verbose         display verbose information

The commands are:

	export          generates an LSIF dump for a workspace
	version         display version information

Use "lsif-go [command] -h" for more information about a command.

`

// commands contains all registered subcommands.
var commands commander

var (
	debug   = flag.Bool("debug", false, `To display debug information.`)
	verbose = flag.Bool("verbose", false, `To display verbose information.`)
)

func main() {
	commands.run(flag.CommandLine, "lsif-go", usageText, os.Args[1:])
}
