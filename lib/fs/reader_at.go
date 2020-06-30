package fs

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"golang.org/x/sys/unix"
)

var disableMmap = flag.Bool("fs.disableMmap", is32BitPtr, "Whether to use pread() instead of mmap() for reading data files. "+
	"By default mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot data files bigger than 2^32 bytes in memory")

const is32BitPtr = (^uintptr(0) >> 32) == 0

// MustReadAtCloser is rand-access read interface.
type MustReadAtCloser interface {
	// MustReadAt must read len(p) bytes from offset off to p.
	MustReadAt(p []byte, off int64)

	// MustClose must close the reader.
	MustClose()
}

// ReaderAt implements rand-access reader.
type ReaderAt struct {
	f        *os.File
	mmapData []byte

	// pageCacheBitmap holds a bitmap for recently touched pages in mmapData.
	// This bitmap allows using simple copy() instead of copyMmap() for reading recently touched pages,
	// which is up to 4x faster when reading small chunks of data via MustReadAt.
	pageCacheBitmap   atomic.Value
	pageCacheBitmapWG sync.WaitGroup

	stopCh chan struct{}
}

type pageCacheBitmap struct {
	m []uint64
}

// MustReadAt reads len(p) bytes at off from r.
func (r *ReaderAt) MustReadAt(p []byte, off int64) {
	if len(p) == 0 {
		return
	}
	if off < 0 {
		logger.Panicf("off=%d cannot be negative", off)
	}
	end := off + int64(len(p))
	if len(r.mmapData) == 0 || (len(p) > 8*1024 && !r.isInPageCache(off, end)) {
		// Read big blocks directly from file.
		// This could be faster than reading these blocks from mmap,
		// since it triggers less page faults.
		n, err := r.f.ReadAt(p, off)
		if err != nil {
			logger.Panicf("FATAL: cannot read %d bytes at offset %d of file %q: %s", len(p), off, r.f.Name(), err)
		}
		if n != len(p) {
			logger.Panicf("FATAL: unexpected number of bytes read; got %d; want %d", n, len(p))
		}
		if len(r.mmapData) > 0 {
			r.markInPageCache(off, end)
		}
	} else {
		if off > int64(len(r.mmapData)-len(p)) {
			logger.Panicf("off=%d is out of allowed range [0...%d] for len(p)=%d", off, len(r.mmapData)-len(p), len(p))
		}
		src := r.mmapData[off:]
		if r.isInPageCache(off, end) {
			// It is safe copying the data with copy(), since it is likely it is in the page cache.
			// This is up to 4x faster than copyMmap() below.
			copy(p, src)
		} else {
			// The data may be missing in the page cache, so it is better to copy it via cgo trick
			// in order to avoid P stalls in Go runtime.
			// See https://medium.com/@valyala/mmap-in-go-considered-harmful-d92a25cb161d for details.
			copyMmap(p, src)
			r.markInPageCache(off, end)
		}
	}
	readCalls.Inc()
	readBytes.Add(len(p))
}

func (r *ReaderAt) isInPageCache(start, end int64) bool {
	if int64(len(r.mmapData))-end < 4096 {
		// If standard copy(dst, src) from Go may read beyond len(src), then this should help
		// fixing SIGBUS panic from https://github.com/VictoriaMetrics/VictoriaMetrics/issues/581
		return false
	}
	startBit := uint64(start) / pageSize
	endBit := uint64(end) / pageSize
	m := r.pageCacheBitmap.Load().(*pageCacheBitmap).m
	for startBit <= endBit {
		idx := startBit / 64
		off := startBit % 64
		if idx >= uint64(len(m)) {
			return true
		}
		n := atomic.LoadUint64(&m[idx])
		if (n>>off)&1 != 1 {
			return false
		}
		startBit++
	}
	return true
}

func (r *ReaderAt) markInPageCache(start, end int64) {
	startBit := uint64(start) / pageSize
	endBit := uint64(end) / pageSize
	m := r.pageCacheBitmap.Load().(*pageCacheBitmap).m
	for startBit <= endBit {
		idx := startBit / 64
		off := startBit % 64
		n := atomic.LoadUint64(&m[idx])
		n |= 1 << off
		// It is OK if multiple concurrent goroutines store the same m[idx].
		atomic.StoreUint64(&m[idx], n)
		startBit++
	}
}

// Assume page size is 4KB
const pageSize = 4 * 1024

// MustClose closes r.
func (r *ReaderAt) MustClose() {
	close(r.stopCh)
	r.pageCacheBitmapWG.Wait()

	fname := r.f.Name()
	if len(r.mmapData) > 0 {
		if err := unix.Munmap(r.mmapData[:cap(r.mmapData)]); err != nil {
			logger.Panicf("FATAL: cannot unmap data for file %q: %s", fname, err)
		}
		r.mmapData = nil
	}
	MustClose(r.f)
	r.f = nil
	readersCount.Dec()
}

// OpenReaderAt opens ReaderAt for reading from filename.
//
// MustClose must be called on the returned ReaderAt when it is no longer needed.
func OpenReaderAt(path string) (*ReaderAt, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %q for reader: %w", path, err)
	}
	var r ReaderAt
	r.f = f
	r.stopCh = make(chan struct{})
	if !*disableMmap {
		fi, err := f.Stat()
		if err != nil {
			return nil, fmt.Errorf("error in stat: %w", err)
		}
		size := fi.Size()
		bm := &pageCacheBitmap{
			m: make([]uint64, 1+size/pageSize/64),
		}
		r.pageCacheBitmap.Store(bm)
		r.pageCacheBitmapWG.Add(1)
		go func() {
			defer r.pageCacheBitmapWG.Done()
			pageCacheBitmapCleaner(&r.pageCacheBitmap, r.stopCh)
		}()

		data, err := mmapFile(f, size)
		if err != nil {
			MustClose(f)
			return nil, fmt.Errorf("cannot init reader for %q: %w", path, err)
		}
		r.mmapData = data
	}
	readersCount.Inc()
	return &r, nil
}

func pageCacheBitmapCleaner(pcbm *atomic.Value, stopCh <-chan struct{}) {
	t := time.NewTimer(time.Minute)
	for {
		select {
		case <-stopCh:
			t.Stop()
			return
		case <-t.C:
		}
		bmOld := pcbm.Load().(*pageCacheBitmap)
		bm := &pageCacheBitmap{
			m: make([]uint64, len(bmOld.m)),
		}
		pcbm.Store(bm)
	}
}

var (
	readCalls    = metrics.NewCounter(`vm_fs_read_calls_total`)
	readBytes    = metrics.NewCounter(`vm_fs_read_bytes_total`)
	readersCount = metrics.NewCounter(`vm_fs_readers`)
)

func mmapFile(f *os.File, size int64) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}
	if size < 0 {
		return nil, fmt.Errorf("got negative file size: %d bytes", size)
	}
	if int64(int(size)) != size {
		return nil, fmt.Errorf("file is too big to be mmap'ed: %d bytes", size)
	}
	// Round size to multiple of 4KB pages as `man 2 mmap` recommends.
	// This may help preventing SIGBUS panic at https://github.com/VictoriaMetrics/VictoriaMetrics/issues/581
	// The SIGBUS could occur if standard copy(dst, src) function may read beyond src bounds.
	sizeOrig := size
	if size%4096 != 0 {
		size += 4096 - size%4096
	}
	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("cannot mmap file with size %d: %w", size, err)
	}
	return data[:sizeOrig], nil
}
