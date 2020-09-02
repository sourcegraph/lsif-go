package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/indexer"
	"github.com/sourcegraph/lsif-go/internal/util"
	"github.com/sourcegraph/lsif-go/internal/writer"
	"github.com/sourcegraph/lsif-go/protocol"
)

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(os.Stdout)
}

func main() {
	if err := mainErr(); err != nil {
		fmt.Fprint(os.Stderr, fmt.Sprintf("\nerror: %v\n", err))
		os.Exit(1)
	}
}

func mainErr() error {
	if err := parseArgs(os.Args[1:]); err != nil {
		return err
	}

	moduleName, dependencies, err := gomod.ListModules(moduleRoot)
	if err != nil {
		return err
	}

	start := time.Now()

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

	packageDataCache := indexer.NewPackageDataCache()

	// TODO(efritz) - With cgo enabled, the indexer cannot handle packages
	// that include assembly (.s) files. To index such a package you need to
	// set CGO_ENABLED=0. Consider maybe doing this explicitly, always.
	indexer := indexer.New(
		repositoryRoot,
		projectRoot,
		toolInfo,
		moduleName,
		moduleVersion,
		dependencies,
		writer.NewJSONWriter(out),
		packageDataCache,
		!noProgress,
		noOutput,
		verboseOutput,
	)

	if err := indexer.Index(); err != nil {
		return fmt.Errorf("index: %v", err)
	}

	if !noOutput && verboseOutput {
		indexerStats := indexer.Stats()
		packageDataCacheStats := packageDataCache.Stats()

		stats := []struct {
			name  string
			value string
		}{
			{"Wall time elapsed", fmt.Sprintf("%s", util.HumanElapsed(start))},
			{"Packages indexed", fmt.Sprintf("%d", indexerStats.NumPkgs)},
			{"Files indexed", fmt.Sprintf("%d", indexerStats.NumFiles)},
			{"Definitions indexed", fmt.Sprintf("%d", indexerStats.NumDefs)},
			{"Elements emitted", fmt.Sprintf("%d", indexerStats.NumElements)},
			{"Packages traversed", fmt.Sprintf("%d", packageDataCacheStats.NumPks)},
		}

		n := 0
		for _, stat := range stats {
			if n < len(stat.name) {
				n = len(stat.name)
			}
		}

		fmt.Printf("\nStats:\n")

		for _, stat := range stats {
			fmt.Printf("\t%s: %s%s\n", stat.name, strings.Repeat(" ", n-len(stat.name)), stat.value)
		}
	}

	return nil
}
