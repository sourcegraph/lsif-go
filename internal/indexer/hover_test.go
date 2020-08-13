package indexer

import (
	"go/ast"
	"testing"
)

func TestFindDocstring(t *testing.T) {
	packages := getTestPackages(t)
	p, target := findDefinitionByName(t, packages, "ParallelizableFunc")
	o := makeObjectInfo(t, "ParallelizableFunc", p, target)

	expectedText := normalizeDocstring(`
		ParallelizableFunc is a function that can be called concurrently with other instances
		of this function type.
	`)
	if text := normalizeDocstring(findDocstring(preload(packages), packages, o)); text != expectedText {
		t.Errorf("unexpected hover text. want=%q have=%q", expectedText, text)
	}
}

func TestFindDocstringInternalPackageName(t *testing.T) {
	packages := getTestPackages(t)
	p, target := findUseByName(t, packages, "secret")
	o := makeObjectInfo(t, "secret", p, target)

	expectedText := normalizeDocstring(`secret is a package that holds secrets.`)
	if text := normalizeDocstring(findDocstring(preload(packages), packages, o)); text != expectedText {
		t.Errorf("unexpected hover text. want=%q have=%q", expectedText, text)
	}
}

func TestFindDocstringExternalPackageName(t *testing.T) {
	packages := getTestPackages(t)
	p, target := findUseByName(t, packages, "sync")
	o := makeObjectInfo(t, "sync", p, target)

	expectedText := normalizeDocstring(`
		Package sync provides basic synchronization primitives such as mutual exclusion locks.
		Other than the Once and WaitGroup types, most are intended for use by low-level library routines.
		Higher-level synchronization is better done via channels and communication.
		Values containing the types defined in this package should not be copied.
	`)
	if text := normalizeDocstring(findDocstring(preload(packages), packages, o)); text != expectedText {
		t.Errorf("unexpected hover text. want=%q have=%q", expectedText, text)
	}
}

func TestFindExternalDocstring(t *testing.T) {
	packages := getTestPackages(t)
	p, target := findUseByName(t, packages, "WaitGroup")
	o := ObjectInfo{
		FileInfo: FileInfo{Package: p},
		Object:   target,
		Ident:    &ast.Ident{Name: "WaitGroup", NamePos: target.Pos()},
	}

	expectedText := normalizeDocstring(`
		A WaitGroup waits for a collection of goroutines to finish.
		The main goroutine calls Add to set the number of goroutines to wait for.
		Then each of the goroutines runs and calls Done when finished.
		At the same time, Wait can be used to block until all goroutines have finished.
		A WaitGroup must not be copied after first use.
	`)
	if text := normalizeDocstring(findExternalDocstring(preload(packages), packages, o)); text != expectedText {
		t.Errorf("unexpected hover text. want=%q have=%q", expectedText, text)
	}
}
