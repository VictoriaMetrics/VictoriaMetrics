package fs

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var disableMmap = flag.Bool("fs.disableMmap", is32BitPtr, "Whether to use pread() instead of mmap() for reading data files. "+
	"By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. "+
	"mmap() is usually faster for reading small data chunks than pread()")

// Disable mmap for architectures with 32-bit pointers in order to be able to work with files exceeding 2^32 bytes.
const is32BitPtr = (^uintptr(0) >> 32) == 0

// MustReadAtCloser is rand-access read interface.
type MustReadAtCloser interface {
	// Path must return path for the reader (e.g. file path, url or in-memory reference)
	Path() string

	// MustReadAt must read len(p) bytes from offset off to p.
	MustReadAt(p []byte, off int64)

	// MustClose must close the reader.
	MustClose()
}

// ReaderAt implements rand-access reader.
type ReaderAt struct {
	readCalls atomic.Int64
	readBytes atomic.Int64

	// path contains the path to the file for reading
	path string

	// mr is used for lazy opening of the file at path on the first access.
	mr     atomic.Pointer[mmapReader]
	mrLock sync.Mutex

	useLocalStats bool
}

// Path returns path to r.
func (r *ReaderAt) Path() string {
	return r.path
}

// MustReadAt reads len(p) bytes at off from r.
func (r *ReaderAt) MustReadAt(p []byte, off int64) {
	if len(p) == 0 {
		return
	}
	if off < 0 {
		logger.Panicf("BUG: off=%d cannot be negative", off)
	}

	// Lazily open the file at r.path on the first access
	mr := r.getMmapReader()

	// Read len(p) bytes at offset off to p.
	if len(mr.mmapData) == 0 {
		mr.mustReadAtViaSyscall(p, off)
	} else {
		if off > int64(len(mr.mmapData)-len(p)) {
			logger.Panicf("BUG: off=%d is out of allowed range [0...%d] for len(p)=%d in file %q", off, len(mr.mmapData)-len(p), len(p), r.path)
		}
		if mr.canFastReadViaMmap(off, len(p)) {
			src := mr.mmapData[off:]
			copy(p, src)
		} else {
			// Fall back for reading the data via syscall in order to avoid thread stalls
			// descirbed at https://valyala.medium.com/mmap-in-go-considered-harmful-d92a25cb161d
			mr.mustReadAtViaSyscall(p, off)
		}
	}
	if r.useLocalStats {
		r.readCalls.Add(1)
		r.readBytes.Add(int64(len(p)))
	} else {
		readCalls.Inc()
		readBytes.Add(len(p))
	}
}

func (r *ReaderAt) getMmapReader() *mmapReader {
	mr := r.mr.Load()
	if mr != nil {
		return mr
	}
	r.mrLock.Lock()
	mr = r.mr.Load()
	if mr == nil {
		mr = newMmapReaderFromPath(r.path)
		r.mr.Store(mr)
	}
	r.mrLock.Unlock()
	return mr
}

var (
	readCalls    = metrics.NewCounter(`vm_fs_read_calls_total`)
	readBytes    = metrics.NewCounter(`vm_fs_read_bytes_total`)
	readersCount = metrics.NewCounter(`vm_fs_readers`)
)

// MustClose closes r.
func (r *ReaderAt) MustClose() {
	mr := r.mr.Load()
	if mr != nil {
		mr.mustClose()
		r.mr.Store(nil)
	}

	if r.useLocalStats {
		readCalls.AddInt64(r.readCalls.Load())
		readBytes.AddInt64(r.readBytes.Load())
		r.readCalls.Store(0)
		r.readBytes.Store(0)
		r.useLocalStats = false
	}
}

// SetUseLocalStats switches to local stats collection instead of global stats collection.
//
// This function must be called before the first call to MustReadAt().
//
// Collecting local stats may improve performance on systems with big number of CPU cores,
// since the locally collected stats is pushed to global stats only at MustClose() call
// instead of pushing it at every MustReadAt call.
func (r *ReaderAt) SetUseLocalStats() {
	r.useLocalStats = true
}

// MustFadviseSequentialRead hints the OS that f is read mostly sequentially.
//
// if prefetch is set, then the OS is hinted to prefetch f data.
func (r *ReaderAt) MustFadviseSequentialRead(prefetch bool) {
	mr := r.getMmapReader()
	if err := fadviseSequentialRead(mr.f, prefetch); err != nil {
		logger.Panicf("FATAL: error in fadviseSequentialRead(%q, %v): %s", r.path, prefetch, err)
	}
}

// MustOpenReaderAt opens ReaderAt for reading from the file located at path.
//
// MustClose must be called on the returned ReaderAt when it is no longer needed.
func MustOpenReaderAt(path string) *ReaderAt {
	var r ReaderAt
	r.path = path
	return &r
}

// NewReaderAt returns ReaderAt for reading from f.
//
// NewReaderAt takes ownership for f, so it shouldn't be closed by the caller.
//
// MustClose must be called on the returned ReaderAt when it is no longer needed.
func NewReaderAt(f *os.File) *ReaderAt {
	mr := newMmapReaderFromFile(f)
	var r ReaderAt
	r.path = f.Name()
	r.mr.Store(mr)
	return &r
}

type mmapReader struct {
	f        *os.File
	mmapData []byte

	mincoreBits                 []atomic.Uint64
	mincoreNextCleanupTimestamp atomic.Uint64
}

