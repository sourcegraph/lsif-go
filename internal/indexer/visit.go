package indexer

import (
	"sync"
	"sync/atomic"
)

// visitEachRawFile invokes the given visitor function on each file reachable from the given set of
// packages. The file info object passed to the given callback function does not have an associated
// document value. This method prints the progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachRawFile(name string, animate bool, fn func(f FileInfo)) {
	n := 0
	for _, p := range i.packages {
		n += len(p.Syntax)
	}

	visitWithProgress(name, animate, uint64(n), func(count *uint64) {
		for _, p := range i.packages {
			for _, f := range p.Syntax {
				fileInfo := FileInfo{
					Package:  p,
					File:     f,
					Filename: p.Fset.Position(f.Package).Filename,
				}

				fn(fileInfo)
				atomic.AddUint64(count, 1)
			}
		}
	})
}

// visitEachFile invokes the given visitor function on each file reachable from the given set of packages that
// also has an entry in the indexer's files map. This method prints the progress of the traversal to stdout
// asynchronously.
func (i *Indexer) visitEachFile(name string, animate bool, fn func(f FileInfo)) {
	processed := map[string]struct{}{}

	visitWithProgress(name, animate, uint64(len(i.documents)), func(count *uint64) {
		for _, p := range i.packages {
			for _, f := range p.Syntax {
				filename := p.Fset.Position(f.Package).Filename

				d, hasDocument := i.documents[filename]
				if !hasDocument {
					continue
				}

				if _, isProcessed := processed[filename]; isProcessed {
					continue
				}
				processed[filename] = struct{}{}

				fileInfo := FileInfo{
					Package:  p,
					File:     f,
					Filename: filename,
					Document: d,
				}

				fn(fileInfo)
				atomic.AddUint64(count, 1)
			}
		}
	})
}

// visitEachReferenceResult invokes the given visitor function on each reference result. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachReferenceResult(name string, animate bool, fn func(referenceResult *ReferenceResultInfo)) {
	visitWithProgress(name, animate, uint64(len(i.referenceResults)), func(count *uint64) {
		for _, r := range i.referenceResults {
			fn(r)
			atomic.AddUint64(count, 1)
		}
	})
}

// visitWithProgress calls the given function in a goroutine. This function prints the progress of the function
// (determined by the function updating the given integer pointer atomically) to stdout asynchronously.
func visitWithProgress(name string, animate bool, n uint64, fn func(count *uint64)) {
	var count uint64
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		fn(&count)
	}()

	withProgress(&wg, name, animate, &count, &n)
}
