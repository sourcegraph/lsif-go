package git

import (
	"github.com/sourcegraph/lsif-go/internal/command"
)

// Check returns true if the current directory is in a git repository.
func Check(dir string) bool {
	_, err := command.Run(dir, "git", "rev-parse", "HEAD")
	return err == nil
}

// changed again
