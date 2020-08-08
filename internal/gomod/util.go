package gomod

import (
	"os"
	"path/filepath"
)

// isModule returns true if there is a go.mod file in the given directory.
func isModule(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}
