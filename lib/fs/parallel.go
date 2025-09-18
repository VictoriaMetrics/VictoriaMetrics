package fs

import (
	"sync"
)

// ParallelReaderAtOpener opens ReaderAt files in parallel.
//
// ParallelReaderAtOpener speeds up opening multiple ReaderAt files on high-latency
// storage systems such as NFS or Ceph.
type ParallelReaderAtOpener struct {
	tasks []parallelReaderAtOpenerTask
}

type parallelReaderAtOpenerTask struct {
	path     string
	rc       *MustReadAtCloser
	fileSize *uint64
}

// Add adds a task for opening the file at the given path and storing it to *r, while storing the file size into *fileSize.
//
// Call Run() for running all the registered tasks in parallel.
func (pro *ParallelReaderAtOpener) Add(path string, rc *MustReadAtCloser, fileSize *uint64) {
	pro.tasks = append(pro.tasks, parallelReaderAtOpenerTask{
		path:     path,
		rc:       rc,
		fileSize: fileSize,
	})
}

// Run executes all the registered tasks in parallel.
func (pro *ParallelReaderAtOpener) Run() {
	var wg sync.WaitGroup
	for _, task := range pro.tasks {
		concurrencyCh <- struct{}{}
		wg.Add(1)

		go func(path string, rc *MustReadAtCloser, fileSize *uint64) {
			defer func() {
				wg.Done()
				<-concurrencyCh
			}()

			*rc = MustOpenReaderAt(path)
			*fileSize = MustFileSize(path)
		}(task.path, task.rc, task.fileSize)
	}
	wg.Wait()
}

// MustCloser must implement MustClose() function.
type MustCloser interface {
	MustClose()
}

// MustCloseParallel closes all the cs in parallel.
//
// Parallel closing reduces the time needed to flush the data to the underlying files on close
// on high-latency storage systems such as NFS or Ceph.
func MustCloseParallel(cs []MustCloser) {
	var wg sync.WaitGroup
	for _, c := range cs {
		concurrencyCh <- struct{}{}
		wg.Add(1)
		go func(c MustCloser) {
			defer func() {
				wg.Done()
				<-concurrencyCh
			}()
			c.MustClose()
		}(c)
	}
	wg.Wait()
}

// concurrencyCh limits the concurrency of parallel operations performed by ParallelReaderAtOpener and MustCloseParallel
var concurrencyCh = make(chan struct{}, 256)
