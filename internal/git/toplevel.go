package git

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/lsif-go/internal/command"
)

// TopLevel returns the root of the git project containing the given directory.
func TopLevel(dir string) (string, error) {
	output, err := command.Run(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("failed to get toplevel: %v\n%s", err, output)
	}

	return strings.TrimSpace(string(output)), nil
}
