package indexer

import (
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/packages"
)

// visitEachRawFile invokes the given visitor function on each file reachable from the given set of
// packages. The file info object passed to the given callback function does not have an associated
// document value. This method prints the progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachRawFile(name string, animate, silent bool, fn func(filename string)) {
	n := 0
	for _, p := range i.packages {
		n += len(p.Syntax)
	}

	visitWithProgress(name, animate, silent, uint64(n), func(count *uint64) {
		for _, p := range i.packages {
			for _, f := range p.Syntax {
				fn(p.Fset.Position(f.Package).Filename)
				atomic.AddUint64(count, 1)
			}
		}
	})
}

// visitEachPackage invokes the given visitor function on each indexed package. This method prints the progress
// of the traversal to stdout asynchronously.
func (i *Indexer) visitEachPackage(name string, animate, silent bool, fn func(p *packages.Package)) {
	visitWithProgress(name, animate, silent, uint64(len(i.documents)), func(count *uint64) {
		for _, p := range i.packages {
			fn(p)
			atomic.AddUint64(count, 1)
		}
	})
}

// visitEachReferenceResult invokes the given visitor function on each reference result. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachReferenceResult(name string, animate, silent bool, fn func(referenceResult *ReferenceResultInfo)) {
	visitWithProgress(name, animate, silent, uint64(len(i.referenceResults)), func(count *uint64) {
		for _, r := range i.referenceResults {
			fn(r)
			atomic.AddUint64(count, 1)
		}
	})
}

// visitWithProgress calls the given function in a goroutine. This function prints the progress of the function
// (determined by the function updating the given integer pointer atomically) to stdout asynchronously.
func visitWithProgress(name string, animate, silent bool, n uint64, fn func(count *uint64)) {
	var count uint64
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		fn(&count)
	}()

	withProgress(&wg, name, animate, silent, &count, &n)
}
