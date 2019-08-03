package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sourcegraph/lsif-go/export"
	"github.com/sourcegraph/lsif-go/log"
	"github.com/sourcegraph/lsif-go/protocol"
)

func init() {
	usage := `
Examples:

  Generate an LSIF dump for a workspace:

    	$ lsif-go export -workspace=myrepo -output=myrepo.lsif

`

	flagSet := flag.NewFlagSet("export", flag.ExitOnError)
	usageFunc := func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'lsif-go %s':\n", flagSet.Name())
		flagSet.PrintDefaults()
		fmt.Println(usage)
	}
	var (
		workspaceFlag      = flagSet.String("workspace", "", `The path to the workspace. (required)`)
		outputFlag         = flagSet.String("output", "data.lsif", `The output location of the dump.`)
		excludeContentFlag = flagSet.Bool("exclude-content", false, `Whether or not to exclude document content in the dump.`)
	)

	handler := func(args []string) error {
		flagSet.Parse(args)

		start := time.Now()

		out, err := os.Create(*outputFlag)
		if err != nil {
			return fmt.Errorf("create dump file: %v", err)
		}
		defer out.Close()

		s, err := export.Export(*workspaceFlag, *excludeContentFlag, out,
			protocol.ToolInfo{
				Name:    "lsif-go",
				Version: version,
				Args:    args,
			})
		if err != nil {
			return fmt.Errorf("export: %v", err)
		}

		log.Printf("%d package(s), %d file(s), %d def(s), %d element(s)", s.NumPkgs, s.NumFiles, s.NumDefs, s.NumElements)
		log.Println("Processed in", time.Since(start))
		return nil
	}

	// Register the command
	commands = append(commands, &command{
		flagSet:   flagSet,
		handler:   handler,
		usageFunc: usageFunc,
	})
}
