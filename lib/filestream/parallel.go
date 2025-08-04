package filestream

import (
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// ParallelFileCreator is used for parallel creating of files for the given dstPath.
//
// ParallelFileCreator is needed for speeding up creating many files on high-latency
// storage systems such as NFS or Ceph.
type ParallelFileCreator struct {
	tasks []parallelFileCreatorTask
}

type parallelFileCreatorTask struct {
	dstPath string
	wc      *WriteCloser
	nocache bool
}

// Add registers a task for creating the file at dstPath and assigning it to *wc.
//
// Tasks are executed in parallel on Run() call.
func (pfc *ParallelFileCreator) Add(dstPath string, wc *WriteCloser, nocache bool) {
	pfc.tasks = append(pfc.tasks, parallelFileCreatorTask{
		dstPath: dstPath,
		wc:      wc,
		nocache: nocache,
	})
}

// Run runs all the registered tasks for creating files in parallel.
func (pfc *ParallelFileCreator) Run() {
	var wg sync.WaitGroup
	for _, task := range pfc.tasks {
		concurrencyCh <- struct{}{}
		wg.Add(1)

		go func(dstPath string, wc *WriteCloser, nocache bool) {
			defer func() {
				wg.Done()
				<-concurrencyCh
			}()

			*wc = MustCreate(dstPath, nocache)
		}(task.dstPath, task.wc, task.nocache)
	}
	wg.Wait()
}

// ParallelFileOpener is used for parallel opening of files at the given dstPath.
//
// ParallelFileOpener is needed for speeding up opening many files on high-latency
// storage systems such as NFS or Ceph.
type ParallelFileOpener struct {
	tasks []parallelFileOpenerTask
}

type parallelFileOpenerTask struct {
	path    string
	rc      *ReadCloser
	nocache bool
}

// Add registers a task for opening the file ath the given path and assigning it to *rc.
//
// Tasks are executed in parallel on Run() call.
func (pfo *ParallelFileOpener) Add(path string, rc *ReadCloser, nocache bool) {
	pfo.tasks = append(pfo.tasks, parallelFileOpenerTask{
		path:    path,
		rc:      rc,
		nocache: nocache,
	})
}

// Run runs all the registered tasks for opening files in parallel.
func (pfo *ParallelFileOpener) Run() {
	var wg sync.WaitGroup
	for _, task := range pfo.tasks {
		concurrencyCh <- struct{}{}
		wg.Add(1)

		go func(path string, rc *ReadCloser, nocache bool) {
			defer func() {
				wg.Done()
				<-concurrencyCh
			}()

			*rc = MustOpen(path, nocache)
		}(task.path, task.rc, task.nocache)
	}
	wg.Wait()
}

// ParallelStreamWriter is used for parallel writing of data from io.WriterTo to the given dstPath files.
//
// ParallelStreamWriter is needed for speeding up writing data to many files on high-latency
// storage systems such as NFS or Ceph.
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

// concurrencyCh limits the concurrency of parallel operations performed by ParallelFileCreator, ParallelFileOpener and ParallelStreamWriter
var concurrencyCh = make(chan struct{}, 256)
