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
	out, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("create dump file: %v", err)
	}
	defer out.Close()

	indexer := makeIndexer(
		repositoryRoot,
		projectRoot,
		moduleName,
		moduleVersion,
		dependencies,
		out,
	)

	start := time.Now()
	stats, err := indexer.Index()
	if err != nil {
		return fmt.Errorf("index: %v", err)
	}

	fmt.Println()
	fmt.Printf("%d package(s), %d file(s), %d def(s), %d element(s)\n", stats.NumPkgs, stats.NumFiles, stats.NumDefs, stats.NumElements)
	fmt.Println("Processed in", time.Since(start))
	return nil
}

func makeIndexer(repositoryRoot, projectRoot, moduleName, moduleVersion string, dependencies map[string]string, out *os.File) *indexer.Indexer {
	toolInfo := protocol.ToolInfo{
		Name:    "lsif-go",
		Version: version,
		Args:    os.Args[1:],
	}

	// TODO(efritz) - With cgo enabled, the indexer cannot handle packages
	// that include assembly (.s) files. To index such a package you need to
	// set CGO_ENABLED=0. Consider maybe doing this explicitly, always.
	return indexer.New(
		repositoryRoot,
		projectRoot,
		toolInfo,
		moduleName,
		moduleVersion,
		dependencies,
		writer.NewJSONWriter(out),
		!noProgress,
	)
}
