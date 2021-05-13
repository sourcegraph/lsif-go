package git

import (
	"fmt"
	"testing"
)

func TestInferRepo(t *testing.T) {
	repo, err := InferRepo("")
	if err != nil {
		t.Fatalf("unexpected error inferring repo: %s", err)
	}

	if repo != "github.com/sourcegraph/lsif-go" {
		t.Errorf("unexpected remote repo. want=%q have=%q", "github.com/sourcegraph/lsif-go", repo)
	}
}

func TestParseRemote(t *testing.T) {
	testCases := map[string]string{
		"git@github.com:sourcegraph/lsif-go.git": "github.com/sourcegraph/lsif-go",
		"https://github.com/sourcegraph/lsif-go": "github.com/sourcegraph/lsif-go",
	}

	for input, expectedOutput := range testCases {
		t.Run(fmt.Sprintf("input=%q", input), func(t *testing.T) {
			output, err := parseRemote(input)
			if err != nil {
				t.Fatalf("unexpected error parsing remote: %s", err)
			}

			if output != expectedOutput {
				t.Errorf("unexpected repo name. want=%q have=%q", expectedOutput, output)
			}
		})
	}
}
