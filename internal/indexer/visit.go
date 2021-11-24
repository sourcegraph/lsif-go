package indexer

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/sourcegraph/lsif-go/internal/output"
	"github.com/sourcegraph/lsif-go/internal/parallel"
)

// visitEachRawFile invokes the given visitor function on each file reachable from the given set of
// packages. The file info object passed to the given callback function does not have an associated
// document value. This method prints the progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachRawFile(name string, fn func(filename string)) {
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
			if i.outputOptions.Verbosity >= output.VeryVerboseOutput {
				log.Printf("\tPackage %s", p.ID)
			}

			for _, f := range p.Syntax {
				filename := p.Fset.Position(f.Package).Filename

				if i.outputOptions.Verbosity >= output.VeryVeryVerboseOutput {
					log.Printf("\t\tFile %s", filename)
				}

				fn(filename)
				atomic.AddUint64(&count, 1)
			}
		}
	}()

	output.WithProgressParallel(&wg, name, i.outputOptions, &count, n)
}

// visitEachPackage invokes the given visitor function on each indexed package. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachPackage(name string, fn func(p *PackageInfo)) {
	ch := make(chan func())

	go func() {
		defer close(ch)

		for _, p := range i.packages {
			t := p
			ch <- func() {
				if i.outputOptions.Verbosity >= output.VeryVerboseOutput {
					log.Printf("\tPackage %s", p.ID)
				}

				fn(t)
			}
		}
	}()

	n := uint64(len(i.packages))
	wg, count := parallel.Run(ch)
	output.WithProgressParallel(wg, name, i.outputOptions, count, n)
}

// visitEachDefinitionInfo invokes the given visitor function on each definition info value. This method
// prints the progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachDefinitionInfo(name string, fn func(d *DefinitionInfo)) {
	maps := []map[interface{}]*DefinitionInfo{
		i.consts,
		i.funcs,
		i.imports,
		i.labels,
		i.types,
		i.vars,
	}

	n := uint64(0)
	for _, m := range maps {
		n += uint64(len(m))
	}

	ch := make(chan func())

	go func() {
		defer close(ch)

		for _, m := range maps {
			for _, d := range m {
				t := d
				ch <- func() { fn(t) }
			}
		}
	}()

	wg, count := parallel.Run(ch)
	output.WithProgressParallel(wg, name, i.outputOptions, count, n)
}

// visitEachDocument invokes the given visitor function on each document. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachDocument(name string, fn func(d *DocumentInfo)) {
	ch := make(chan func())

	go func() {
		defer close(ch)

		for _, d := range i.documents {
			t := d
			ch <- func() { fn(t) }
		}
	}()

	n := uint64(len(i.documents))
	wg, count := parallel.Run(ch)
	output.WithProgressParallel(wg, name, i.outputOptions, count, n)
}
