package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/sourcegraph/lsif-go/protocol"
)

const version = "0.1.0"

func init() {
	usage := `
Examples:

  Display the tool and protocol version:

    	$ lsif-go version

`

	flagSet := flag.NewFlagSet("version", flag.ExitOnError)
	usageFunc := func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'lsif-go %s':\n", flagSet.Name())
		flagSet.PrintDefaults()
		fmt.Println(usage)
	}

	handler := func(args []string) error {
		flagSet.Parse(args)

		log.Println("Go LSIF exporter:", version)
		log.Println("Protocol version:", protocol.Version)

		return nil
	}

	// Register the command
	commands = append(commands, &command{
		flagSet:   flagSet,
		handler:   handler,
		usageFunc: usageFunc,
	})
}
