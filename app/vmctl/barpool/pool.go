// Package barpool provides access to the global
// pool of progress bars, so they could be rendered
// altogether.
package barpool

import "github.com/cheggaaa/pb/v3"

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
	bar := pb.ProgressBarTemplate(format).New(total)
	Add(bar)
	return bar
}
