package gomod

import (
	"fmt"
	"log"
	"strings"

	"github.com/sourcegraph/lsif-go/internal/command"
	"github.com/sourcegraph/lsif-go/internal/output"
	"golang.org/x/tools/go/vcs"
)

// ModuleName returns the resolved name of the go module declared in the given
// directory usable for moniker identifiers. Note that this is distinct from the
// declared module as this does not uniquely identify a project via its code host
// coordinates in the presence of forks.
func ModuleName(dir, repo string, outputOptions output.Options) (moduleName string, err error) {
	resolve := func() {
		name := repo

		if !isModule(dir) {
			log.Println("WARNING: No go.mod file found in current directory.")
		} else {
			if name, err = command.Run(dir, "go", "list", "-mod=readonly", "-m"); err != nil {
				err = fmt.Errorf("failed to list modules: %v\n%s", err, name)
				return
			}
		}

		moduleName, err = resolveModuleName(repo, name)
	}

	output.WithProgress("Resolving module name", resolve, outputOptions)
	return moduleName, err
}

// resolveModuleName converts the given repository and import path into a canonical
// representation of a module name usable for moniker identifiers. The base of the
// import path will be the resolved repository remote, and the given module name
// is used only to determine the path suffix.
func resolveModuleName(repo, name string) (string, error) {
	// Determine path suffix relative to repository root
	var suffix string

	if nameRepoRoot, err := vcs.RepoRootForImportPath(name, false); err == nil {
		suffix = strings.TrimPrefix(name, nameRepoRoot.Root)
	} else {
		// A user-visible warning will occur on this path as the declared
		// module will be resolved as part of gomod.ListDependencies.
	}

	// Determine the canonical code host of the current repository
	repoRepoRoot, err := vcs.RepoRootForImportPath(repo, false)
	if err != nil {
		help := "Make sure your git repo has a remote (git remote add git@github.com:owner/repo)"
		return "", fmt.Errorf("%v\n\n%s", err, help)
	}

	return repoRepoRoot.Repo + suffix, nil
}
