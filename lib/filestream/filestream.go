package filestream

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fsutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/metrics"
)

var disableFadvise = flag.Bool("filestream.disableFadvise", false, "Whether to disable fadvise() syscall when reading large data files. "+
	"The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. "+
	"In some rare cases it is better to disable the syscall if it uses too much CPU")

const dontNeedBlockSize = 16 * 1024 * 1024

// ReadCloser is a standard interface for filestream Reader.
type ReadCloser interface {
	Path() string
	Read(p []byte) (int, error)
	MustClose()
}

// WriteCloser is a standard interface for filestream Writer.
type WriteCloser interface {
	Path() string
	Write(p []byte) (int, error)
	MustClose()
}

func getReadBufferSize() int {
	readBufferSizeOnce.Do(func() {
		n := memory.Allowed() / 1024 / 64
		if n < 4*1024 {
			n = 4 * 1024
		}
		if n > 64*1024 {
			n = 64 * 1024
		}
		readBufferSize = n
	})
	return readBufferSize
}

var (
	readBufferSize     int
	readBufferSizeOnce sync.Once
)

func getWriteBufferSize() int {
	writeBufferSizeOnce.Do(func() {
		n := memory.Allowed() / 1024 / 8
		if n < 4*1024 {
			n = 4 * 1024
		}
		if n > 128*1024 {
			n = 128 * 1024
		}
		writeBufferSize = n
	})
	return writeBufferSize
}

var (
	writeBufferSize     int
	writeBufferSizeOnce sync.Once
)

// Reader implements buffered file reader.
type Reader struct {
	f  *os.File
	br *bufio.Reader
	st streamTracker
}

// Path returns the path to r
func (r *Reader) Path() string {
	return r.f.Name()
}

// OpenReaderAt opens the file at the given path in nocache mode at the given offset.
//
// If nocache is set, then the reader doesn't pollute OS page cache.
func OpenReaderAt(path string, offset int64, nocache bool) (*Reader, error) {
	r := MustOpen(path, nocache)
	n, err := r.f.Seek(offset, io.SeekStart)
	if err != nil {
		r.MustClose()
		return nil, fmt.Errorf("cannot seek to offset=%d for %q: %w", offset, path, err)
	}
	if n != offset {
		r.MustClose()
		return nil, fmt.Errorf("invalid seek offset for %q; got %d; want %d", path, n, offset)
	}
	return r, nil
}

// MustOpen opens the file from the given path in nocache mode.
//
// If nocache is set, then the reader doesn't pollute OS page cache.
func MustOpen(path string, nocache bool) *Reader {
	f, err := os.Open(path)
	if err != nil {
		logger.Panicf("FATAL: cannot open file: %s", err)
	}
	r := &Reader{
		f:  f,
		br: getBufioReader(f),
	}
	if *disableFadvise {
		// Unconditionally disable fadvise() syscall
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5120 for details on why this is needed
		nocache = false
	}
	if nocache {
		r.st.fd = f.Fd()
	}
	readersCount.Inc()
	return r
}

// MustClose closes the underlying file passed to MustOpen.
func (r *Reader) MustClose() {
	if err := r.st.close(); err != nil {
		logger.Panicf("FATAL: cannot close streamTracker for file %q: %s", r.f.Name(), err)
	}
	if err := r.f.Close(); err != nil {
		logger.Panicf("FATAL: cannot close file %q: %s", r.f.Name(), err)
	}
	r.f = nil

	putBufioReader(r.br)
	r.br = nil

	readersCount.Dec()
}

var (
	readDuration      = metrics.NewFloatCounter(`vm_filestream_read_duration_seconds_total`)
	readCallsBuffered = metrics.NewCounter(`vm_filestream_buffered_read_calls_total`)
	readCallsReal     = metrics.NewCounter(`vm_filestream_real_read_calls_total`)
	readBytesBuffered = metrics.NewCounter(`vm_filestream_buffered_read_bytes_total`)
	readBytesReal     = metrics.NewCounter(`vm_filestream_real_read_bytes_total`)
	readersCount      = metrics.NewCounter(`vm_filestream_readers`)
)

// Read reads file contents to p.
func (r *Reader) Read(p []byte) (int, error) {
	readCallsBuffered.Inc()
	n, err := r.br.Read(p)
	readBytesBuffered.Add(n)
	if err != nil {
		return n, err
	}
	if err := r.st.adviseDontNeed(n, false); err != nil {
		return n, fmt.Errorf("advise error for %q: %w", r.f.Name(), err)
	}
	return n, nil
}

type statReader struct {
	*os.File
}

func (sr *statReader) Read(p []byte) (int, error) {
	startTime := time.Now()
	readCallsReal.Inc()
	n, err := sr.File.Read(p)
	d := time.Since(startTime).Seconds()
	readDuration.Add(d)
	readBytesReal.Add(n)
	return n, err
}

func getBufioReader(f *os.File) *bufio.Reader {
	sr := &statReader{f}
	v := brPool.Get()
	if v == nil {
		return bufio.NewReaderSize(sr, getReadBufferSize())
	}
	br := v.(*bufio.Reader)
	br.Reset(sr)
	return br
}

