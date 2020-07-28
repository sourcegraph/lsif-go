package index

import (
	"go/token"
	"testing"
)

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
