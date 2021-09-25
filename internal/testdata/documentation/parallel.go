package testdata

import (
	"context"
	"sync"
)

// ParallelizableFunc is a function that can be called concurrently with other instances
// of this function type.
type ParallelizableFunc func(ctx context.Context) error

// Parallel invokes each of the given parallelizable functions in their own goroutines and
// returns the first error to occur. This method will block until all goroutines have returned.
func Parallel(ctx context.Context, fns ...ParallelizableFunc) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(fns))

	for _, fn := range fns {
		wg.Add(1)

		go func(fn ParallelizableFunc) {
			errs <- fn(ctx)
			wg.Done()
		}(fn)
	}

	wg.Wait()

	for err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}
