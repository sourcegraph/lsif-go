package main

import (
	"fmt"
	"time"

	"github.com/sourcegraph/lsif-go/internal/indexer"
)

func displayStats(indexerStats *indexer.IndexerStats, elapsed time.Duration, heapAlloc uint64) {
	fmt.Println()
	fmt.Printf("Total elapsed: %s\n", elapsed)
	fmt.Printf("Peak heap allocations: %dMB\n", heapAlloc/1024/1024)

	fmt.Println()
	fmt.Printf("Packages indexed: %d\n", indexerStats.NumPkgs)
	fmt.Printf("Files indexed: %d\n", indexerStats.NumFiles)
	fmt.Printf("Definitions indexed: %d\n", indexerStats.NumDefs)
	fmt.Printf("Elements emitted: %d\n", indexerStats.NumElements)
}
