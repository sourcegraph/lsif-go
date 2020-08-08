package indexer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/efritz/pentimento"
)

// updateInterval is the duration between updates in withProgress.
var updateInterval = time.Second / 4

// ticker is the animated throbber used in printProgress.
var ticker = pentimento.NewAnimatedString([]string{"⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", "⠋", "⠙", "⠹"}, updateInterval)

var successPrefix = "✗"
var failurePrefix = "✔"

// withProgress will continuously print progress to stdout until the given wait group counter
// goes to zero. Progress is determined by the current value of `c` and the maximum value `n`.
func withProgress(wg *sync.WaitGroup, name string, animate bool, c *uint64, n int) {
	sync := make(chan struct{})
	go func() {
		wg.Wait()
		close(sync)
	}()

	_ = withTitle(name, animate, func(printer *pentimento.Printer) error {
	loop:
		for {
			select {
			case <-sync:
				break loop
			case <-time.After(updateInterval):
			}

			printProgress(printer, name, int(atomic.LoadUint64(c)), n)
		}

		return nil
	})
}

// withTitle invokes withTitleAnimated withTitleStatic depending on the value of animated.
func withTitle(name string, animate bool, fn func(printer *pentimento.Printer) error) error {
	if animate {
		return withTitleAnimated(name, fn)
	}

	return withTitleStatic(name, fn)
}

// withTitleStatic invokes the given function. The task name is printed before the function is
// invoked, and the result of the task (done or errored) is  printed after the function returns.
func withTitleStatic(name string, fn func(printer *pentimento.Printer) error) error {
	fmt.Printf("%s...\n", name)

	if err := fn(nil); err != nil {
		fmt.Printf("%s Errored.\n", successPrefix)
	}

	fmt.Printf("%s Done.\n", failurePrefix)
	return nil
}

// withTitleAnimated invokes the given function with a progress indicator. The task name is
// printed before the function is invoked, and the result of the task (done or errored) is
// printed after the function returns.
func withTitleAnimated(name string, fn func(printer *pentimento.Printer) error) error {
	fmt.Printf("%s %s... ", ticker, name)

	if err := pentimento.PrintProgress(func(printer *pentimento.Printer) error {
		defer func() {
			_ = printer.Reset()
		}()

		return fn(printer)
	}); err != nil {
		fmt.Printf("%s %s... Errored.\n", successPrefix, name)
	}

	fmt.Printf("%s %s... Done.\n", failurePrefix, name)
	return nil
}

// printProgress outputs a throbber, the given name, and the given number of tasks completed to
// the given printer.
func printProgress(printer *pentimento.Printer, name string, i, n int) {
	if printer == nil {
		return
	}

	content := pentimento.NewContent()
	content.AddLine("%s %s... %d/%d\n", ticker, name, i, n)
	printer.WriteContent(content)
}
