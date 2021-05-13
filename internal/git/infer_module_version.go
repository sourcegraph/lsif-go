package git

import (
	"fmt"

	"github.com/sourcegraph/lsif-go/internal/command"
)

// InferModuleVersion returns the version of the module declared in the given
// directory. This will be either the work tree commit's tag, or it will be the
// most recent tag with a short revhash appended to it.
func InferModuleVersion(dir string) (string, error) {
	version, err := command.Run(dir, "git", "tag", "-l", "--points-at", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to tags for current commit: %v\n%s", err, version)
	}
	if version != "" {
		return version, nil
	}

	commit, err := command.Run(dir, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current commit: %v\n%s", err, commit)
	}

	tag, err := command.Run(dir, "git", "describe", "--tags", "--abbrev=0", "--always")
	if err != nil {
		return "", fmt.Errorf("failed to describe tags: %v\n%s", err, commit)
	}
	if tag == commit {
		tag = "v0.0.0"
	}

	return fmt.Sprintf("%s-%s", tag, commit[:12]), nil
}
