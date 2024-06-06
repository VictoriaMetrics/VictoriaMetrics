// Package barpool provides access to the global
// pool of progress bars, so they could be rendered
// altogether.
package barpool

import (
	"fmt"
	"io"
	"os"

	"github.com/cheggaaa/pb/v3"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/terminal"
)

var isDisabled bool

// Disable sets progress bar to be no-op if v==true
func Disable(v bool) {
	isDisabled = v
}

var pool = pb.NewPool()

// Bar is an interface for progress bar
type Bar interface {
	Add(value int)
	Increment()
	Start()
	Finish()
	NewProxyReader(r io.Reader) *pb.Reader
}

type progressBar struct {
	*pb.ProgressBar
}

func (pb *progressBar) Finish()       { pb.ProgressBar.Finish() }
func (pb *progressBar) Start()        { pb.ProgressBar.Start() }
func (pb *progressBar) Add(value int) { pb.ProgressBar.Add(value) }
func (pb *progressBar) Increment()    { pb.ProgressBar.Increment() }
func (pb *progressBar) NewProxyReader(r io.Reader) *pb.Reader {
	return pb.ProgressBar.NewProxyReader(r)
}

type progressBarNoOp struct{}

func (pbno *progressBarNoOp) Finish()                               {}
func (pbno *progressBarNoOp) Start()                                {}
func (pbno *progressBarNoOp) Add(int)                               {}
func (pbno *progressBarNoOp) Increment()                            {}
func (pbno *progressBarNoOp) NewProxyReader(_ io.Reader) *pb.Reader { return nil }

// Start starts the global pool
// Must be called after all progress bars were added
func Start() error {
	if isDisabled {
		return nil
	}
	return pool.Start()
}

// Stop stops the global pool
func Stop() {
	if isDisabled {
		return
	}
	_ = pool.Stop()
}

// AddWithTemplate adds bar with the given template
// to the global pool
func AddWithTemplate(format string, total int) Bar {
	if isDisabled {
		return &progressBarNoOp{}
	}
	tpl := getTemplate(format)
	bar := pb.ProgressBarTemplate(tpl).New(total)
	pool.Add(bar)
	return &progressBar{bar}
}

// NewSingleProgress returns progress bar with given template
func NewSingleProgress(format string, total int) Bar {
	if isDisabled {
		return &progressBarNoOp{}
	}
	tpl := getTemplate(format)
	return &progressBar{pb.ProgressBarTemplate(tpl).New(total)}
}

func getTemplate(format string) string {
	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
	if !isTerminal {
		format = fmt.Sprintf("%s\n", format)
	}
	return format
}
