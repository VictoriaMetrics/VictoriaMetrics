package persistentqueue

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// MaxBlockSize is the maximum size of the block persistent queue can work with.
const MaxBlockSize = 32 * 1024 * 1024

// DefaultChunkFileSize represents default chunk file size
const DefaultChunkFileSize = (MaxBlockSize + 8) * 16

var chunkFileNameRegex = regexp.MustCompile("^[0-9A-F]{16}$")

// queue represents persistent queue.
//
// It is unsafe to call queue methods from concurrent goroutines.
type queue struct {
	chunkFileSize   uint64
	maxBlockSize    uint64
	maxPendingBytes uint64

	dir  string
	name string

	flockF *os.File

	reader            *filestream.Reader
	readerPath        string
	readerOffset      uint64
	readerLocalOffset uint64

	writer              *filestream.Writer
	writerPath          string
	writerOffset        uint64
	writerLocalOffset   uint64
	writerFlushedOffset uint64

	lastMetainfoFlushTime uint64

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
func (q *queue) ResetIfEmpty() {
	if q.readerOffset != q.writerOffset {
		// The queue isn't empty.
		return
	}
	if q.readerOffset < 16*1024*1024 {
		// The file is too small to drop. Leave it as is in order to reduce filesystem load.
		return
	}
	q.mustResetFiles()
}

func (q *queue) mustResetFiles() {
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
	w := filestream.MustCreate(q.writerPath, false)
	q.writer = w

	q.readerPath = q.writerPath
	r := filestream.MustOpen(q.readerPath, true)
	q.reader = r

	if err := q.flushMetainfo(); err != nil {
		logger.Panicf("FATAL: cannot flush metainfo: %s", err)
	}
}

// GetPendingBytes returns the number of pending bytes in the queue.
func (q *queue) GetPendingBytes() uint64 {
	if q.readerOffset > q.writerOffset {
		logger.Panicf("BUG: readerOffset=%d cannot exceed writerOffset=%d", q.readerOffset, q.writerOffset)
	}
	n := q.writerOffset - q.readerOffset
	return n
}

// mustOpen opens persistent queue from the given path.
//
// If maxPendingBytes is greater than 0, then the max queue size is limited by this value.
// The oldest data is deleted when queue size exceeds maxPendingBytes.
func mustOpen(path, name string, maxPendingBytes int64) *queue {
	if maxPendingBytes < 0 {
		maxPendingBytes = 0
	}
	return mustOpenInternal(path, name, DefaultChunkFileSize, MaxBlockSize, uint64(maxPendingBytes))
}

func mustOpenInternal(path, name string, chunkFileSize, maxBlockSize, maxPendingBytes uint64) *queue {
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

func tryOpeningQueue(path, name string, chunkFileSize, maxBlockSize, maxPendingBytes uint64) (*queue, error) {
	// Protect from concurrent opens.
	var q queue
	q.chunkFileSize = chunkFileSize
	q.maxBlockSize = maxBlockSize
	q.maxPendingBytes = maxPendingBytes
	q.dir = path
	q.name = name

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

	fs.MustMkdirIfNotExist(path)
	q.flockF = fs.MustCreateFlockFile(path)
	mustCloseFlockF := true
	defer func() {
		if mustCloseFlockF {
			fs.MustClose(q.flockF)
		}
	}()

	// Read metainfo.
	var mi metainfo
	metainfoPath := q.metainfoPath()
	if err := mi.ReadFromFile(metainfoPath); err != nil {
		if !os.IsNotExist(err) {
			logger.Errorf("cannot read metainfo for persistent queue from %q: %s; re-creating %q", metainfoPath, err, path)
		}

		// path contents is broken or missing. Re-create it from scratch.
		fs.MustClose(q.flockF)
		fs.RemoveDirContents(path)
		q.flockF = fs.MustCreateFlockFile(path)
		mi.Reset()
		mi.Name = q.name
		if err := mi.WriteToFile(metainfoPath); err != nil {
			return nil, fmt.Errorf("cannot create %q: %w", metainfoPath, err)
		}

		// Create initial chunk file.
		filepath := q.chunkFilePath(0)
		fs.MustWriteAtomic(filepath, nil, false)
	}

	// Locate reader and writer chunks in the path.
	des := fs.MustReadDir(path)
	for _, de := range des {
		fname := de.Name()
		filepath := filepath.Join(path, fname)
		if de.IsDir() {
			logger.Errorf("skipping unknown directory %q", filepath)
			continue
		}
		if fname == metainfoFilename {
			// skip metainfo file
			continue
		}
		if fname == fs.FlockFilename {
			// skip flock file
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
			logger.Errorf("unexpected offset for chunk file %q: %d; it must be multiple of %d; removing the file", filepath, offset, q.chunkFileSize)
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
				if fileSize < q.writerLocalOffset {
					logger.Errorf("%q size (%d bytes) is smaller than the writer offset (%d bytes); removing the file",
						q.writerPath, fileSize, q.writerLocalOffset)
					fs.MustRemoveAll(q.writerPath)
					continue
				}
				logger.Warnf("%q size (%d bytes) is bigger than writer offset (%d bytes); "+
					"this may be the case on unclean shutdown (OOM, `kill -9`, hardware reset); trying to fix it by adjusting fileSize to %d",
					q.writerPath, fileSize, q.writerLocalOffset, q.writerLocalOffset)
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
	if q.readerOffset > q.writerOffset {
		cleanOnError()
		return nil, fmt.Errorf("readerOffset=%d cannot exceed writerOffset=%d", q.readerOffset, q.writerOffset)
	}
	mustCloseFlockF = false
	return &q, nil
}

// MustClose closes q.
//
// MustWriteBlock mustn't be called during and after the call to MustClose.
func (q *queue) MustClose() {
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

	// Close flockF
	fs.MustClose(q.flockF)
	q.flockF = nil
}

func (q *queue) chunkFilePath(offset uint64) string {
	return filepath.Join(q.dir, fmt.Sprintf("%016X", offset))
}

func (q *queue) metainfoPath() string {
	return filepath.Join(q.dir, metainfoFilename)
}

// MustWriteBlock writes block to q.
//
// The block size cannot exceed MaxBlockSize.
func (q *queue) MustWriteBlock(block []byte) {
	if uint64(len(block)) > q.maxBlockSize {
		logger.Panicf("BUG: too big block to send: %d bytes; it mustn't exceed %d bytes", len(block), q.maxBlockSize)
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
			bb.B, err = q.readBlock(bb.B[:0])
			if err == errEmptyQueue {
				break
			}
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
	if err := q.writeBlock(block); err != nil {
		logger.Panicf("FATAL: %s", err)
	}
}

var blockBufPool bytesutil.ByteBufferPool

func (q *queue) writeBlock(block []byte) error {
	startTime := time.Now()
	defer func() {
		writeDurationSeconds.Add(time.Since(startTime).Seconds())
	}()
	if q.writerLocalOffset+q.maxBlockSize+8 > q.chunkFileSize {
		if err := q.nextChunkFileForWrite(); err != nil {
			return fmt.Errorf("cannot create next chunk file: %w", err)
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
	return q.flushWriterMetainfoIfNeeded()
}

var writeDurationSeconds = metrics.NewFloatCounter(`vm_persistentqueue_write_duration_seconds_total`)

func (q *queue) nextChunkFileForWrite() error {
	// Finalize the current chunk and start new one.
	q.writer.MustClose()
	// There is no need to do fs.MustSyncPath(q.writerPath) here,
	// since MustClose already does this.
	if n := q.writerOffset % q.chunkFileSize; n > 0 {
		q.writerOffset += q.chunkFileSize - n
	}
	q.writerFlushedOffset = q.writerOffset
	q.writerLocalOffset = 0
	q.writerPath = q.chunkFilePath(q.writerOffset)
	w := filestream.MustCreate(q.writerPath, false)
	q.writer = w
	if err := q.flushMetainfo(); err != nil {
		return fmt.Errorf("cannot flush metainfo: %w", err)
	}
	fs.MustSyncPath(q.dir)
	return nil
}

// MustReadBlockNonblocking appends the next block from q to dst and returns the result.
//
// false is returned if q is empty.
func (q *queue) MustReadBlockNonblocking(dst []byte) ([]byte, bool) {
	if q.readerOffset > q.writerOffset {
		logger.Panicf("BUG: readerOffset=%d cannot exceed writerOffset=%d", q.readerOffset, q.writerOffset)
	}
	if q.readerOffset == q.writerOffset {
		return dst, false
	}
	var err error
	dst, err = q.readBlock(dst)
	if err != nil {
		if err == errEmptyQueue {
			return dst, false
		}
		logger.Panicf("FATAL: %s", err)
	}
	return dst, true
}

func (q *queue) readBlock(dst []byte) ([]byte, error) {
	startTime := time.Now()
	defer func() {
		readDurationSeconds.Add(time.Since(startTime).Seconds())
	}()
	if q.readerLocalOffset+q.maxBlockSize+8 > q.chunkFileSize {
		if err := q.nextChunkFileForRead(); err != nil {
			return dst, fmt.Errorf("cannot open next chunk file: %w", err)
		}
	}

again:
	// Read block len.
	header := headerBufPool.Get()
	header.B = bytesutil.ResizeNoCopyMayOverallocate(header.B, 8)
	err := q.readFull(header.B)
	blockLen := encoding.UnmarshalUint64(header.B)
	headerBufPool.Put(header)
	if err != nil {
		logger.Errorf("skipping corrupted %q, since header with size 8 bytes cannot be read from it: %s", q.readerPath, err)
		if err := q.skipBrokenChunkFile(); err != nil {
			return dst, err
		}
		goto again
	}
	if blockLen > q.maxBlockSize {
		logger.Errorf("skipping corrupted %q, since too big block size is read from it: %d bytes; cannot exceed %d bytes", q.readerPath, blockLen, q.maxBlockSize)
		if err := q.skipBrokenChunkFile(); err != nil {
			return dst, err
		}
		goto again
	}

	// Read block contents.
	dstLen := len(dst)
	dst = bytesutil.ResizeWithCopyMayOverallocate(dst, dstLen+int(blockLen))
	if err := q.readFull(dst[dstLen:]); err != nil {
		logger.Errorf("skipping corrupted %q, since contents with size %d bytes cannot be read from it: %s", q.readerPath, blockLen, err)
		if err := q.skipBrokenChunkFile(); err != nil {
			return dst[:dstLen], err
		}
		goto again
	}
	q.blocksRead.Inc()
	q.bytesRead.Add(int(blockLen))
	if err := q.flushReaderMetainfoIfNeeded(); err != nil {
		return dst, err
	}
	return dst, nil
}

var readDurationSeconds = metrics.NewFloatCounter(`vm_persistentqueue_read_duration_seconds_total`)

func (q *queue) skipBrokenChunkFile() error {
	// Try to recover from broken chunk file by skipping it.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1030
	q.readerOffset += q.chunkFileSize - q.readerOffset%q.chunkFileSize
	if q.readerOffset >= q.writerOffset {
		q.mustResetFiles()
		return errEmptyQueue
	}
	return q.nextChunkFileForRead()
}

var errEmptyQueue = fmt.Errorf("the queue is empty")

func (q *queue) nextChunkFileForRead() error {
	// Remove the current chunk and go to the next chunk.
	q.reader.MustClose()
	fs.MustRemoveAll(q.readerPath)
	if n := q.readerOffset % q.chunkFileSize; n > 0 {
		q.readerOffset += q.chunkFileSize - n
	}
	if err := q.checkReaderWriterOffsets(); err != nil {
		return err
	}
	q.readerLocalOffset = 0
	q.readerPath = q.chunkFilePath(q.readerOffset)
	r := filestream.MustOpen(q.readerPath, true)
	q.reader = r
	if err := q.flushMetainfo(); err != nil {
		return fmt.Errorf("cannot flush metainfo: %w", err)
	}
	fs.MustSyncPath(q.dir)
	return nil
}

func (q *queue) write(buf []byte) error {
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

func (q *queue) readFull(buf []byte) error {
	bufLen := uint64(len(buf))
	if q.readerOffset+bufLen > q.writerFlushedOffset {
		q.writer.MustFlush(false)
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
	return q.checkReaderWriterOffsets()
}

func (q *queue) checkReaderWriterOffsets() error {
	if q.readerOffset > q.writerOffset {
		return fmt.Errorf("readerOffset=%d cannot exceed writerOffset=%d; it is likely persistent queue files were corrupted on unclean shutdown",
			q.readerOffset, q.writerOffset)
	}
	return nil
}

func (q *queue) flushReaderMetainfoIfNeeded() error {
	t := fasttime.UnixTimestamp()
	if t == q.lastMetainfoFlushTime {
		return nil
	}
	if err := q.flushMetainfo(); err != nil {
		return fmt.Errorf("cannot flush metainfo: %w", err)
	}
	q.lastMetainfoFlushTime = t
	return nil
}

func (q *queue) flushWriterMetainfoIfNeeded() error {
	t := fasttime.UnixTimestamp()
	if t == q.lastMetainfoFlushTime {
		return nil
	}
	q.writer.MustFlush(true)
	if err := q.flushMetainfo(); err != nil {
		return fmt.Errorf("cannot flush metainfo: %w", err)
	}
	q.lastMetainfoFlushTime = t
	return nil
}

func (q *queue) flushMetainfo() error {
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
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write persistent queue metainfo to %q: %w", path, err)
	}
	fs.MustSyncPath(path)
	return nil
}

func (mi *metainfo) ReadFromFile(path string) error {
	mi.Reset()
	data, err := os.ReadFile(path)
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
