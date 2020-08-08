package indexer

// union concatenates, flattens, and deduplicates the given string slices.
func union(as ...[]string) (flattened []string) {
	m := map[string]struct{}{}
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
