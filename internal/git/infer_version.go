package git

import (
	"fmt"

	"github.com/sourcegraph/lsif-go/internal/command"
)

// InferModuleVersion returns the version of the module declared in the given
// directory. This will be either the work tree commit's tag, or it will be the
// most recent tag with a short revhash appended to it.
func InferModuleVersion(dir string) (string, error) {
	if version, err := getCurrentTag(dir); err != nil || version != "" {
		return version, err
	}

	return constructVersion(dir)
}

// getCurrentTag returns the tag of the work tree's commit, if one exists.
func getCurrentTag(dir string) (string, error) {
	output, err := command.Run(dir, "git", "tag", "-l", "--points-at", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to tags for current commit: %v\n%s", err, output)
	}

	return output, nil
}

// constructVersion finds the most recent tag and appends the 12-character prefix
// of the work tree's commit. This mirrors what happens when you run go mod update.
func constructVersion(dir string) (string, error) {
	tag, err := getMostRecentTag(dir)
	if err != nil {
		return "", err
	}

	output, err := command.Run(dir, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current commit: %v\n%s", err, output)
	}

	return fmt.Sprintf("%s-%s", tag, output[:12]), nil
}

// getMostRecentTag returns the tag attached to the closest ancestor commit.
func getMostRecentTag(dir string) (string, error) {
	hasTags, err := hasTags(dir)
	if err != nil {
		return "", err
	}
	if hasTags {
		// Get the most recent tag. We only want to run this command if there _are_
		// tags in the current repository. If we don't do the check above, this
		// command will exist with status 128.
		output, err := command.Run(dir, "git", "describe", "--tags", "--abbrev=0")
		if err != nil {
			return "", fmt.Errorf("failed to get most recent tag: %v\n%s", err, output)
		}

		return output, nil
	}

	// If we have no tags, just return a canned version.
	return "v0.0.0", nil
}

// hasTags returns true if git tags exist in the given directory.
func hasTags(dir string) (bool, error) {
	output, err := command.Run(dir, "git", "tag")
	if err != nil {
		return false, fmt.Errorf("failed to list tags: %v\n%s", err, output)
	}

	return output != "", nil
}
