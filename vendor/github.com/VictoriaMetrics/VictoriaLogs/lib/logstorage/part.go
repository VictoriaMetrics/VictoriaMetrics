package logstorage

import (
	"fmt"
	"path/filepath"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fsutil"
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
}

type bloomValuesReaderAt struct {
	bloom  fs.MustReadAtCloser
	values fs.MustReadAtCloser
}

func (r *bloomValuesReaderAt) appendCloserTasks(pe *fsutil.ParallelExecutor) {
	pe.Add(fs.NewCloserTask(r.bloom))
	pe.Add(fs.NewCloserTask(r.values))
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

	return &p
}

func mustOpenFilePart(pt *partition, path string) *part {
	var p part
	p.pt = pt
	p.path = path
	p.ph.mustReadMetadata(path)

	columnNamesPath := filepath.Join(path, columnNamesFilename)
	columnIdxsPath := filepath.Join(path, columnIdxsFilename)
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

	// Read metaindex
	metaindexReader := filestream.MustOpen(metaindexPath, true)
	var mrs readerWithStats
	mrs.init(metaindexReader)
	p.indexBlockHeaders = mustReadIndexBlockHeaders(p.indexBlockHeaders[:0], &mrs)
	mrs.MustClose()

	// Open data files
	p.indexFile = fs.OpenReaderAt(indexPath)
	if p.ph.FormatVersion >= 1 {
		p.columnsHeaderIndexFile = fs.OpenReaderAt(columnsHeaderIndexPath)
	}
	p.columnsHeaderFile = fs.OpenReaderAt(columnsHeaderPath)
	p.timestampsFile = fs.OpenReaderAt(timestampsPath)

	// Open files with bloom filters and column values
	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)
	p.messageBloomValues.bloom = fs.OpenReaderAt(messageBloomFilterPath)

	messageValuesPath := filepath.Join(path, messageValuesFilename)
	p.messageBloomValues.values = fs.OpenReaderAt(messageValuesPath)

	if p.ph.FormatVersion < 1 {
		bloomPath := filepath.Join(path, oldBloomFilename)
		p.oldBloomValues.bloom = fs.OpenReaderAt(bloomPath)

		valuesPath := filepath.Join(path, oldValuesFilename)
		p.oldBloomValues.values = fs.OpenReaderAt(valuesPath)
	} else {
		p.bloomValuesShards = make([]bloomValuesReaderAt, p.ph.BloomValuesShardsCount)
		for i := range p.bloomValuesShards {
			shard := &p.bloomValuesShards[i]

			bloomPath := getBloomFilePath(path, uint64(i))
			shard.bloom = fs.OpenReaderAt(bloomPath)

			valuesPath := getValuesFilePath(path, uint64(i))
			shard.values = fs.OpenReaderAt(valuesPath)
		}
	}

	return &p
}

func mustClosePart(p *part) {
	// Close files in parallel in order to speed up this operation
	// on high-latency storage systems such as NFS and Ceph.
	var pe fsutil.ParallelExecutor

	pe.Add(fs.NewCloserTask(p.indexFile))
	if p.ph.FormatVersion >= 1 {
		pe.Add(fs.NewCloserTask(p.columnsHeaderIndexFile))
	}
	pe.Add(fs.NewCloserTask(p.columnsHeaderFile))
	pe.Add(fs.NewCloserTask(p.timestampsFile))
	p.messageBloomValues.appendCloserTasks(&pe)

	if p.ph.FormatVersion < 1 {
		p.oldBloomValues.appendCloserTasks(&pe)
	} else {
		for i := range p.bloomValuesShards {
			p.bloomValuesShards[i].appendCloserTasks(&pe)
		}
	}

	pe.Run()

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
