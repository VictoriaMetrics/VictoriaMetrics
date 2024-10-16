package logstorage

import (
	"fmt"
	"path/filepath"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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

	// indexBlockHeaders contains a list of indexBlockHeader entries for the given part.
	indexBlockHeaders []indexBlockHeader

	indexFile              fs.MustReadAtCloser
	columnsHeaderIndexFile fs.MustReadAtCloser
	columnsHeaderFile      fs.MustReadAtCloser
	timestampsFile         fs.MustReadAtCloser

	messageBloomValues bloomValuesReaderAt
	oldBloomValues     bloomValuesReaderAt
	bloomValuesShards  [bloomValuesShardsCount]bloomValuesReaderAt
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

	// Read metaindex
	metaindexReader := mp.metaindex.NewReader()
	var mrs readerWithStats
	mrs.init(metaindexReader)
	p.indexBlockHeaders = mustReadIndexBlockHeaders(p.indexBlockHeaders[:0], &mrs)

	// Open data files
	p.indexFile = &mp.index
	p.columnsHeaderIndexFile = &mp.columnsHeaderIndex
	p.columnsHeaderFile = &mp.columnsHeader
	p.timestampsFile = &mp.timestamps

	p.messageBloomValues.bloom = &mp.messageBloomValues.bloom
	p.messageBloomValues.values = &mp.messageBloomValues.values

	// Open files with bloom filters and column values
	for i := range p.bloomValuesShards[:] {
		shard := &p.bloomValuesShards[i]
		shard.bloom = &mp.bloomValuesShards[i].bloom
		shard.values = &mp.bloomValuesShards[i].values
	}

	return &p
}

func mustOpenFilePart(pt *partition, path string) *part {
	var p part
	p.pt = pt
	p.path = path
	p.ph.mustReadMetadata(path)

	columnNamesPath := filepath.Join(path, columnNamesFilename)
	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderIndexPath := filepath.Join(path, columnsHeaderIndexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)

	// Read columnNames
	if p.ph.FormatVersion >= 1 {
		columnNamesReader := filestream.MustOpen(columnNamesPath, true)
		var crs readerWithStats
		crs.init(columnNamesReader)
		p.columnNames, p.columnNameIDs = mustReadColumnNames(columnNamesReader)
		crs.MustClose()
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
		for i := range p.bloomValuesShards[:] {
			shard := &p.bloomValuesShards[i]

			bloomPath := getBloomFilePath(path, uint64(i))
			shard.bloom = fs.MustOpenReaderAt(bloomPath)

			valuesPath := getValuesFilePath(path, uint64(i))
			shard.values = fs.MustOpenReaderAt(valuesPath)
		}
	}

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
		for i := range p.bloomValuesShards[:] {
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

	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(name))
	idx := h % uint64(len(p.bloomValuesShards))
	return &p.bloomValuesShards[idx]
}

func getBloomFilePath(partPath string, shardNum uint64) string {
	return filepath.Join(partPath, bloomFilename) + fmt.Sprintf("%d", shardNum)
}

func getValuesFilePath(partPath string, shardNum uint64) string {
	return filepath.Join(partPath, valuesFilename) + fmt.Sprintf("%d", shardNum)
}
