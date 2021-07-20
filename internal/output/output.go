package output

import (
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/efritz/pentimento"
	"github.com/sourcegraph/lsif-go/internal/parallel"
	"github.com/sourcegraph/lsif-go/internal/util"
)

type Options struct {
	Verbosity      Verbosity
	ShowAnimations bool
}

type Verbosity int

const (
	NoOutput Verbosity = iota
	DefaultOutput
	VerboseOutput
	VeryVerboseOutput
	VeryVeryVerboseOutput
)

// updateInterval is the duration between updates in withProgress.
var updateInterval = time.Second / 4

// ticker is the animated throbber used in printProgress.
var ticker = pentimento.NewAnimatedString([]string{
	"⠸", "⠼",
	"⠴", "⠦",
	"⠧", "⠇",
	"⠏", "⠋",
	"⠙", "⠹",
}, updateInterval)

var successPrefix = "✔"
var failurePrefix = "✗"

// logger is used to log at the level -vv and above from multiple goroutines.
var logger = log.New(os.Stdout, "", 0)

// WithProgress prints a spinner while the given function is active.
func WithProgress(name string, fn func(), outputOptions Options) {
	ch := make(chan func(), 1)
	ch <- fn
	close(ch)

	wg, count := parallel.Run(ch)
	WithProgressParallel(wg, name, outputOptions, count, 1)
}

// WithProgressParallel will continuously print progress to stdout until the given wait group
// counter goes to zero. Progress is determined by the values of `c` (number of tasks completed)
// and the value `n` (total number of tasks).
func WithProgressParallel(wg *sync.WaitGroup, name string, outputOptions Options, c *uint64, n uint64) {
	sync := make(chan struct{})
	go func() {
		wg.Wait()
		close(sync)
	}()

	withTitle(name, outputOptions, func(printer *pentimento.Printer) {
		for {
			select {
			case <-sync:
				return
			case <-time.After(updateInterval):
			}

			printProgress(printer, name, c, n)
		}
	})
}

// withTitle invokes withTitleAnimated withTitleStatic depending on the value of animated.
func withTitle(name string, outputOptions Options, fn func(printer *pentimento.Printer)) {
	if outputOptions.Verbosity == NoOutput {
		fn(nil)
	} else if !outputOptions.ShowAnimations || outputOptions.Verbosity >= VeryVerboseOutput {
		withTitleStatic(name, outputOptions.Verbosity, fn)
	} else {
		withTitleAnimated(name, outputOptions.Verbosity, fn)
	}
}

// withTitleStatic invokes the given function with non-animated output.
func withTitleStatic(name string, verbosity Verbosity, fn func(printer *pentimento.Printer)) {
	start := time.Now()
	fmt.Printf("%s\n", name)
	fn(nil)

	if verbosity > DefaultOutput {
		fmt.Printf("Finished in %s.\n\n", util.HumanElapsed(start))
	}
}

// withTitleStatic invokes the given function with animated output.
func withTitleAnimated(name string, verbosity Verbosity, fn func(printer *pentimento.Printer)) {
	start := time.Now()
	fmt.Printf("%s %s... ", ticker, name)

	_ = pentimento.PrintProgress(func(printer *pentimento.Printer) error {
		defer func() {
			_ = printer.Reset()
		}()

		fn(printer)
		return nil
	})

	if verbosity > DefaultOutput {
		fmt.Printf("%s %s... Done (%s)\n", successPrefix, name, util.HumanElapsed(start))
	} else {
		fmt.Printf("%s %s... Done\n", successPrefix, name)
	}
}

// printProgress outputs a throbber, the given name, and the given number of tasks completed to
// the given printer.
func printProgress(printer *pentimento.Printer, name string, c *uint64, n uint64) {
	if printer == nil {
		return
	}

	content := pentimento.NewContent()

	if c == nil {
		content.AddLine("%s %s...", ticker, name)
	} else {
		content.AddLine("%s %s... %d/%d\n", ticker, name, atomic.LoadUint64(c), n)
	}

	printer.WriteContent(content)
}
