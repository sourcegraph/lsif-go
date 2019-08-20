package gomod

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// MaxToken is the maximum size of one JSON input line
const MaxToken = 1024 * 1024 * 1024

func Decorate(in io.Reader, out io.Writer, projectRoot string) error {
	packageName, packageVersion, err := readModFile(projectRoot)
	if err != nil {
		return err
	}

	dependencies, err := readSumFile(projectRoot)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, MaxToken), MaxToken)
	decorator := newDecorator(out, packageName, packageVersion, dependencies)

	for scanner.Scan() {
		_, err := io.Copy(out, bytes.NewReader([]byte("HONK:"+scanner.Text()+"\n")))
		if err != nil {
			return fmt.Errorf("failed to write: %v", err)
		}

		if err := decorator.decorate(scanner.Text()); err != nil {
			return err
		}
	}

	return scanner.Err()
}
