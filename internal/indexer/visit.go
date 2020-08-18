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

	withProgress(&wg, name, animate, silent, &count, &n)
}

// visitEachPackage invokes the given visitor function on each indexed package. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachPackage(name string, animate, silent bool, fn func(p *packages.Package)) {
	var n uint64
	ch := make(chan func() error)

	go func() {
		defer close(ch)

		for _, p := range i.packages {
			atomic.AddUint64(&n, 1)
			ch <- func(p *packages.Package) func() error {
				return func() error {
					fn(p)
					return nil
				}
			}(p)
		}
	}()

	wg, errs, count := runParallel(ch)
	withProgress(wg, name, i.animate, i.silent, count, &n)
	<-errs
}

// visitEachReferenceResult invokes the given visitor function on each reference result. This method
// prints the progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachReferenceResult(name string, animate, silent bool, fn func(referenceResult *ReferenceResultInfo)) {
	var n uint64
	ch := make(chan func() error)

	go func() {
		defer close(ch)

		for _, r := range i.referenceResults {
			atomic.AddUint64(&n, 1)
			ch <- func(r *ReferenceResultInfo) func() error {
				return func() error {
					fn(r)
					return nil
				}
			}(r)
		}
	}()

	wg, errs, count := runParallel(ch)
	withProgress(wg, name, i.animate, i.silent, count, &n)
	<-errs
}

// visitEachDocument invokes the given visitor function on each document. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachDocument(name string, animate, silent bool, fn func(d *DocumentInfo)) {
	var n uint64
	ch := make(chan func() error)

	go func() {
		defer close(ch)

		for _, d := range i.documents {
			atomic.AddUint64(&n, 1)
			ch <- func(d *DocumentInfo) func() error {
				return func() error {
					fn(d)
					return nil
				}
			}(d)
		}
	}()

	wg, errs, count := runParallel(ch)
	withProgress(wg, name, i.animate, i.silent, count, &n)
	<-errs
}
