package fs

// ReaderAtOpenerTask task to open ReaderAt files in parallel.
type ReaderAtOpenerTask struct {
	path     string
	rc       *MustReadAtCloser
	fileSize *uint64
}

// NewReaderAtOpenerTask creates new task for writing the data from src to the path
//
// ParallelReaderAtOpener speeds up opening multiple ReaderAt files on high-latency
// storage systems such as NFS or Ceph.
func NewReaderAtOpenerTask(path string, rc *MustReadAtCloser, fileSize *uint64) *ReaderAtOpenerTask {
	return &ReaderAtOpenerTask{
		path:     path,
		rc:       rc,
		fileSize: fileSize,
	}
}

func (t *ReaderAtOpenerTask) Run() {
	*t.rc = OpenReaderAt(t.path)
	*t.fileSize = MustFileSize(t.path)
}

// MustCloser must implement MustClose() function.
type MustCloser interface {
	MustClose()
}

// CloserTask task to close all the MustCloser in parallel.
//
// Parallel closing reduces the time needed to flush the data to the underlying files on close
// on high-latency storage systems such as NFS or Ceph.
type CloserTask struct {
	c MustCloser
}

// NewCloserTask creates new task for writing the data from src to the path
//
// NewCloserTask speeds up opening multiple MustCloser files on high-latency
// storage systems such as NFS or Ceph.
func NewCloserTask(c MustCloser) *CloserTask {
	return &CloserTask{
		c: c,
	}
}

func (t *CloserTask) Run() {
	t.c.MustClose()
}
