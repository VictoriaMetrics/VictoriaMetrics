package persistentqueue

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// MaxBlockSize is the maximum size of the block persistent queue can work with.
const MaxBlockSize = 32 * 1024 * 1024

const defaultChunkFileSize = (MaxBlockSize + 8) * 16

var chunkFileNameRegex = regexp.MustCompile("^[0-9A-F]{16}$")

// Queue represents persistent queue.
type Queue struct {
	chunkFileSize   uint64
	maxBlockSize    uint64
	maxPendingBytes uint64

	dir  string
	name string

	// mu protects all the fields below.
	mu sync.Mutex

	// cond is used for notifying blocked readers when new data has been added
	// or when MustClose is called.
	cond sync.Cond

	reader            *filestream.Reader
	readerPath        string
	readerOffset      uint64
	readerLocalOffset uint64

	writer              *filestream.Writer
	writerPath          string
	writerOffset        uint64
	writerLocalOffset   uint64
	writerFlushedOffset uint64

	mustStop bool

	blocksDropped *metrics.Counter
	bytesDropped  *metrics.Counter

	blocksWritten *metrics.Counter
	bytesWritten  *metrics.Counter

	blocksRead *metrics.Counter
	bytesRead  *metrics.Counter
}

// ResetIfEmpty resets q if it is empty.
//
// This is needed in order to remove chunk file associated with empty q.
func (q *Queue) ResetIfEmpty() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.readerOffset != q.writerOffset {
		// The queue isn't empty.
		return
	}
	if q.readerOffset < 16*1024*1024 {
		// The file is too small to drop. Leave it as is in order to reduce filesystem load.
		return
	}
	if q.readerPath != q.writerPath {
		logger.Panicf("BUG: readerPath=%q doesn't match writerPath=%q", q.readerPath, q.writerPath)
	}

	q.reader.MustClose()
	q.writer.MustClose()
	fs.MustRemoveAll(q.readerPath)

	q.writerOffset = 0
	q.writerLocalOffset = 0
	q.writerFlushedOffset = 0

	q.readerOffset = 0
	q.readerLocalOffset = 0

	q.writerPath = q.chunkFilePath(q.writerOffset)
	w, err := filestream.Create(q.writerPath, false)
	if err != nil {
		logger.Panicf("FATAL: cannot create chunk file %q: %s", q.writerPath, err)
	}
	q.writer = w

	q.readerPath = q.writerPath
	r, err := filestream.Open(q.readerPath, true)
	if err != nil {
		logger.Panicf("FATAL: cannot open chunk file %q: %s", q.readerPath, err)
	}
	q.reader = r

	if err := q.flushMetainfo(); err != nil {
		logger.Panicf("FATAL: cannot flush metainfo: %s", err)
	}
}

// GetPendingBytes returns the number of pending bytes in the queue.
func (q *Queue) GetPendingBytes() uint64 {
	q.mu.Lock()
	n := q.writerOffset - q.readerOffset
	q.mu.Unlock()
	return n
}

// MustOpen opens persistent queue from the given path.
//
// If maxPendingBytes is greater than 0, then the max queue size is limited by this value.
// The oldest data is deleted when queue size exceeds maxPendingBytes.
func MustOpen(path, name string, maxPendingBytes int) *Queue {
	if maxPendingBytes < 0 {
		maxPendingBytes = 0
	}
	return mustOpen(path, name, defaultChunkFileSize, MaxBlockSize, uint64(maxPendingBytes))
}

func mustOpen(path, name string, chunkFileSize, maxBlockSize, maxPendingBytes uint64) *Queue {
	if chunkFileSize < 8 || chunkFileSize-8 < maxBlockSize {
		logger.Panicf("BUG: too small chunkFileSize=%d for maxBlockSize=%d; chunkFileSize must fit at least one block", chunkFileSize, maxBlockSize)
	}
	if maxBlockSize <= 0 {
		logger.Panicf("BUG: maxBlockSize must be greater than 0; got %d", maxBlockSize)
	}
	q, err := tryOpeningQueue(path, name, chunkFileSize, maxBlockSize, maxPendingBytes)
	if err != nil {
		logger.Errorf("cannot open persistent queue at %q: %s; cleaning it up and trying again", path, err)
		fs.RemoveDirContents(path)
		q, err = tryOpeningQueue(path, name, chunkFileSize, maxBlockSize, maxPendingBytes)
		if err != nil {
			logger.Panicf("FATAL: %s", err)
		}
	}
	return q
}

