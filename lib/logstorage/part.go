package logstorage

import (
	"path/filepath"

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

	// indexBlockHeaders contains a list of indexBlockHeader entries for the given part.
	indexBlockHeaders []indexBlockHeader

	indexFile              fs.MustReadAtCloser
	columnsHeaderFile      fs.MustReadAtCloser
	timestampsFile         fs.MustReadAtCloser
	fieldValuesFile        fs.MustReadAtCloser
	fieldBloomFilterFile   fs.MustReadAtCloser
	messageValuesFile      fs.MustReadAtCloser
	messageBloomFilterFile fs.MustReadAtCloser
}

func mustOpenInmemoryPart(pt *partition, mp *inmemoryPart) *part {
	var p part
	p.pt = pt
	p.path = ""
	p.ph = mp.ph

	// Read metaindex
	metaindexReader := mp.metaindex.NewReader()
	var mrs readerWithStats
	mrs.init(metaindexReader)
	p.indexBlockHeaders = mustReadIndexBlockHeaders(p.indexBlockHeaders[:0], &mrs)

	// Open data files
	p.indexFile = &mp.index
	p.columnsHeaderFile = &mp.columnsHeader
	p.timestampsFile = &mp.timestamps
	p.fieldValuesFile = &mp.fieldValues
	p.fieldBloomFilterFile = &mp.fieldBloomFilter
	p.messageValuesFile = &mp.messageValues
	p.messageBloomFilterFile = &mp.messageBloomFilter

	return &p
}

func mustOpenFilePart(pt *partition, path string) *part {
	var p part
	p.pt = pt
	p.path = path
	p.ph.mustReadMetadata(path)

	metaindexPath := filepath.Join(path, metaindexFilename)
	indexPath := filepath.Join(path, indexFilename)
	columnsHeaderPath := filepath.Join(path, columnsHeaderFilename)
	timestampsPath := filepath.Join(path, timestampsFilename)
	fieldValuesPath := filepath.Join(path, fieldValuesFilename)
	fieldBloomFilterPath := filepath.Join(path, fieldBloomFilename)
	messageValuesPath := filepath.Join(path, messageValuesFilename)
	messageBloomFilterPath := filepath.Join(path, messageBloomFilename)

	// Read metaindex
	metaindexReader := filestream.MustOpen(metaindexPath, true)
	var mrs readerWithStats
	mrs.init(metaindexReader)
	p.indexBlockHeaders = mustReadIndexBlockHeaders(p.indexBlockHeaders[:0], &mrs)
	mrs.MustClose()

	// Open data files
	p.indexFile = fs.MustOpenReaderAt(indexPath)
	p.columnsHeaderFile = fs.MustOpenReaderAt(columnsHeaderPath)
	p.timestampsFile = fs.MustOpenReaderAt(timestampsPath)
	p.fieldValuesFile = fs.MustOpenReaderAt(fieldValuesPath)
	p.fieldBloomFilterFile = fs.MustOpenReaderAt(fieldBloomFilterPath)
	p.messageValuesFile = fs.MustOpenReaderAt(messageValuesPath)
	p.messageBloomFilterFile = fs.MustOpenReaderAt(messageBloomFilterPath)

	return &p
}

func mustClosePart(p *part) {
	p.indexFile.MustClose()
	p.columnsHeaderFile.MustClose()
	p.timestampsFile.MustClose()
	p.fieldValuesFile.MustClose()
	p.fieldBloomFilterFile.MustClose()
	p.messageValuesFile.MustClose()
	p.messageBloomFilterFile.MustClose()

	p.pt = nil
}
