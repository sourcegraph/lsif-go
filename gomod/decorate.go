package gomod

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// MaxToken is the maximum size of one JSON input line
const MaxToken = 1024 * 1024 * 1024

// Decorate reads JSON lines from in and writes decorated JSON lines to out.
// Lines will only be added, not modified or deleted. Each import or export
// moniker for which there is information in the project's go.mod/go.sum will
// be decorated.
func Decorate(in io.Reader, out io.Writer, projectRoot string) error {
	packageName, err := readModFile(projectRoot)
	if err != nil {
		return err
	}

	// TODO - need to find this from git checkout
	// https://github.com/golang/vgo/blob/9d567625acf4c5e156b9890bf6feb16eb9fa5c51/vendor/cmd/go/internal/modfetch/coderepo.go#L88
	packageVersion := "0.0.0"

	dependencies, err := readSumFile(projectRoot)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, MaxToken), MaxToken)
	decorator := newDecorator(out, packageName, packageVersion, dependencies)

	for scanner.Scan() {
		// Always write original line
		_, err := io.Copy(out, bytes.NewReader([]byte(scanner.Text()+"\n")))
		if err != nil {
			return fmt.Errorf("write: %v", err)
		}

		// Possibly write additional lines
		if err := decorator.decorate(scanner.Text()); err != nil {
			return err
		}
	}

	return scanner.Err()
}
