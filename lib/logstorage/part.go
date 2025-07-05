package logstorage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type part struct {
	// pt is the partition the part belongs to
	pt *partition

	// path is the path to the part on disk.
	//
	// If the part is in-memory then the path is empty.
	path string

	// ph contains partHeader for the given part.
	ph partHeader

	// columnNameIDs is a mapping from column names seen in the given part to internal IDs.
	// The internal IDs are used in columnHeaderRef.
	columnNameIDs map[string]uint64

	// columnNames is a mapping from internal IDs to column names.
	// The internal IDs are used in columnHeaderRef.
	columnNames []string

	// columnIdxs is a mapping from column name to the corresponding item at bloomValuesShards
	columnIdxs map[string]uint64

	// indexBlockHeaders contains a list of indexBlockHeader entries for the given part.
	indexBlockHeaders []indexBlockHeader

	indexFile              fs.MustReadAtCloser
	columnsHeaderIndexFile fs.MustReadAtCloser
	columnsHeaderFile      fs.MustReadAtCloser
	timestampsFile         fs.MustReadAtCloser

	messageBloomValues bloomValuesReaderAt
	oldBloomValues     bloomValuesReaderAt

	bloomValuesShards []bloomValuesReaderAt

	// marker holds marker data for marker types (delete, ttl, etc.). May be nil if not present.
	marker *marker

	// appliedTSeq is the highest delete task sequence applied to this part.
	appliedTSeq atomic.Uint64
}

type bloomValuesReaderAt struct {
	bloom  fs.MustReadAtCloser
	values fs.MustReadAtCloser
}

func (r *bloomValuesReaderAt) MustClose() {
	r.bloom.MustClose()
	r.values.MustClose()
}

func mustOpenInmemoryPart(pt *partition, mp *inmemoryPart) *part {
	var p part
	p.pt = pt
	p.path = ""
	p.ph = mp.ph

	// Read columnNames
	columnNamesReader := mp.columnNames.NewReader()
	p.columnNames, p.columnNameIDs = mustReadColumnNames(columnNamesReader)
	columnNamesReader.MustClose()

	// Read columnIdxs
	columnIdxsReader := mp.columnIdxs.NewReader()
	p.columnIdxs = mustReadColumnIdxs(columnIdxsReader, p.columnNames, p.ph.BloomValuesShardsCount)
	columnIdxsReader.MustClose()

	// Read metaindex
	metaindexReader := mp.metaindex.NewReader()
	var mrs readerWithStats
	mrs.init(metaindexReader)
	p.indexBlockHeaders = mustReadIndexBlockHeaders(p.indexBlockHeaders[:0], &mrs)
	metaindexReader.MustClose()

	// Open data files
	p.indexFile = &mp.index
	p.columnsHeaderIndexFile = &mp.columnsHeaderIndex
	p.columnsHeaderFile = &mp.columnsHeader
	p.timestampsFile = &mp.timestamps

	// Open files with bloom filters and column values
	p.messageBloomValues.bloom = &mp.messageBloomValues.bloom
	p.messageBloomValues.values = &mp.messageBloomValues.values

	p.bloomValuesShards = []bloomValuesReaderAt{
		{
			bloom:  &mp.fieldBloomValues.bloom,
			values: &mp.fieldBloomValues.values,
		},
	}

	p.marker = &marker{
		blocksCount: p.ph.BlocksCount,
	}

	p.appliedTSeq.Store(pt.getPendingAsyncTask().Seq)

	return &p
}

