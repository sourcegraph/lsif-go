package indexer

import (
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/packages"
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
			if i.outputOptions.Verbosity >= VeryVerboseOutput {
				log.Printf("\tPackage %s", p.ID)
			}

			for _, f := range p.Syntax {
				filename := p.Fset.Position(f.Package).Filename

				if i.outputOptions.Verbosity >= VeryVeryVerboseOutput {
					log.Printf("\t\tFile %s", filename)
				}

				fn(filename)
				atomic.AddUint64(&count, 1)
			}
		}
	}()

	withProgress(&wg, name, i.outputOptions, &count, n)
}

// visitEachPackage invokes the given visitor function on each indexed package. This method prints the
// progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachPackage(name string, fn func(p *packages.Package)) {
	visitPackage := func(p *packages.Package) bool {
		log.Printf("package %q %q %q %q", p.Name, p.ID, p.PkgPath, p.Types.Path())
		for _, f := range p.Syntax {
			log.Println(" - ", p.Fset.Position(f.Name.Pos()).Filename)
		}

		// TODO(sqs): HACK, the loader returns 4 packages because (loader.Config).Tests==true and we
		// want to avoid duplication.
		if p.Name == "main" && strings.HasSuffix(p.ID, ".test]") {
			return false // synthesized `go test` program
		}
		if strings.HasSuffix(p.Name, "_test") {
			return true
		}

		// Index only the combined test package if it's present. If the package has no test files,
		// it won't be present, and we need to just index the default package.
		f0 := p.Fset.Position(p.Syntax[0].Name.Pos()).Filename
		pkgHasTests := len(i.packagesByFile[f0]) > 1
		if pkgHasTests && !strings.HasSuffix(p.ID, ".test]") {
			return false
		}

		return true
	}

	ch := make(chan func())

	go func() {
		defer close(ch)

		for _, p := range i.packages {
			if !visitPackage(p) {
				continue
			}

			t := p
			ch <- func() {
				if i.outputOptions.Verbosity >= VeryVerboseOutput {
					log.Printf("\tPackage %s", p.ID)
				}

				fn(t)
			}
		}
	}()

	n := uint64(len(i.packages))
	wg, count := runParallel(ch)
	withProgress(wg, name, i.outputOptions, count, n)
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

	wg, count := runParallel(ch)
	withProgress(wg, name, i.outputOptions, count, n)
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
	wg, count := runParallel(ch)
	withProgress(wg, name, i.outputOptions, count, n)
}