func putBufioReader(br *bufio.Reader) {
	brPool.Put(br)
}

var brPool sync.Pool

// Writer implements buffered file writer.
type Writer struct {
	f  *os.File
	bw *bufio.Writer
	st streamTracker
}

// Path returns the path to r
func (w *Writer) Path() string {
	return w.f.Name()
}

// OpenWriterAt opens the file at path in nocache mode for writing at the given offset.
//
// The file at path is created if it is missing.
//
// If nocache is set, the writer doesn't pollute OS page cache.
func OpenWriterAt(path string, offset int64, nocache bool) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	n, err := f.Seek(offset, io.SeekStart)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("cannot seek to offset=%d in %q: %w", offset, path, err)
	}
	if n != offset {
		_ = f.Close()
		return nil, fmt.Errorf("invalid seek offset for %q; got %d; want %d", path, n, offset)
	}
	return newWriter(f, nocache), nil
}

// MustCreate creates the file for the given path in nocache mode.
//
// If nocache is set, the writer doesn't pollute OS page cache.
func MustCreate(path string, nocache bool) *Writer {
	f, err := os.Create(path)
	if err != nil {
		logger.Panicf("FATAL: cannot create file %q: %s", path, err)
	}
	return newWriter(f, nocache)
}

func newWriter(f *os.File, nocache bool) *Writer {
	w := &Writer{
		f:  f,
		bw: getBufioWriter(f),
	}
	if nocache {
		w.st.fd = f.Fd()
	}
	writersCount.Inc()
	return w
}

// MustClose syncs the underlying file to storage and then closes it.
func (w *Writer) MustClose() {
	if err := w.bw.Flush(); err != nil {
		logger.Panicf("FATAL: cannot flush buffered data to file %q: %s", w.f.Name(), err)
	}
	putBufioWriter(w.bw)
	w.bw = nil

	if !fsutil.IsFsyncDisabled() {
		if err := w.f.Sync(); err != nil {
			logger.Panicf("FATAL: cannot sync file %q: %s", w.f.Name(), err)
		}
	}
	if err := w.st.close(); err != nil {
		logger.Panicf("FATAL: cannot close streamTracker for file %q: %s", w.f.Name(), err)
	}
	if err := w.f.Close(); err != nil {
		logger.Panicf("FATAL: cannot close file %q: %s", w.f.Name(), err)
	}
	w.f = nil

	writersCount.Dec()
}

var (
	writeDuration        = metrics.NewFloatCounter(`vm_filestream_write_duration_seconds_total`)
	writeCallsBuffered   = metrics.NewCounter(`vm_filestream_buffered_write_calls_total`)
	writeCallsReal       = metrics.NewCounter(`vm_filestream_real_write_calls_total`)
	writtenBytesBuffered = metrics.NewCounter(`vm_filestream_buffered_written_bytes_total`)
	writtenBytesReal     = metrics.NewCounter(`vm_filestream_real_written_bytes_total`)
	writersCount         = metrics.NewCounter(`vm_filestream_writers`)
)

// Write writes p to the underlying file.
func (w *Writer) Write(p []byte) (int, error) {
	writeCallsBuffered.Inc()
	n, err := w.bw.Write(p)
	writtenBytesBuffered.Add(n)
	if err != nil {
		return n, err
	}
	if err := w.st.adviseDontNeed(n, true); err != nil {
		return n, fmt.Errorf("advise error for %q: %w", w.f.Name(), err)
	}
	return n, nil
}

// MustFlush flushes all the buffered data to file.
//
// if isSync is true, then the flushed data is fsynced to the underlying storage.
func (w *Writer) MustFlush(isSync bool) {
	startTime := time.Now()
	defer func() {
		d := time.Since(startTime).Seconds()
		writeDuration.Add(d)
	}()
	if err := w.bw.Flush(); err != nil {
		logger.Panicf("FATAL: cannot flush buffered data to file %q: %s", w.f.Name(), err)
	}
	if isSync {
		if err := w.f.Sync(); err != nil {
			logger.Panicf("FATAL: cannot fsync data to the underlying storage for file %q: %s", w.f.Name(), err)
		}
	}
}

type statWriter struct {
	*os.File
}

func (sw *statWriter) Write(p []byte) (int, error) {
	startTime := time.Now()
	writeCallsReal.Inc()
	n, err := sw.File.Write(p)
	d := time.Since(startTime).Seconds()
	writeDuration.Add(d)
	writtenBytesReal.Add(n)
	return n, err
}

func getBufioWriter(f *os.File) *bufio.Writer {
	sw := &statWriter{f}
	v := bwPool.Get()
	if v == nil {
		return bufio.NewWriterSize(sw, getWriteBufferSize())
	}
	bw := v.(*bufio.Writer)
	bw.Reset(sw)
	return bw
}

func putBufioWriter(bw *bufio.Writer) {
	bwPool.Put(bw)
}

var bwPool sync.Pool

type streamTracker struct {
	fd     uintptr
	offset uint64 // nolint:unused
	length uint64 // nolint:unused
}
