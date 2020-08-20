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
	n := uint64(0)
	for _, p := range i.packages {
		n += uint64(len(p.Syntax))
	}

	var count uint64
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		for _, p := range i.packages {
			for _, f := range p.Syntax {
				fn(p.Fset.Position(f.Package).Filename)
				atomic.AddUint64(&count, 1)
			}
		}
	}()

	withProgress(&wg, name, animate, silent, &count, n)
}

// visitEachPackage invokes the given visitor function on each indexed package. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachPackage(name string, animate, silent bool, fn func(p *packages.Package)) {
	ch := make(chan func())

	go func() {
		defer close(ch)

		for _, p := range i.packages {
			t := p
			ch <- func() { fn(t) }
		}
	}()

	n := uint64(len(i.packages))
	wg, count := runParallel(ch)
	withProgress(wg, name, i.animate, i.silent, count, n)
}

// visitEachReferenceResult invokes the given visitor function on each reference result. This method
// prints the progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachReferenceResult(name string, animate, silent bool, fn func(referenceResult *ReferenceResultInfo)) {
	ch := make(chan func())

	go func() {
		defer close(ch)

		for _, r := range i.referenceResults {
			t := r
			ch <- func() { fn(t) }
		}
	}()

	n := uint64(len(i.referenceResults))
	wg, count := runParallel(ch)
	withProgress(wg, name, i.animate, i.silent, count, n)
}

// visitEachDocument invokes the given visitor function on each document. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachDocument(name string, animate, silent bool, fn func(d *DocumentInfo)) {
	ch := make(chan func())

	go func() {
		defer close(ch)

		for _, d := range i.documents {
			t := d
			ch <- func() { fn(t) }
		}
	}()

	n := uint64(len(i.documents))
	wg, count := runParallel(ch)
	withProgress(wg, name, i.animate, i.silent, count, n)
}
