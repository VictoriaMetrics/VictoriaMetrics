package netstorage

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

// InitTmpBlocksDir initializes directory to store temporary search results.
//
// It stores data in system-defined temporary directory if tmpDirPath is empty.
func InitTmpBlocksDir(tmpDirPath string) {
	if len(tmpDirPath) == 0 {
		tmpDirPath = os.TempDir()
	}
	tmpBlocksDir = tmpDirPath + "/searchResults"
	fs.MustRemoveAll(tmpBlocksDir)
	if err := fs.MkdirAllIfNotExist(tmpBlocksDir); err != nil {
		logger.Panicf("FATAL: cannot create %q: %s", tmpBlocksDir, err)
	}
}

var tmpBlocksDir string

const maxInmemoryTmpBlocksFile = 512 * 1024

type tmpBlocksFile struct {
	buf []byte

	f  *os.File
	bw *bufio.Writer

	offset uint64
}

func getTmpBlocksFile() *tmpBlocksFile {
	v := tmpBlocksFilePool.Get()
	if v == nil {
		return &tmpBlocksFile{}
	}
	return v.(*tmpBlocksFile)
}

func putTmpBlocksFile(tbf *tmpBlocksFile) {
	tbf.MustClose()
	tbf.buf = tbf.buf[:0]
	tbf.f = nil
	tbf.bw = nil
	tbf.offset = 0
	tmpBlocksFilePool.Put(tbf)
}

var tmpBlocksFilePool sync.Pool

type tmpBlockAddr struct {
	offset uint64
	size   int
}

func (addr tmpBlockAddr) String() string {
	return fmt.Sprintf("offset %d, size %d", addr.offset, addr.size)
}

func getBufioWriter(f *os.File) *bufio.Writer {
	v := bufioWriterPool.Get()
	if v == nil {
		return bufio.NewWriterSize(f, maxInmemoryTmpBlocksFile*2)
	}
	bw := v.(*bufio.Writer)
	bw.Reset(f)
	return bw
}

func putBufioWriter(bw *bufio.Writer) {
	bufioWriterPool.Put(bw)
}

var bufioWriterPool sync.Pool

var tmpBlocksFilesCreated = metrics.NewCounter(`vm_tmp_blocks_files_created_total`)

// WriteBlock writes b to tbf.
//
// It returns errors since the operation may fail on space shortage
// and this must be handled.
func (tbf *tmpBlocksFile) WriteBlock(b *storage.Block) (tmpBlockAddr, error) {
	var addr tmpBlockAddr
	addr.offset = tbf.offset

	tbfBufLen := len(tbf.buf)
	tbf.buf = storage.MarshalBlock(tbf.buf, b)
	addr.size = len(tbf.buf) - tbfBufLen
	tbf.offset += uint64(addr.size)
	if tbf.offset <= maxInmemoryTmpBlocksFile {
		return addr, nil
	}

	if tbf.f == nil {
		f, err := ioutil.TempFile(tmpBlocksDir, "")
		if err != nil {
			return addr, err
		}
		tbf.f = f
		tbf.bw = getBufioWriter(f)
		tmpBlocksFilesCreated.Inc()
	}
	_, err := tbf.bw.Write(tbf.buf)
	tbf.buf = tbf.buf[:0]
	if err != nil {
		return addr, fmt.Errorf("cannot write block to %q: %s", tbf.f.Name(), err)
	}
	return addr, nil
}

func (tbf *tmpBlocksFile) Finalize() error {
	if tbf.f == nil {
		return nil
	}

	err := tbf.bw.Flush()
	putBufioWriter(tbf.bw)
	tbf.bw = nil
	if _, err := tbf.f.Seek(0, 0); err != nil {
		logger.Panicf("FATAL: cannot seek to the start of file: %s", err)
	}
	mustFadviseRandomRead(tbf.f)
	return err
}

func (tbf *tmpBlocksFile) MustReadBlockAt(dst *storage.Block, addr tmpBlockAddr) {
	var buf []byte
	if tbf.f == nil {
		buf = tbf.buf[addr.offset : addr.offset+uint64(addr.size)]
	} else {
		bb := tmpBufPool.Get()
		defer tmpBufPool.Put(bb)
		bb.B = bytesutil.Resize(bb.B, addr.size)
		n, err := tbf.f.ReadAt(bb.B, int64(addr.offset))
		if err != nil {
			logger.Panicf("FATAL: cannot read from %q at %s: %s", tbf.f.Name(), addr, err)
		}
		if n != len(bb.B) {
			logger.Panicf("FATAL: too short number of bytes read at %s; got %d; want %d", addr, n, len(bb.B))
		}
		buf = bb.B
	}
	tail, err := storage.UnmarshalBlock(dst, buf)
	if err != nil {
		logger.Panicf("FATAL: cannot unmarshal data at %s: %s", addr, err)
	}
	if len(tail) > 0 {
		logger.Panicf("FATAL: unexpected non-empty tail left after unmarshaling data at %s; len(tail)=%d", addr, len(tail))
	}
}

var tmpBufPool bytesutil.ByteBufferPool

func (tbf *tmpBlocksFile) MustClose() {
	if tbf.f == nil {
		return
	}
	if tbf.bw != nil {
		putBufioWriter(tbf.bw)
		tbf.bw = nil
	}
	fname := tbf.f.Name()

	// Remove the file at first, then close it.
	// This way the OS shouldn't try to flush file contents to storage
	// on close.
	if err := os.Remove(fname); err != nil {
		logger.Panicf("FATAL: cannot remove %q: %s", fname, err)
	}
	if err := tbf.f.Close(); err != nil {
		logger.Panicf("FATAL: cannot close %q: %s", fname, err)
	}
	tbf.f = nil
}
