package command

import (
	"os/exec"
	"strings"
)

// Run runs the given command using the given working directory. If the command succeeds,
// the value of stdout is returned with trailing whitespace removed. If the command fails,
// the combined stdout/stderr text will also be returned.
func Run(dir, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
