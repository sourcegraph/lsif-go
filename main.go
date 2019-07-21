// The program lsif-go is an LSIF exporter for Go.
package main

import (
	"flag"
	"log"
	"os"
)

const usageText = `lsif-go is an LSIF exporter for Go.
For more information, see https://github.com/sourcegraph/lsif-go

Usage:

	lsif-go [options] command [command options]

The options are:

The commands are:

	export          generates an LSIF dump for a workspace
	version         display version information

Use "lsif-go [command] -h" for more information about a command.

`

// commands contains all registered subcommands.
var commands commander

func main() {
	// Configure logging
	log.SetFlags(0)
	log.SetPrefix("")

	commands.run(flag.CommandLine, "lsif-go", usageText, os.Args[1:])
}
