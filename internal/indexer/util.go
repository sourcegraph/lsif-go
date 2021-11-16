package indexer

import (
	"runtime"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

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

func displayMemStats() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	p := message.NewPrinter(language.English)
	p.Printf("%10d MB\n", memStats.HeapAlloc/10e6)
}
