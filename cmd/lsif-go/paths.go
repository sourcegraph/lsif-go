package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/lsif-go/internal/git"
)

var wd = newCachedString(func() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}

	return ""
})

var toplevel = newCachedString(func() string {
	if toplevel, err := git.TopLevel("."); err == nil {
		return toplevel
	}

	return ""
})

func searchForGoMod(path, repositoryRoot string) string {
	for ; !strings.HasPrefix(path, repositoryRoot); path = filepath.Dir(path) {
		_, err := os.Stat(filepath.Join(path, "go.mod"))
		if err == nil {
			return rel(path)
		}

		if !os.IsNotExist(err) {
			// Actual FS error, stop
			break
		}

		if filepath.Dir(path) == path {
			// We just checked the root, prevent infinite loop
			break
		}
	}

	return "."
}

func rel(path string) string {
	relative, err := filepath.Rel(wd.Value(), path)
	if err != nil {
		return "."
	}

	return relative
}