func mustOpenFilePart(pt *partition, path string) *part {
	var p part
	p.pt = pt
	p.path = path
	p.ph.mustReadMetadata(path)

	columnNamesPath := filepath.Join(path, columnNamesFilename)
	columnIdxsPath := filepath.Join(path, columnIdxsFilename)
	markerDatPath := filepath.Join(path, rowMarkerDatFilename)
	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderIndexPath := filepath.Join(path, columnsHeaderIndexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)

	// Read columnNames
	if p.ph.FormatVersion >= 1 {
		columnNamesReader := filestream.MustOpen(columnNamesPath, true)
		p.columnNames, p.columnNameIDs = mustReadColumnNames(columnNamesReader)
		columnNamesReader.MustClose()
	}
	if p.ph.FormatVersion >= 3 {
		columnIdxsReader := filestream.MustOpen(columnIdxsPath, true)
		p.columnIdxs = mustReadColumnIdxs(columnIdxsReader, p.columnNames, p.ph.BloomValuesShardsCount)
		columnIdxsReader.MustClose()
	}

	// Load marker data
	if fs.IsPathExist(markerDatPath) {
		markerDatReader := filestream.MustOpen(markerDatPath, true)
		p.marker = mustReadMarkerData(markerDatReader, p.ph.BlocksCount)
		markerDatReader.MustClose()
	} else {
		p.marker = nil
	}

	// Read metaindex
	metaindexReader := filestream.MustOpen(metaindexPath, true)
	var mrs readerWithStats
	mrs.init(metaindexReader)
	p.indexBlockHeaders = mustReadIndexBlockHeaders(p.indexBlockHeaders[:0], &mrs)
	mrs.MustClose()

	// Open data files
	p.indexFile = fs.MustOpenReaderAt(indexPath)
	if p.ph.FormatVersion >= 1 {
		p.columnsHeaderIndexFile = fs.MustOpenReaderAt(columnsHeaderIndexPath)
	}
	p.columnsHeaderFile = fs.MustOpenReaderAt(columnsHeaderPath)
	p.timestampsFile = fs.MustOpenReaderAt(timestampsPath)

	// Open files with bloom filters and column values
	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)
	p.messageBloomValues.bloom = fs.MustOpenReaderAt(messageBloomFilterPath)

	messageValuesPath := filepath.Join(path, messageValuesFilename)
	p.messageBloomValues.values = fs.MustOpenReaderAt(messageValuesPath)

	if p.ph.FormatVersion < 1 {
		bloomPath := filepath.Join(path, oldBloomFilename)
		p.oldBloomValues.bloom = fs.MustOpenReaderAt(bloomPath)

		valuesPath := filepath.Join(path, oldValuesFilename)
		p.oldBloomValues.values = fs.MustOpenReaderAt(valuesPath)
	} else {
		p.bloomValuesShards = make([]bloomValuesReaderAt, p.ph.BloomValuesShardsCount)
		for i := range p.bloomValuesShards {
			shard := &p.bloomValuesShards[i]

			bloomPath := getBloomFilePath(path, uint64(i))
			shard.bloom = fs.MustOpenReaderAt(bloomPath)

			valuesPath := getValuesFilePath(path, uint64(i))
			shard.values = fs.MustOpenReaderAt(valuesPath)
		}
	}

	// Load appliedTSeq from disk for this part.
	seq := mustReadAppliedTSeq(path)
	p.appliedTSeq.Store(seq)

	return &p
}

func mustClosePart(p *part) {
	p.indexFile.MustClose()
	if p.ph.FormatVersion >= 1 {
		p.columnsHeaderIndexFile.MustClose()
	}
	p.columnsHeaderFile.MustClose()
	p.timestampsFile.MustClose()
	p.messageBloomValues.MustClose()

	if p.ph.FormatVersion < 1 {
		p.oldBloomValues.MustClose()
	} else {
		for i := range p.bloomValuesShards {
			p.bloomValuesShards[i].MustClose()
		}
	}

	p.pt = nil
}

func (p *part) getBloomValuesFileForColumnName(name string) *bloomValuesReaderAt {
	if name == "" {
		return &p.messageBloomValues
	}

	if p.ph.FormatVersion < 1 {
		return &p.oldBloomValues
	}
	if p.ph.FormatVersion < 3 {
		n := len(p.bloomValuesShards)
		shardIdx := uint64(0)
		if n > 1 {
			h := xxhash.Sum64(bytesutil.ToUnsafeBytes(name))
			shardIdx = h % uint64(n)
		}
		return &p.bloomValuesShards[shardIdx]
	}

	shardIdx, ok := p.columnIdxs[name]
	if !ok {
		logger.Panicf("BUG: unknown shard index for column %q; columnIdxs=%v", name, p.columnIdxs)
	}
	return &p.bloomValuesShards[shardIdx]
}

func getBloomFilePath(partPath string, shardIdx uint64) string {
	return filepath.Join(partPath, bloomFilename) + fmt.Sprintf("%d", shardIdx)
}

func getValuesFilePath(partPath string, shardIdx uint64) string {
	return filepath.Join(partPath, valuesFilename) + fmt.Sprintf("%d", shardIdx)
}

// setAppliedTSeq updates the applied sequence for the part and persists it to disk
// if the part is file-based. Best-effort: on error it only logs and leaves the
// in-memory counter updated, so the worker will retry later.
func (p *part) setAppliedTSeq(seq uint64) {
	if p.appliedTSeq.Load() >= seq {
		return
	}
	p.appliedTSeq.Store(seq)

	if p.path == "" {
		return // in-memory part – nothing to persist
	}

	seqPath := filepath.Join(p.path, appliedTSeqFilename)
	if err := os.WriteFile(seqPath, fmt.Appendf(nil, "%d", seq), 0o644); err != nil {
		logger.Warnf("cannot write appliedTSeq to %q: %s", seqPath, err)
		return
	}
	fs.MustSyncPath(p.path)
}

func (p *part) isPayingAsyncTask() bool {
	seq, ok := p.pt.isPayingAsyncTask()
	return ok && p.appliedTSeq.Load() < seq
}
