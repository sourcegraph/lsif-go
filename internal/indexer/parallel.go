package indexer

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// runParallel will run the functions read from teh given channel concurrently. This function
// returns a wait group synchronized on the invocation functions, a channel on which any error
// values are written, and a pointer to the number of tasks that have completed, which is
// updated atomically.
func runParallel(ch <-chan func() error) (*sync.WaitGroup, <-chan error, *uint64) {
	var wg sync.WaitGroup
	wg.Add(1)

	errs := make(chan error, 1)
	semaphore := makeSemaphore()

	go func() {
		defer close(errs)
		defer close(semaphore)

		wg.Wait()
	}()

	var count uint64

	go func() {
		defer wg.Done()

		for fn := range ch {
			wg.Add(1)

			go func(fn func() error) {
				defer wg.Done()
				<-semaphore
				defer func() { semaphore <- struct{}{} }()

				if err := fn(); err != nil {
					select {
					case errs <- err:
					default:
						return
					}
				}

				atomic.AddUint64(&count, 1)
			}(fn)
		}
	}()

	return &wg, errs, &count
}

// makeSemaphore constructs a buffered channel that can be used to limit the number
// of active goroutines running. The channel will contain as many values as there are
// available cores.
func makeSemaphore() chan struct{} {
	concurrency := runtime.GOMAXPROCS(0)

	semaphore := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		semaphore <- struct{}{}
	}

	return semaphore
}
