package fs

import (
	"flag"
	"fmt"
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"golang.org/x/sys/unix"
)

var disableMmap = flag.Bool("fs.disableMmap", false, "Whether to use pread() instead of mmap() for reading data files")

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
}

// MustReadAt reads len(p) bytes at off from r.
func (r *ReaderAt) MustReadAt(p []byte, off int64) {
	if len(p) == 0 {
		return
	}
	if len(r.mmapData) == 0 || len(p) > 8*1024 {
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
	} else {
		if off < 0 || off > int64(len(r.mmapData)-len(p)) {
			logger.Panicf("off=%d is out of allowed range [0...%d] for len(p)=%d", off, len(r.mmapData)-len(p), len(p))
		}
		copyMmap(p, r.mmapData[off:])
	}
	readCalls.Inc()
	readBytes.Add(len(p))
}

// MustClose closes r.
func (r *ReaderAt) MustClose() {
	fname := r.f.Name()
	if len(r.mmapData) > 0 {
		if err := unix.Munmap(r.mmapData); err != nil {
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
		return nil, fmt.Errorf("cannot open file %q for reader: %s", path, err)
	}
	var r ReaderAt
	r.f = f
	if !*disableMmap {
		data, err := mmapFile(f)
		if err != nil {
			MustClose(f)
			return nil, fmt.Errorf("cannot init reader for %q: %s", path, err)
		}
		r.mmapData = data
	}
	readersCount.Inc()
	return &r, nil
}

var (
	readCalls    = metrics.NewCounter(`vm_fs_read_calls_total`)
	readBytes    = metrics.NewCounter(`vm_fs_read_bytes_total`)
	readersCount = metrics.NewCounter(`vm_fs_readers`)
)

func mmapFile(f *os.File) ([]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("error in stat: %s", err)
	}
	size := fi.Size()
	if size == 0 {
		return nil, nil
	}
	if size < 0 {
		return nil, fmt.Errorf("got negative file size: %d bytes", size)
	}
	if int64(int(size)) != size {
		return nil, fmt.Errorf("file is too big to be mmap'ed: %d bytes", size)
	}
	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("cannot mmap file with size %d: %s", size, err)
	}
	return data, nil
}