func tryOpeningQueue(path, name string, chunkFileSize, maxBlockSize, maxPendingBytes uint64) (*Queue, error) {
	var q Queue
	q.chunkFileSize = chunkFileSize
	q.maxBlockSize = maxBlockSize
	q.maxPendingBytes = maxPendingBytes
	q.dir = path
	q.name = name
	q.cond.L = &q.mu

	q.blocksDropped = metrics.GetOrCreateCounter(fmt.Sprintf(`vm_persistentqueue_blocks_dropped_total{path=%q}`, path))
	q.bytesDropped = metrics.GetOrCreateCounter(fmt.Sprintf(`vm_persistentqueue_bytes_dropped_total{path=%q}`, path))
	q.blocksWritten = metrics.GetOrCreateCounter(fmt.Sprintf(`vm_persistentqueue_blocks_written_total{path=%q}`, path))
	q.bytesWritten = metrics.GetOrCreateCounter(fmt.Sprintf(`vm_persistentqueue_bytes_written_total{path=%q}`, path))
	q.blocksRead = metrics.GetOrCreateCounter(fmt.Sprintf(`vm_persistentqueue_blocks_read_total{path=%q}`, path))
	q.bytesRead = metrics.GetOrCreateCounter(fmt.Sprintf(`vm_persistentqueue_bytes_read_total{path=%q}`, path))

	cleanOnError := func() {
		if q.reader != nil {
			q.reader.MustClose()
		}
		if q.writer != nil {
			q.writer.MustClose()
		}
	}

	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, fmt.Errorf("cannot create directory %q: %w", path, err)
	}

	// Read metainfo.
	var mi metainfo
	metainfoPath := q.metainfoPath()
	if err := mi.ReadFromFile(metainfoPath); err != nil {
		if !os.IsNotExist(err) {
			logger.Errorf("cannot read metainfo for persistent queue from %q: %s; re-creating %q", metainfoPath, err, path)
		}

		// path contents is broken or missing. Re-create it from scratch.
		fs.RemoveDirContents(path)
		mi.Reset()
		mi.Name = q.name
		if err := mi.WriteToFile(metainfoPath); err != nil {
			return nil, fmt.Errorf("cannot create %q: %w", metainfoPath, err)
		}

		// Create initial chunk file.
		filepath := q.chunkFilePath(0)
		if err := fs.WriteFileAtomically(filepath, nil); err != nil {
			return nil, fmt.Errorf("cannot create %q: %w", filepath, err)
		}
	}
	if mi.Name != q.name {
		return nil, fmt.Errorf("unexpected queue name; got %q; want %q", mi.Name, q.name)
	}

	// Locate reader and writer chunks in the path.
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read contents of the directory %q: %w", path, err)
	}
	for _, fi := range fis {
		fname := fi.Name()
		filepath := path + "/" + fname
		if fi.IsDir() {
			logger.Errorf("skipping unknown directory %q", filepath)
			continue
		}
		if fname == "metainfo.json" {
			// skip metainfo file
			continue
		}
		if !chunkFileNameRegex.MatchString(fname) {
			logger.Errorf("skipping unknown file %q", filepath)
			continue
		}
		offset, err := strconv.ParseUint(fname, 16, 64)
		if err != nil {
			logger.Panicf("BUG: cannot parse hex %q: %s", fname, err)
		}
		if offset%q.chunkFileSize != 0 {
			logger.Errorf("unexpected offset for chunk file %q: %d; it must divide by %d; removing the file", filepath, offset, q.chunkFileSize)
			fs.MustRemoveAll(filepath)
			continue
		}
		if mi.ReaderOffset >= offset+q.chunkFileSize {
			logger.Errorf("unexpected chunk file found from the past: %q; removing it", filepath)
			fs.MustRemoveAll(filepath)
			continue
		}
		if mi.WriterOffset < offset {
			logger.Errorf("unexpected chunk file found from the future: %q; removing it", filepath)
			fs.MustRemoveAll(filepath)
			continue
		}
		if mi.ReaderOffset >= offset && mi.ReaderOffset < offset+q.chunkFileSize {
			// Found the chunk for reading
			if q.reader != nil {
				logger.Panicf("BUG: reader is already initialized with readerPath=%q, readerOffset=%d, readerLocalOffset=%d",
					q.readerPath, q.readerOffset, q.readerLocalOffset)
			}
			q.readerPath = filepath
			q.readerOffset = mi.ReaderOffset
			q.readerLocalOffset = mi.ReaderOffset % q.chunkFileSize
			if fileSize := fs.MustFileSize(q.readerPath); fileSize < q.readerLocalOffset {
				logger.Errorf("chunk file %q size is too small for the given reader offset; file size %d bytes; reader offset: %d bytes; removing the file",
					q.readerPath, fileSize, q.readerLocalOffset)
				fs.MustRemoveAll(q.readerPath)
				continue
			}
			r, err := filestream.OpenReaderAt(q.readerPath, int64(q.readerLocalOffset), true)
			if err != nil {
				logger.Errorf("cannot open %q for reading at offset %d: %s; removing this file", q.readerPath, q.readerLocalOffset, err)
				fs.MustRemoveAll(filepath)
				continue
			}
			q.reader = r
		}
		if mi.WriterOffset >= offset && mi.WriterOffset < offset+q.chunkFileSize {
			// Found the chunk file for writing
			if q.writer != nil {
				logger.Panicf("BUG: writer is already initialized with writerPath=%q, writerOffset=%d, writerLocalOffset=%d",
					q.writerPath, q.writerOffset, q.writerLocalOffset)
			}
			q.writerPath = filepath
			q.writerOffset = mi.WriterOffset
			q.writerLocalOffset = mi.WriterOffset % q.chunkFileSize
			q.writerFlushedOffset = mi.WriterOffset
			if fileSize := fs.MustFileSize(q.writerPath); fileSize != q.writerLocalOffset {
				logger.Errorf("chunk file %q size doesn't match writer offset; file size %d bytes; writer offset: %d bytes",
					q.writerPath, fileSize, q.writerLocalOffset)
				fs.MustRemoveAll(q.writerPath)
				continue
			}
			w, err := filestream.OpenWriterAt(q.writerPath, int64(q.writerLocalOffset), false)
			if err != nil {
				logger.Errorf("cannot open %q for writing at offset %d: %s; removing this file", q.writerPath, q.writerLocalOffset, err)
				fs.MustRemoveAll(filepath)
				continue
			}
			q.writer = w
		}
	}
	if q.reader == nil {
		cleanOnError()
		return nil, fmt.Errorf("couldn't find chunk file for reading in %q", q.dir)
	}
	if q.writer == nil {
		cleanOnError()
		return nil, fmt.Errorf("couldn't find chunk file for writing in %q", q.dir)
	}
	return &q, nil
}

