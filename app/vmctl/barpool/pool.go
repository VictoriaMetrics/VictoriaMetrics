// Package barpool provides access to the global
// pool of progress bars, so they could be rendered
// altogether.
package barpool

import (
	"fmt"
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/terminal"
	"github.com/cheggaaa/pb/v3"
)

var pool = pb.NewPool()

// Add adds bar to the global pool
func Add(bar *pb.ProgressBar) { pool.Add(bar) }

// Start starts the global pool
// Must be called after all progress bars were added
func Start() error { return pool.Start() }

// Stop stops the global pool
func Stop() { _ = pool.Stop() }

// AddWithTemplate adds bar with the given template
// to the global pool
func AddWithTemplate(format string, total int) *pb.ProgressBar {
	tpl := getTemplate(format)
	bar := pb.ProgressBarTemplate(tpl).New(total)
	Add(bar)
	return bar
}

// NewSingleProgress returns progress bar with given template
func NewSingleProgress(format string, total int) *pb.ProgressBar {
	tpl := getTemplate(format)
	return pb.ProgressBarTemplate(tpl).New(total)
}

func getTemplate(format string) string {
	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
	if !isTerminal {
		format = fmt.Sprintf("%s\n", format)
	}
	return format
}
