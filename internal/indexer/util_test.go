package indexer

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUnion(t *testing.T) {
	u := union(
		[]uint64{10, 20, 30},
		[]uint64{100, 200, 300},
		[]uint64{10, 200, 3000},
	)
	sort.Slice(u, func(i, j int) bool {
		return u[i] < u[j]
	})

	expected := []uint64{
		10, 20, 30,
		100, 200, 300,
		3000,
	}

	if diff := cmp.Diff(expected, u); diff != "" {
		t.Errorf("unexpected union (-want +got): %s", diff)
	}
}