// MustClose closes q.
//
// It unblocks all the MustReadBlock calls.
//
// MustWriteBlock mustn't be called during and after the call to MustClose.
func (q *Queue) MustClose() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Unblock goroutines blocked on cond in MustReadBlock.
	q.mustStop = true
	q.cond.Broadcast()

	// Close writer.
	q.writer.MustClose()
	q.writer = nil

	// Close reader.
	q.reader.MustClose()
	q.reader = nil

	// Store metainfo
	if err := q.flushMetainfo(); err != nil {
		logger.Panicf("FATAL: cannot flush chunked queue metainfo: %s", err)
	}
}

func (q *Queue) chunkFilePath(offset uint64) string {
	return fmt.Sprintf("%s/%016X", q.dir, offset)
}

func (q *Queue) metainfoPath() string {
	return q.dir + "/metainfo.json"
}

// MustWriteBlock writes block to q.
//
// The block size cannot exceed MaxBlockSize.
//
// It is safe calling this function from concurrent goroutines.
func (q *Queue) MustWriteBlock(block []byte) {
	if uint64(len(block)) > q.maxBlockSize {
		logger.Panicf("BUG: too big block to send: %d bytes; it mustn't exceed %d bytes", len(block), q.maxBlockSize)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.mustStop {
		logger.Panicf("BUG: MustWriteBlock cannot be called after MustClose")
	}
	if q.readerOffset > q.writerOffset {
		logger.Panicf("BUG: readerOffset=%d shouldn't exceed writerOffset=%d", q.readerOffset, q.writerOffset)
	}
	if q.maxPendingBytes > 0 {
		// Drain the oldest blocks until the number of pending bytes becomes enough for the block.
		blockSize := uint64(len(block) + 8)
		maxPendingBytes := q.maxPendingBytes
		if blockSize < maxPendingBytes {
			maxPendingBytes -= blockSize
		} else {
			maxPendingBytes = 0
		}
		bb := blockBufPool.Get()
		for q.writerOffset-q.readerOffset > maxPendingBytes {
			var err error
			bb.B, err = q.readBlockLocked(bb.B[:0])
			if err != nil {
				logger.Panicf("FATAL: cannot read the oldest block %s", err)
			}
			q.blocksDropped.Inc()
			q.bytesDropped.Add(len(bb.B))
		}
		blockBufPool.Put(bb)
		if blockSize > q.maxPendingBytes {
			// The block is too big to put it into the queue. Drop it.
			return
		}
	}
	if err := q.writeBlockLocked(block); err != nil {
		logger.Panicf("FATAL: %s", err)
	}

	// Notify blocked reader if any.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/484 for details.
	q.cond.Signal()
}

var blockBufPool bytesutil.ByteBufferPool

func (q *Queue) writeBlockLocked(block []byte) error {
	if q.writerLocalOffset+q.maxBlockSize+8 > q.chunkFileSize {
		// Finalize the current chunk and start new one.
		q.writer.MustClose()
		if n := q.writerOffset % q.chunkFileSize; n > 0 {
			q.writerOffset += (q.chunkFileSize - n)
		}
		q.writerFlushedOffset = q.writerOffset
		q.writerLocalOffset = 0
		q.writerPath = q.chunkFilePath(q.writerOffset)
		w, err := filestream.Create(q.writerPath, false)
		if err != nil {
			return fmt.Errorf("cannot create chunk file %q: %w", q.writerPath, err)
		}
		q.writer = w
		if err := q.flushMetainfo(); err != nil {
			return fmt.Errorf("cannot flush metainfo: %w", err)
		}
	}

	// Write block len.
	blockLen := uint64(len(block))
	header := headerBufPool.Get()
	header.B = encoding.MarshalUint64(header.B, blockLen)
	err := q.write(header.B)
	headerBufPool.Put(header)
	if err != nil {
		return fmt.Errorf("cannot write header with size 8 bytes to %q: %w", q.writerPath, err)
	}

	// Write block contents.
	if err := q.write(block); err != nil {
		return fmt.Errorf("cannot write block contents with size %d bytes to %q: %w", len(block), q.writerPath, err)
	}
	q.blocksWritten.Inc()
	q.bytesWritten.Add(len(block))
	return nil
}

// MustReadBlock appends the next block from q to dst and returns the result.
//
// false is returned after MustClose call.
//
// It is safe calling this function from concurrent goroutines.
func (q *Queue) MustReadBlock(dst []byte) ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for {
		if q.mustStop {
			return dst, false
		}
		if q.readerOffset > q.writerOffset {
			logger.Panicf("BUG: readerOffset=%d cannot exceed writerOffset=%d", q.readerOffset, q.writerOffset)
		}
		if q.readerOffset < q.writerOffset {
			break
		}
		q.cond.Wait()
	}

	data, err := q.readBlockLocked(dst)
	if err != nil {
		logger.Panicf("FATAL: %s", err)
	}
	return data, true
}

