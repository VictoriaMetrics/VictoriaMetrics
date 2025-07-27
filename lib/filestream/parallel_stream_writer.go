package filestream

import (
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// ParallelStreamWriter is used for parallel writing of data from io.WriterTo to the given dstPath files.
type ParallelStreamWriter struct {
	tasks []parallelStreamWriterTask
}

type parallelStreamWriterTask struct {
	dstPath string
	src     io.WriterTo
}

// Add adds a task to execute in parallel - to write the data from src to the dstPath.
//
// Tasks are executed in parallel on Run() call.
func (psw *ParallelStreamWriter) Add(dstPath string, src io.WriterTo) {
	psw.tasks = append(psw.tasks, parallelStreamWriterTask{
		dstPath: dstPath,
		src:     src,
	})
}

// Run executes all the tasks added via Add() call in parallel.
func (psw *ParallelStreamWriter) Run() {
	var wg sync.WaitGroup
	concurrencyCh := make(chan struct{}, min(32, len(psw.tasks)))
	for _, task := range psw.tasks {
		concurrencyCh <- struct{}{}
		wg.Add(1)

		go func(dstPath string, src io.WriterTo) {
			defer func() {
				wg.Done()
				<-concurrencyCh
			}()

			f := MustCreate(dstPath, false)
			if _, err := src.WriteTo(f); err != nil {
				f.MustClose()
				// Do not call MustRemovePath(path), so the user could inspect
				// the file contents during investigation of the issue.
				logger.Panicf("FATAL: cannot write data to %q: %s", dstPath, err)
			}
			f.MustClose()
		}(task.dstPath, task.src)
	}
	wg.Wait()
}
