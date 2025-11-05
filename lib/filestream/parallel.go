package filestream

import (
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FileCreatorTask a task for creating the file at the given path and assigning it to *wc.
type FileCreatorTask struct {
	path    string
	wc      *WriteCloser
	nocache bool
}

// NewFileCreatorTask creates new task for creating the file at the given path an assigning it to *wc
func NewFileCreatorTask(path string, wc *WriteCloser, nocache bool) *FileCreatorTask {
	return &FileCreatorTask{
		path:    path,
		wc:      wc,
		nocache: nocache,
	}
}

// Run executes file creating task
func (t *FileCreatorTask) Run() {
	*t.wc = MustCreate(t.path, t.nocache)
}

// FileOpenerTask a task for opening the file at the given path and assigning it to *rc.
type FileOpenerTask struct {
	path    string
	rc      *ReadCloser
	nocache bool
}

// NewFileOpenerTask creates new task for opening the file at the given path an assigning it to *rc
func NewFileOpenerTask(path string, rc *ReadCloser, nocache bool) *FileOpenerTask {
	return &FileOpenerTask{
		path:    path,
		rc:      rc,
		nocache: nocache,
	}
}

// Run executes file opening task
func (t *FileOpenerTask) Run() {
	*t.rc = MustOpen(t.path, t.nocache)
}

// StreamWriterTask adds a task to execute in parallel - to write the data from src to the path.
type StreamWriterTask struct {
	path string
	src  io.WriterTo
}

// NewStreamWriterTask creates new task for writing the data from src to the path
func NewStreamWriterTask(path string, src io.WriterTo) *StreamWriterTask {
	return &StreamWriterTask{
		path: path,
		src:  src,
	}
}

func (t *StreamWriterTask) Run() {
	f := MustCreate(t.path, false)
	if _, err := t.src.WriteTo(f); err != nil {
		f.MustClose()
		// Do not call MustRemovePath(path), so the user could inspect
		// the file contents during investigation of the issue.
		logger.Panicf("FATAL: cannot write data to %q: %s", t.path, err)
	}
	f.MustClose()
}