func (q *Queue) readBlockLocked(dst []byte) ([]byte, error) {
	if q.readerLocalOffset+q.maxBlockSize+8 > q.chunkFileSize {
		// Remove the current chunk and go to the next chunk.
		q.reader.MustClose()
		fs.MustRemoveAll(q.readerPath)
		if n := q.readerOffset % q.chunkFileSize; n > 0 {
			q.readerOffset += (q.chunkFileSize - n)
		}
		q.readerLocalOffset = 0
		q.readerPath = q.chunkFilePath(q.readerOffset)
		r, err := filestream.Open(q.readerPath, true)
		if err != nil {
			return dst, fmt.Errorf("cannot open chunk file %q: %w", q.readerPath, err)
		}
		q.reader = r
		if err := q.flushMetainfo(); err != nil {
			return dst, fmt.Errorf("cannot flush metainfo: %w", err)
		}
	}

	// Read block len.
	header := headerBufPool.Get()
	header.B = bytesutil.Resize(header.B, 8)
	err := q.readFull(header.B)
	blockLen := encoding.UnmarshalUint64(header.B)
	headerBufPool.Put(header)
	if err != nil {
		return dst, fmt.Errorf("cannot read header with size 8 bytes from %q: %w", q.readerPath, err)
	}
	if blockLen > q.maxBlockSize {
		return dst, fmt.Errorf("too big block size read from %q: %d bytes; cannot exceed %d bytes", q.readerPath, blockLen, q.maxBlockSize)
	}

	// Read block contents.
	dstLen := len(dst)
	dst = bytesutil.Resize(dst, dstLen+int(blockLen))
	if err := q.readFull(dst[dstLen:]); err != nil {
		return dst, fmt.Errorf("cannot read block contents with size %d bytes from %q: %w", blockLen, q.readerPath, err)
	}
	q.blocksRead.Inc()
	q.bytesRead.Add(int(blockLen))
	return dst, nil
}

