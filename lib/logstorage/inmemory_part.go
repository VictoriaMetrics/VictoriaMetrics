package logstorage

import (
	"path/filepath"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/chunkedbuffer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// inmemoryPart is an in-memory part.
type inmemoryPart struct {
	// ph contains partHeader information for the given in-memory part.
	ph partHeader

	columnNames        chunkedbuffer.Buffer
	columnIdxs         chunkedbuffer.Buffer
	metaindex          chunkedbuffer.Buffer
	index              chunkedbuffer.Buffer
	columnsHeaderIndex chunkedbuffer.Buffer
	columnsHeader      chunkedbuffer.Buffer
	timestamps         chunkedbuffer.Buffer

	messageBloomValues bloomValuesBuffer
	fieldBloomValues   bloomValuesBuffer
}

type bloomValuesBuffer struct {
	bloom  chunkedbuffer.Buffer
	values chunkedbuffer.Buffer
}

func (b *bloomValuesBuffer) reset() {
	b.bloom.Reset()
	b.values.Reset()
}

func (b *bloomValuesBuffer) NewStreamReader() bloomValuesStreamReader {
	return bloomValuesStreamReader{
		bloom:  b.bloom.NewReader(),
		values: b.values.NewReader(),
	}
}

func (b *bloomValuesBuffer) NewStreamWriter() bloomValuesStreamWriter {
	return bloomValuesStreamWriter{
		bloom:  &b.bloom,
		values: &b.values,
	}
}

// reset resets mp, so it can be reused
func (mp *inmemoryPart) reset() {
	mp.ph.reset()

	mp.columnNames.Reset()
	mp.columnIdxs.Reset()
	mp.metaindex.Reset()
	mp.index.Reset()
	mp.columnsHeaderIndex.Reset()
	mp.columnsHeader.Reset()
	mp.timestamps.Reset()

	mp.messageBloomValues.reset()
	mp.fieldBloomValues.reset()
}

// mustInitFromRows initializes mp from lr.
func (mp *inmemoryPart) mustInitFromRows(lr *logRows) {
	mp.reset()

	sort.Sort(lr)
	lr.sortFieldsInRows()

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

	columnNamesPath := filepath.Join(path, columnNamesFilename)
	columnIdxsPath := filepath.Join(path, columnIdxsFilename)
	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderIndexPath := filepath.Join(path, columnsHeaderIndexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)
	messageValuesPath := filepath.Join(path, messageValuesFilename)
	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)

	fs.MustWriteStreamSync(columnNamesPath, &mp.columnNames)
	fs.MustWriteStreamSync(columnIdxsPath, &mp.columnIdxs)
	fs.MustWriteStreamSync(metaindexPath, &mp.metaindex)
	fs.MustWriteStreamSync(indexPath, &mp.index)
	fs.MustWriteStreamSync(columnsHeaderIndexPath, &mp.columnsHeaderIndex)
	fs.MustWriteStreamSync(columnsHeaderPath, &mp.columnsHeader)
	fs.MustWriteStreamSync(timestampsPath, &mp.timestamps)

	fs.MustWriteStreamSync(messageBloomFilterPath, &mp.messageBloomValues.bloom)
	fs.MustWriteStreamSync(messageValuesPath, &mp.messageBloomValues.values)

	bloomPath := getBloomFilePath(path, 0)
	fs.MustWriteStreamSync(bloomPath, &mp.fieldBloomValues.bloom)

	valuesPath := getValuesFilePath(path, 0)
	fs.MustWriteStreamSync(valuesPath, &mp.fieldBloomValues.values)

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
