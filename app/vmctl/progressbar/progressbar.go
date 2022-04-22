package progressbar

import "github.com/cheggaaa/pb/v3"

var pool = pb.NewPool()

func Add(bar *pb.ProgressBar) { pool.Add(bar) }
func Start() error            { return pool.Start() }
func Stop() error             { return pool.Stop() }
func AddWithTemplate(tmpl pb.ProgressBarTemplate, total int) *pb.ProgressBar {
	bar := tmpl.New(total)
	pool.Add(bar)
	return bar
}