func (q *Queue) write(buf []byte) error {
	bufLen := uint64(len(buf))
	n, err := q.writer.Write(buf)
	if err != nil {
		return err
	}
	if uint64(n) != bufLen {
		return fmt.Errorf("unexpected number of bytes written; got %d bytes; want %d bytes", n, bufLen)
	}
	q.writerLocalOffset += bufLen
	q.writerOffset += bufLen
	return nil
}

func (q *Queue) readFull(buf []byte) error {
	bufLen := uint64(len(buf))
	if q.readerOffset+bufLen > q.writerFlushedOffset {
		q.writer.MustFlush()
		q.writerFlushedOffset = q.writerOffset
	}
	n, err := io.ReadFull(q.reader, buf)
	if err != nil {
		return err
	}
	if uint64(n) != bufLen {
		return fmt.Errorf("unexpected number of bytes read; got %d bytes; want %d bytes", n, bufLen)
	}
	q.readerLocalOffset += bufLen
	q.readerOffset += bufLen
	return nil
}

func (q *Queue) flushMetainfo() error {
	mi := &metainfo{
		Name:         q.name,
		ReaderOffset: q.readerOffset,
		WriterOffset: q.writerOffset,
	}
	metainfoPath := q.metainfoPath()
	if err := mi.WriteToFile(metainfoPath); err != nil {
		return fmt.Errorf("cannot write metainfo to %q: %w", metainfoPath, err)
	}
	return nil
}

var headerBufPool bytesutil.ByteBufferPool

type metainfo struct {
	Name         string
	ReaderOffset uint64
	WriterOffset uint64
}

func (mi *metainfo) Reset() {
	mi.ReaderOffset = 0
	mi.WriterOffset = 0
}

func (mi *metainfo) WriteToFile(path string) error {
	data, err := json.Marshal(mi)
	if err != nil {
		return fmt.Errorf("cannot marshal persistent queue metainfo %#v: %w", mi, err)
	}
	if err := ioutil.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write persistent queue metainfo to %q: %w", path, err)
	}
	return nil
}

func (mi *metainfo) ReadFromFile(path string) error {
	mi.Reset()
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fmt.Errorf("cannot read %q: %w", path, err)
	}
	if err := json.Unmarshal(data, mi); err != nil {
		return fmt.Errorf("cannot unmarshal persistent queue metainfo from %q: %w", path, err)
	}
	if mi.ReaderOffset > mi.WriterOffset {
		return fmt.Errorf("invalid data read from %q: readerOffset=%d cannot exceed writerOffset=%d", path, mi.ReaderOffset, mi.WriterOffset)
	}
	return nil
}
