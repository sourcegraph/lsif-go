package main

import (
	"fmt"
	"os"
	"time"

	"github.com/sourcegraph/lsif-go/internal/indexer"
	"github.com/sourcegraph/lsif-go/internal/writer"
	"github.com/sourcegraph/lsif-go/protocol"
)

func writeIndex(repositoryRoot, projectRoot, moduleName, moduleVersion string, dependencies map[string]string, outFile string) error {
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

	preloader := indexer.NewPreloader()

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
		preloader,
		!noProgress,
		noOutput,
		verboseOutput,
	)

	if err := indexer.Index(); err != nil {
		return fmt.Errorf("index: %v", err)
	}

	if !noOutput && verboseOutput {
		displayStats(indexer.Stats(), preloader.Stats(), start)
	}

	return nil
}
