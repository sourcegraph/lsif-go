package main

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"
)

// heapSampleDuration is how frequently mem stats are read.
const heapSampleDuration = time.Millisecond * 25

// maxAlloc is the maximum HeapAlloc stat during the run of this program.
var maxAlloc uint64

// monitorHeap will continuously read heap stats and update maxAlloc.
func monitorHeap(ctx context.Context) {
	for {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		if m.Alloc > atomic.LoadUint64(&maxAlloc) {
			atomic.StoreUint64(&maxAlloc, m.HeapAlloc)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(heapSampleDuration):
		}
	}
}
