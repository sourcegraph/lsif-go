package indexer

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUnion(t *testing.T) {
	u := union(
		[]string{"a1", "a2", "a3"},
		[]string{"b1", "b2", "b3"},
		[]string{"a1", "b2", "c3"},
	)
	sort.Strings(u)

	expected := []string{
		"a1", "a2", "a3", "b1", "b2", "b3", "c3",
	}

	if diff := cmp.Diff(expected, u); diff != "" {
		t.Errorf("unexpected union (-want +got): %s", diff)
	}
}
