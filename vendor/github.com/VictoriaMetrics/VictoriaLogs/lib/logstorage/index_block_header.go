package logstorage

import (
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// indexBlockHeader contains index information about multiple blocks.
//
// It allows locating the block by streamID and/or by time range.
type indexBlockHeader struct {
	// streamID is the minimum streamID covered by the indexBlockHeader
	streamID streamID

	// minTimestamp is the mimumum timestamp seen across blocks covered by the indexBlockHeader
	minTimestamp int64

	// maxTimestamp is the maximum timestamp seen across blocks covered by the indexBlockHeader
	maxTimestamp int64

	// indexBlockOffset is an offset of the linked index block at indexFilename
	indexBlockOffset uint64

	// indexBlockSize is the size of the linked index block at indexFilename
	indexBlockSize uint64
}

// reset resets ih for subsequent reuse.
func (ih *indexBlockHeader) reset() {
	ih.streamID.reset()
	ih.minTimestamp = 0
	ih.maxTimestamp = 0
	ih.indexBlockOffset = 0
	ih.indexBlockSize = 0
}

// mustWriteIndexBlock writes data with the given additional args to sw and updates ih accordingly.
func (ih *indexBlockHeader) mustWriteIndexBlock(data []byte, sidFirst streamID, minTimestamp, maxTimestamp int64, sw *streamWriters) {
	ih.streamID = sidFirst
	ih.minTimestamp = minTimestamp
	ih.maxTimestamp = maxTimestamp

	bb := longTermBufPool.Get()
	bb.B = encoding.CompressZSTDLevel(bb.B[:0], data, 1)
	ih.indexBlockOffset = sw.indexWriter.bytesWritten
	ih.indexBlockSize = uint64(len(bb.B))
	sw.indexWriter.MustWrite(bb.B)
	longTermBufPool.Put(bb)
}

// mustReadNextIndexBlock reads the next index block associated with ih from src, appends it to dst and returns the result.
func (ih *indexBlockHeader) mustReadNextIndexBlock(dst []byte, sr *streamReaders) []byte {
	indexReader := &sr.indexReader

	indexBlockSize := ih.indexBlockSize
	if indexBlockSize > maxIndexBlockSize {
		logger.Panicf("FATAL: %s: indexBlockHeader.indexBlockSize=%d cannot exceed %d bytes", indexReader.Path(), indexBlockSize, maxIndexBlockSize)
	}
	if ih.indexBlockOffset != indexReader.bytesRead {
		logger.Panicf("FATAL: %s: indexBlockHeader.indexBlockOffset=%d must equal to %d", indexReader.Path(), ih.indexBlockOffset, indexReader.bytesRead)
	}
	bbCompressed := longTermBufPool.Get()
	bbCompressed.B = bytesutil.ResizeNoCopyMayOverallocate(bbCompressed.B, int(indexBlockSize))
	indexReader.MustReadFull(bbCompressed.B)

	// Decompress bbCompressed to dst
	var err error
	dst, err = encoding.DecompressZSTD(dst, bbCompressed.B)
	longTermBufPool.Put(bbCompressed)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot decompress indexBlock read at offset %d with size %d: %s", indexReader.Path(), ih.indexBlockOffset, indexBlockSize, err)
	}
	return dst
}

// marshal appends marshaled ih to dst and returns the result.
func (ih *indexBlockHeader) marshal(dst []byte) []byte {
	dst = ih.streamID.marshal(dst)
	dst = encoding.MarshalUint64(dst, uint64(ih.minTimestamp))
	dst = encoding.MarshalUint64(dst, uint64(ih.maxTimestamp))
	dst = encoding.MarshalUint64(dst, ih.indexBlockOffset)
	dst = encoding.MarshalUint64(dst, ih.indexBlockSize)
	return dst
}

// unmarshal unmarshals ih from src and returns the tail left.
func (ih *indexBlockHeader) unmarshal(src []byte) ([]byte, error) {
	srcOrig := src

	// unmarshal ih.streamID
	tail, err := ih.streamID.unmarshal(src)
	if err != nil {
		return srcOrig, fmt.Errorf("cannot unmarshal streamID: %w", err)
	}
	src = tail

	// unmarshal the rest of indexBlockHeader fields
	if len(src) < 32 {
		return srcOrig, fmt.Errorf("cannot unmarshal indexBlockHeader from %d bytes; need at least 32 bytes", len(src))
	}
	ih.minTimestamp = int64(encoding.UnmarshalUint64(src))
	ih.maxTimestamp = int64(encoding.UnmarshalUint64(src[8:]))
	ih.indexBlockOffset = encoding.UnmarshalUint64(src[16:])
	ih.indexBlockSize = encoding.UnmarshalUint64(src[24:])

	return src[32:], nil
}

// mustWriteIndexBlockHeaders writes metaindexData to w.
func mustWriteIndexBlockHeaders(w *writerWithStats, metaindexData []byte) {
	bb := longTermBufPool.Get()
	bb.B = encoding.CompressZSTDLevel(bb.B[:0], metaindexData, 1)
	w.MustWrite(bb.B)
	if len(bb.B) < 1024*1024 {
		longTermBufPool.Put(bb)
	}
}

// mustReadIndexBlockHeaders reads indexBlockHeader entries from r, appends them to dst and returns the result.
func mustReadIndexBlockHeaders(dst []indexBlockHeader, r *readerWithStats) []indexBlockHeader {
	data, err := io.ReadAll(r)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot read indexBlockHeader entries: %s", r.Path(), err)
	}

	bb := longTermBufPool.Get()
	bb.B, err = encoding.DecompressZSTD(bb.B[:0], data)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot decompress indexBlockHeader entries: %s", r.Path(), err)
	}
	dst, err = unmarshalIndexBlockHeaders(dst, bb.B)
	if len(bb.B) < 1024*1024 {
		longTermBufPool.Put(bb)
	}
	if err != nil {
		logger.Panicf("FATAL: %s: cannot parse indexBlockHeader entries: %s", r.Path(), err)
	}

	return dst
}

// unmarshalIndexBlockHeaders appends unmarshaled from src indexBlockHeader entries to dst and returns the result.
func unmarshalIndexBlockHeaders(dst []indexBlockHeader, src []byte) ([]indexBlockHeader, error) {
	dstOrig := dst
	for len(src) > 0 {
		if len(dst) < cap(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, indexBlockHeader{})
		}
		ih := &dst[len(dst)-1]
		tail, err := ih.unmarshal(src)
		if err != nil {
			return dstOrig, fmt.Errorf("cannot unmarshal indexBlockHeader %d: %w", len(dst)-len(dstOrig), err)
		}
		src = tail
	}
	if err := validateIndexBlockHeaders(dst[len(dstOrig):]); err != nil {
		return dstOrig, err
	}
	return dst, nil
}

func validateIndexBlockHeaders(ihs []indexBlockHeader) error {
	for i := 1; i < len(ihs); i++ {
		if ihs[i].streamID.less(&ihs[i-1].streamID) {
			return fmt.Errorf("unexpected indexBlockHeader with smaller streamID=%s after bigger streamID=%s", &ihs[i].streamID, &ihs[i-1].streamID)
		}
	}
	return nil
}
