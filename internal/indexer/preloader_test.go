package indexer

import (
	"testing"
)

func TestPreloader(t *testing.T) {
	packages := getTestPackages(t)
	preloader := preload(packages)
	p, target := findDefinitionByName(t, packages, "ParallelizableFunc")

	expectedText := normalizeDocstring(`
		ParallelizableFunc is a function that can be called concurrently with other instances
		of this function type.
	`)

	t.Run("Text", func(t *testing.T) {
		for _, f := range p.Syntax {
			if text := normalizeDocstring(preloader.Text(f, target.Pos())); text != "" {
				if text != expectedText {
					t.Errorf("unexpected hover text. want=%q have=%q", expectedText, text)
				}

				return
			}
		}

		t.Fatalf("did not find target name")
	})

	t.Run("TextFromPackage", func(t *testing.T) {
		if text := normalizeDocstring(preloader.TextFromPackage(p, target.Pos())); text != expectedText {
			t.Errorf("unexpected hover text. want=%q have=%q", expectedText, text)
		}
	})
}
