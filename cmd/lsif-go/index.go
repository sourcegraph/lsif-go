package main

import (
	"fmt"
	"os"
	"time"

	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/lsif-go/internal/indexer"
	"github.com/sourcegraph/lsif-go/internal/output"
	protocol "github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer"
)

func writeIndex(repositoryRoot, repositoryRemote, projectRoot, moduleName, moduleVersion string, dependencies map[string]gomod.GoModule, projectDependencies []string, outFile string, outputOptions output.Options) error {
	start := time.Now()

	out, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create dump file: %v", err)
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
		repositoryRemote,
		projectRoot,
		toolInfo,
		moduleName,
		moduleVersion,
		dependencies,
		projectDependencies,
		writer.NewJSONWriter(out),
		packageDataCache,
		outputOptions,
	)

	if err := indexer.Index(); err != nil {
		return err
	}

	if isVerbose() {
		displayStats(indexer.Stats(), packageDataCache.Stats(), start)
	}

	return nil
}

var verbosityLevels = map[int]output.Verbosity{
	0: output.DefaultOutput,
	1: output.VerboseOutput,
	2: output.VeryVerboseOutput,
	3: output.VeryVeryVerboseOutput,
}

func getVerbosity() output.Verbosity {
	if noOutput {
		return output.NoOutput
	}

	if verbosity >= len(verbosityLevels) {
		verbosity = len(verbosityLevels) - 1
	}

	return verbosityLevels[verbosity]
}

func isVerbose() bool {
	return getVerbosity() >= output.VerboseOutput
}
