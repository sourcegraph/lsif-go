package indexer

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// runParallel will run the functions read from the given channel concurrently. This function
// returns a wait group synchronized on the invocation functions, a channel on which any error
// values are written, and a pointer to the number of tasks that have completed, which is
// updated atomically.
func runParallel(ch <-chan func() error) (*sync.WaitGroup, <-chan error, *uint64) {
	var count uint64
	errs := make(chan error, 1)
	var wg sync.WaitGroup

	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for fn := range ch {
				if err := fn(); err != nil {
					select {
					case errs <- err:
					default:
						for range ch {
						}

						return
					}
				}

				atomic.AddUint64(&count, 1)
			}
		}()
	}

	go func() {
		defer close(errs)
		wg.Wait()
	}()

	return &wg, errs, &count
}
