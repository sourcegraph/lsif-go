package indexer

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunParallel(t *testing.T) {
	ch := make(chan func() error, 3)
	ch <- func() error { return nil }
	ch <- func() error { return nil }
	ch <- func() error { return nil }
	close(ch)

	wg, errs, n := runParallel(ch)

	wg.Wait()
	if err := <-errs; err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if *n != 3 {
		t.Errorf("unexpected count. want=%d want=%d", 3, *n)
	}
}

func TestRunParallelFailure(t *testing.T) {
	ch := make(chan func() error, 3)
	ch <- func() error { return nil }
	ch <- func() error { return fmt.Errorf("oops") }
	ch <- func() error { return nil }
	close(ch)

	wg, errs, _ := runParallel(ch)
	wg.Wait()
	if err := <-errs; err == nil || !strings.Contains(err.Error(), "oops") {
		t.Fatalf("unexpected error. want=%s have=%v", "oops", err)
	}
}

func TestRunParallelProgress(t *testing.T) {
	sync1 := make(chan struct{})
	sync2 := make(chan struct{})
	sync3 := make(chan struct{})

	ch := make(chan func() error, 3)
	ch <- func() error { <-sync1; return nil }
	ch <- func() error { <-sync2; return nil }
	ch <- func() error { <-sync3; return nil }
	close(ch)

	wg, _, n := runParallel(ch)

	checkValue := func(expected uint64) {
		var v uint64

		for i := 0; i < 10; i++ {
			if v = atomic.LoadUint64(n); v == expected {
				return
			}

			<-time.After(time.Millisecond)
		}

		t.Fatalf("unexpected progress value. want=%d have=%d", expected, v)
	}

	checkValue(0)
	close(sync1)
	checkValue(1)
	close(sync2)
	checkValue(2)
	close(sync3)
	checkValue(3)
	wg.Wait()
}
