package parallel

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// Run will run the functions read from the given channel concurrently. This function
// returns a wait group synchronized on the invocation functions, a channel on which any error
// values are written, and a pointer to the number of tasks that have completed, which is
// updated atomically.
func Run(ch <-chan func()) (*sync.WaitGroup, *uint64) {
	var count uint64
	var wg sync.WaitGroup

	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for fn := range ch {
				fn()
				atomic.AddUint64(&count, 1)
			}
		}()
	}

	return &wg, &count
}
