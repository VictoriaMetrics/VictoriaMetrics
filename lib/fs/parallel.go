package fs

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fsutil"
)

// ParallelReaderAtOpener opens ReaderAt files in parallel.
//
// ParallelReaderAtOpener speeds up opening multiple ReaderAt files on high-latency
// storage systems such as NFS or Ceph.
type ParallelReaderAtOpener struct {
	tasks []parallelReaderAtOpenerTask
}

type parallelReaderAtOpenerTask struct {
	path              string
	rc                *MustReadAtCloser
	fileSize          *uint64
	useRandomReadHint bool
}

// Add adds a task for opening the file at the given path and storing it to *r, while storing the file size into *fileSize.
//
// Call Run() for running all the registered tasks in parallel.
func (pro *ParallelReaderAtOpener) Add(path string, rc *MustReadAtCloser, fileSize *uint64) {
	pro.add(path, rc, fileSize, false)
}

// AddRandomAccess adds a task for opening the file at the given path for reading at random offsets
// and storing it to *r, while storing the file size into *fileSize.
//
// The OS is hinted to skip readahead for reads performed through the opened file descriptor.
//
// Call Run() for running all the registered tasks in parallel.
func (pro *ParallelReaderAtOpener) AddRandomAccess(path string, rc *MustReadAtCloser, fileSize *uint64) {
	pro.add(path, rc, fileSize, true)
}

func (pro *ParallelReaderAtOpener) add(path string, rc *MustReadAtCloser, fileSize *uint64, useRandomReadHint bool) {
	pro.tasks = append(pro.tasks, parallelReaderAtOpenerTask{
		path:              path,
		rc:                rc,
		fileSize:          fileSize,
		useRandomReadHint: useRandomReadHint,
	})
}

// Run executes all the registered tasks in parallel.
func (pro *ParallelReaderAtOpener) Run() {
	var wg sync.WaitGroup
	concurrencyCh := fsutil.GetConcurrencyCh()
	for _, task := range pro.tasks {
		concurrencyCh <- struct{}{}

		wg.Go(func() {
			if task.useRandomReadHint {
				*task.rc = MustOpenReaderAtRandomAccess(task.path)
			} else {
				*task.rc = MustOpenReaderAt(task.path)
			}
			*task.fileSize = MustFileSize(task.path)

			<-concurrencyCh
		})
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
	concurrencyCh := fsutil.GetConcurrencyCh()
	for _, c := range cs {
		concurrencyCh <- struct{}{}
		wg.Go(func() {
			c.MustClose()
			<-concurrencyCh
		})
	}
	wg.Wait()
}
