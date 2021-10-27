package indexer

// union concatenates, flattens, and deduplicates the given identifier slices.
func union(as ...[]uint64) (flattened []uint64) {
	m := map[uint64]struct{}{}
	for _, a := range as {
		for _, v := range a {
			m[v] = struct{}{}
		}
	}

	for v := range m {
		flattened = append(flattened, v)
	}

	return flattened
}

// random change