func newMmapReaderFromPath(path string) *mmapReader {
	f, err := os.Open(path)
	if err != nil {
		logger.Panicf("FATAL: cannot open file for reading: %s; try increasing the limit on the number of open files via 'ulimit -n'", err)
	}
	return newMmapReaderFromFile(f)
}

func newMmapReaderFromFile(f *os.File) *mmapReader {
	var mmapData []byte
	var mincoreBits []atomic.Uint64

	if !*disableMmap {
		fi, err := f.Stat()
		if err != nil {
			path := f.Name()
			MustClose(f)
			logger.Panicf("FATAL: error in fstat(%q): %s", path, err)
		}
		size := fi.Size()
		data, err := mmapFile(f, size)
		if err != nil {
			path := f.Name()
			MustClose(f)
			logger.Panicf("FATAL: cannot mmap %q: %s", path, err)
		}
		mmapData = data

		if hasMincore() {
			mincoreBitsSize := (((uint64(size) + pageSizeBytes - 1) / pageSizeBytes) + 63) / 64
			mincoreBits = make([]atomic.Uint64, mincoreBitsSize)
		}
	}

	readersCount.Inc()
	mr := &mmapReader{
		f:        f,
		mmapData: mmapData,

		mincoreBits: mincoreBits,
	}
	mr.mincoreNextCleanupTimestamp.Store(fasttime.UnixTimestamp() + 60)

	return mr
}

func (mr *mmapReader) mustClose() {
	fname := mr.f.Name()
	if len(mr.mmapData) > 0 {
		if err := mUnmap(mr.mmapData[:cap(mr.mmapData)]); err != nil {
			logger.Panicf("FATAL: cannot unmap data for file %q: %s", fname, err)
		}
		mr.mmapData = nil
		mmappedFiles.Dec()
	}
	MustClose(mr.f)
	mr.f = nil

	readersCount.Dec()
}

func (mr *mmapReader) mustReadAtViaSyscall(p []byte, off int64) {
	n, err := mr.f.ReadAt(p, off)
	if err != nil {
		logger.Panicf("FATAL: cannot read %d bytes at offset %d of file %q: %s", len(p), off, mr.f.Name(), err)
	}
	if n != len(p) {
		logger.Panicf("FATAL: unexpected number of bytes read from file %q; got %d; want %d", mr.f.Name(), n, len(p))
	}

	if len(mr.mmapData) == 0 || !hasMincore() {
		return
	}

	// Mark the just read data as available for fast read via mmap
	mincoreBits := mr.mincoreBits
	pageSize := pageSizeBytes

	end := off + int64(n)
	off -= int64(uint64(off) % pageSize)
	pageIdx := uint64(off) / pageSize
	for off < end {
		wordIdx := pageIdx / 64
		bitIdx := pageIdx % 64
		mask := uint64(1) << bitIdx
		wordPtr := &mincoreBits[wordIdx]
		word := wordPtr.Load()
		for (word&mask) == 0 && !wordPtr.CompareAndSwap(word, word|mask) {
			word = wordPtr.Load()
		}

		off += int64(pageSize)
		pageIdx++
	}
}

func (mr *mmapReader) canFastReadViaMmap(off int64, n int) bool {
	if !hasMincore() {
		return true
	}

	mincoreBits := mr.mincoreBits
	pageSize := pageSizeBytes

	ct := fasttime.UnixTimestamp()
	nextCleanup := mr.mincoreNextCleanupTimestamp.Load()
	if ct > nextCleanup && mr.mincoreNextCleanupTimestamp.CompareAndSwap(nextCleanup, ct+60) {
		for i := range mincoreBits {
			mincoreBits[i].Store(0)
		}
	}

	end := off + int64(n)
	off -= int64(uint64(off) % pageSize)
	pageIdx := uint64(off) / pageSize
	for off < end {
		wordIdx := pageIdx / 64
		bitIdx := pageIdx % 64
		mask := uint64(1) << bitIdx
		wordPtr := &mincoreBits[wordIdx]
		word := wordPtr.Load()
		if (word & mask) == 0 {
			if !mincore(&mr.mmapData[off]) {
				return false
			}
			for (word&mask) == 0 && !wordPtr.CompareAndSwap(word, word|mask) {
				word = wordPtr.Load()
			}
		}

		off += int64(pageSize)
		pageIdx++
	}

	return true
}

var pageSizeBytes = uint64(os.Getpagesize())

func mmapFile(f *os.File, size int64) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}
	if size < 0 {
		return nil, fmt.Errorf("got negative file size: %d bytes", size)
	}
	if int64(int(size)) != size {
		return nil, fmt.Errorf("file is too big to be memory mapped: %d bytes", size)
	}
	// Round size to multiple of 4KB pages as `man 2 mmap` recommends.
	// This may help preventing SIGBUS panic at https://github.com/VictoriaMetrics/VictoriaMetrics/issues/581
	// The SIGBUS could occur if standard copy(dst, src) function may read beyond src bounds.
	sizeOrig := size
	if size%4096 != 0 {
		size += 4096 - size%4096
	}
	data, err := mmap(int(f.Fd()), int(size))
	if err != nil {
		return nil, fmt.Errorf("cannot mmap file with size %d bytes; already memory mapped files: %d: %w; "+
			"try increasing /proc/sys/vm/max_map_count or passing -fs.disableMmap command-line flag to the application", size, mmappedFiles.Get(), err)
	}
	mmappedFiles.Inc()
	return data[:sizeOrig], nil
}

var mmappedFiles = metrics.NewCounter("vm_mmapped_files")
