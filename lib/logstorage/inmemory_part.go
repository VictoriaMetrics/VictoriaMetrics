package logstorage

import (
	"path/filepath"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// inmemoryPart is an in-memory part.
type inmemoryPart struct {
	// ph contains partHeader information for the given in-memory part.
	ph partHeader

	metaindex          bytesutil.ByteBuffer
	index              bytesutil.ByteBuffer
	columnsHeader      bytesutil.ByteBuffer
	timestamps         bytesutil.ByteBuffer
	fieldValues        bytesutil.ByteBuffer
	fieldBloomFilter   bytesutil.ByteBuffer
	messageValues      bytesutil.ByteBuffer
	messageBloomFilter bytesutil.ByteBuffer
}

// reset resets mp, so it can be re-used
func (mp *inmemoryPart) reset() {
	mp.ph.reset()

	mp.metaindex.Reset()
	mp.index.Reset()
	mp.columnsHeader.Reset()
	mp.timestamps.Reset()
	mp.fieldValues.Reset()
	mp.fieldBloomFilter.Reset()
	mp.messageValues.Reset()
	mp.messageBloomFilter.Reset()
}

// mustInitFromRows initializes mp from lr.
func (mp *inmemoryPart) mustInitFromRows(lr *LogRows) {
	mp.reset()

	if len(lr.timestamps) == 0 {
		return
	}

	sort.Sort(lr)

	bsw := getBlockStreamWriter()
	bsw.MustInitForInmemoryPart(mp)
	trs := getTmpRows()
	var sidPrev *streamID
	uncompressedBlockSizeBytes := uint64(0)
	timestamps := lr.timestamps
	rows := lr.rows
	streamIDs := lr.streamIDs
	for i := range timestamps {
		streamID := &streamIDs[i]
		if sidPrev == nil {
			sidPrev = streamID
		}

		if uncompressedBlockSizeBytes >= maxUncompressedBlockSize || !streamID.equal(sidPrev) {
			bsw.MustWriteRows(sidPrev, trs.timestamps, trs.rows)
			trs.reset()
			sidPrev = streamID
			uncompressedBlockSizeBytes = 0
		}
		fields := rows[i]
		trs.timestamps = append(trs.timestamps, timestamps[i])
		trs.rows = append(trs.rows, fields)
		uncompressedBlockSizeBytes += uncompressedRowSizeBytes(fields)
	}
	bsw.MustWriteRows(sidPrev, trs.timestamps, trs.rows)
	putTmpRows(trs)
	bsw.Finalize(&mp.ph)
	putBlockStreamWriter(bsw)
}

// MustStoreToDisk stores mp to disk at the given path.
func (mp *inmemoryPart) MustStoreToDisk(path string) {
	fs.MustMkdirFailIfExist(path)

	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)
	fieldValuesPath := filepath.Join(path, fieldValuesFilename)
	fieldBloomFilterPath := filepath.Join(path, fieldBloomFilename)
	messageValuesPath := filepath.Join(path, messageValuesFilename)
	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)

	fs.MustWriteSync(metaindexPath, mp.metaindex.B)
	fs.MustWriteSync(indexPath, mp.index.B)
	fs.MustWriteSync(columnsHeaderPath, mp.columnsHeader.B)
	fs.MustWriteSync(timestampsPath, mp.timestamps.B)
	fs.MustWriteSync(fieldValuesPath, mp.fieldValues.B)
	fs.MustWriteSync(fieldBloomFilterPath, mp.fieldBloomFilter.B)
	fs.MustWriteSync(messageValuesPath, mp.messageValues.B)
	fs.MustWriteSync(messageBloomFilterPath, mp.messageBloomFilter.B)

	mp.ph.mustWriteMetadata(path)

	fs.MustSyncPath(path)
	// Do not sync parent directory - it must be synced by the caller.
}

// tmpRows is used as a helper for inmemoryPart.mustInitFromRows()
type tmpRows struct {
	timestamps []int64

	rows [][]Field
}

func (trs *tmpRows) reset() {
	trs.timestamps = trs.timestamps[:0]

	rows := trs.rows
	for i := range rows {
		rows[i] = nil
	}
	trs.rows = rows[:0]
}

func getTmpRows() *tmpRows {
	v := tmpRowsPool.Get()
	if v == nil {
		return &tmpRows{}
	}
	return v.(*tmpRows)
}

func putTmpRows(trs *tmpRows) {
	trs.reset()
	tmpRowsPool.Put(trs)
}

var tmpRowsPool sync.Pool

func getInmemoryPart() *inmemoryPart {
	v := inmemoryPartPool.Get()
	if v == nil {
		return &inmemoryPart{}
	}
	return v.(*inmemoryPart)
}

func putInmemoryPart(mp *inmemoryPart) {
	mp.reset()
	inmemoryPartPool.Put(mp)
}

var inmemoryPartPool sync.Pool
