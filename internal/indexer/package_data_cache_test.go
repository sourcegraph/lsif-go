package indexer

import "testing"

func TestPackageDataCache(t *testing.T) {
	packages := getTestPackages(t)
	p, obj := findDefinitionByName(t, packages, "ParallelizableFunc")

	expectedText := normalizeDocstring(`
		ParallelizableFunc is a function that can be called concurrently with other instances
		of this function type.
	`)

	if text := normalizeDocstring(NewPackageDataCache().Text(p, obj.Pos())); text != "" {
		if text != expectedText {
			t.Errorf("unexpected hover text. want=%q have=%q", expectedText, text)
		}

		return
	}

	t.Fatalf("did not find target name")
}
