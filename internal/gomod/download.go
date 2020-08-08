package gomod

import (
	"fmt"

	"github.com/sourcegraph/lsif-go/internal/command"
)

// Download runs go mod download in the given directory if it contains a go.mod file.
func Download(dir string) error {
	if !isModule(dir) {
		return nil
	}

	if output, err := command.Run(dir, "go", "mod", "download"); err != nil {
		return fmt.Errorf("failed to download modules: %v\n%s", err, output)
	}

	return nil
}
