package indexer

import (
	"go/token"
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

const TestPositionSize = 100000

func TestFindFirstIntersectingIndex(t *testing.T) {
	positions := make([]token.Pos, 0, TestPositionSize)
	for i := 0; i < TestPositionSize; i++ {
		positions = append(positions, token.Pos(i+500))
	}

	// first
	if idx := findFirstIntersectingIndex(&node{token.Pos(500)}, positions); idx != 0 {
		t.Errorf("unexpected index. want=%d have=%d", 0, idx)
	}

	// middle
	if idx := findFirstIntersectingIndex(&node{token.Pos(TestPositionSize/2 + 500)}, positions); idx != TestPositionSize/2 {
		t.Errorf("unexpected index. want=%d have=%d", TestPositionSize/2, idx)
	}

	// last
	if idx := findFirstIntersectingIndex(&node{token.Pos(TestPositionSize + 500 - 1)}, positions); idx != TestPositionSize-1 {
		t.Errorf("unexpected index. want=%d have=%d", TestPositionSize-1, idx)
	}

	// before
	if idx := findFirstIntersectingIndex(&node{token.Pos(100)}, positions); idx != 0 {
		t.Errorf("unexpected index. want=%d have=%d", 0, idx)
	}

	// after
	if idx := findFirstIntersectingIndex(&node{token.Pos(TestPositionSize * 2)}, positions); idx != TestPositionSize {
		t.Errorf("unexpected index. want=%d have=%d", TestPositionSize, idx)
	}
}

type node struct {
	pos token.Pos
}

func (n *node) Pos() token.Pos { return n.pos }
func (n *node) End() token.Pos { return n.pos }
