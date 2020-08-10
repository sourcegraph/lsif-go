package indexer

import "github.com/efritz/pentimento"

// visitEachRawFile invokes the given visitor function on each file reachable from the given set of
// packages. The file info object passed to the given callback function does not have an associated
// document value. This method prints the progress of the traversal to stdout asynchronously.
func (i *Indexer) visitEachRawFile(name string, animate bool, fn func(f FileInfo)) {
	n := 0
	for _, p := range i.packages {
		n += len(p.Syntax)
	}

	_ = withTitle(name, animate, func(printer *pentimento.Printer) error {
		c := 0
		for _, p := range i.packages {
			for _, f := range p.Syntax {
				fn(FileInfo{
					Package:  p,
					File:     f,
					Filename: p.Fset.Position(f.Package).Filename,
				})

				c++
				printProgress(printer, name, c, n)
			}
		}

		return nil
	})
}

// visitEachFile invokes the given visitor function on each file reachable from the given set of packages that
// also has an entry in the indexer's files map. This method prints the progress of the traversal to stdout
// asynchronously.
func (i *Indexer) visitEachFile(name string, animate bool, fn func(f FileInfo)) {
	_ = withTitle(name, animate, func(printer *pentimento.Printer) error {
		processed := map[string]bool{}

		c := 0
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

				fn(FileInfo{
					Package:  p,
					File:     f,
					Filename: filename,
					Document: d,
				})

				c++
				printProgress(printer, name, c, len(i.documents))
				processed[filename] = true
			}
		}

		return nil
	})
}
