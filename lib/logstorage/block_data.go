package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// blockData contains packed data for a single block.
//
// The main purpose of this struct is to reduce the work needed during background merge of parts.
// If the block is full, then the blockData can be written to the destination part
// without the need to unpack it.
type blockData struct {
	// streamID is id of the stream for the data
	streamID streamID

	// uncompressedSizeBytes is the original (uncompressed) size of log entries stored in the block
	uncompressedSizeBytes uint64

	// rowsCount is the number of log entries in the block
	rowsCount uint64

	// timestampsData contains the encoded timestamps data for the block
	timestampsData timestampsData

	// columnsData contains packed per-column data
	columnsData []columnData

	// constColumns contains data for const columns across the block
	constColumns []Field

	// a is used for storing byte slices for timestamps and columns.
	//
	// It reduces fragmentation for them.
	a arena
}

// reset resets bd for subsequent re-use
func (bd *blockData) reset() {
	bd.streamID.reset()
	bd.uncompressedSizeBytes = 0
	bd.rowsCount = 0
	bd.timestampsData.reset()

	cds := bd.columnsData
	for i := range cds {
		cds[i].reset()
	}
	bd.columnsData = cds[:0]

	ccs := bd.constColumns
	for i := range ccs {
		ccs[i].Reset()
	}
	bd.constColumns = ccs[:0]

	bd.a.reset()
}

func (bd *blockData) resizeColumnsData(columnsDataLen int) []columnData {
	cds := bd.columnsData
	if n := columnsDataLen - cap(cds); n > 0 {
		cds = append(cds[:cap(cds)], make([]columnData, n)...)
	}
	cds = cds[:columnsDataLen]
	bd.columnsData = cds
	return cds
}

// copyFrom copies src to bd.
func (bd *blockData) copyFrom(src *blockData) {
	bd.reset()

	bd.streamID = src.streamID
	bd.uncompressedSizeBytes = src.uncompressedSizeBytes
	bd.rowsCount = src.rowsCount
	bd.timestampsData.copyFrom(&src.timestampsData, &bd.a)

	cdsSrc := src.columnsData
	cds := bd.resizeColumnsData(len(cdsSrc))
	for i := range cds {
		cds[i].copyFrom(&cdsSrc[i], &bd.a)
	}
	bd.columnsData = cds

	bd.constColumns = append(bd.constColumns[:0], src.constColumns...)
}

// unmarshalRows appends unmarshaled from bd log entries to dst.
//
// The returned log entries are valid until sbu and vd are valid.
func (bd *blockData) unmarshalRows(dst *rows, sbu *stringsBlockUnmarshaler, vd *valuesDecoder) error {
	b := getBlock()
	defer putBlock(b)

	if err := b.InitFromBlockData(bd, sbu, vd); err != nil {
		return err
	}
	b.appendRowsTo(dst)
	return nil
}

// mustWriteTo writes bd with the given sid to sw and updates bh accordingly
func (bd *blockData) mustWriteTo(bh *blockHeader, sw *streamWriters) {
	// Do not store the version used for encoding directly in the block data, since:
	// - all the blocks in the same part use the same encoding
	// - the block encoding version can be put in metadata file for the part (aka metadataFilename)

	bh.reset()

	bh.streamID = bd.streamID
	bh.uncompressedSizeBytes = bd.uncompressedSizeBytes
	bh.rowsCount = bd.rowsCount

	// Marshal timestamps
	bd.timestampsData.mustWriteTo(&bh.timestampsHeader, sw)

	// Marshal columns
	cds := bd.columnsData
	csh := getColumnsHeader()
	chs := csh.resizeColumnHeaders(len(cds))
	for i := range cds {
		cds[i].mustWriteTo(&chs[i], sw)
	}
	csh.constColumns = append(csh.constColumns[:0], bd.constColumns...)

	bb := longTermBufPool.Get()
	bb.B = csh.marshal(bb.B)
	putColumnsHeader(csh)
	bh.columnsHeaderOffset = sw.columnsHeaderWriter.bytesWritten
	bh.columnsHeaderSize = uint64(len(bb.B))
	if bh.columnsHeaderSize > maxColumnsHeaderSize {
		logger.Panicf("BUG: too big columnsHeaderSize: %d bytes; mustn't exceed %d bytes", bh.columnsHeaderSize, maxColumnsHeaderSize)
	}
	sw.columnsHeaderWriter.MustWrite(bb.B)
	longTermBufPool.Put(bb)
}

// mustReadFrom reads block data associated with bh from sr to bd.
func (bd *blockData) mustReadFrom(bh *blockHeader, sr *streamReaders) {
	bd.reset()

	bd.streamID = bh.streamID
	bd.uncompressedSizeBytes = bh.uncompressedSizeBytes
	bd.rowsCount = bh.rowsCount

	// Read timestamps
	bd.timestampsData.mustReadFrom(&bh.timestampsHeader, sr, &bd.a)

	// Read columns
	if bh.columnsHeaderOffset != sr.columnsHeaderReader.bytesRead {
		logger.Panicf("FATAL: %s: unexpected columnsHeaderOffset=%d; must equal to the number of bytes read: %d",
			sr.columnsHeaderReader.Path(), bh.columnsHeaderOffset, sr.columnsHeaderReader.bytesRead)
	}
	columnsHeaderSize := bh.columnsHeaderSize
	if columnsHeaderSize > maxColumnsHeaderSize {
		logger.Panicf("BUG: %s: too big columnsHeaderSize: %d bytes; mustn't exceed %d bytes", sr.columnsHeaderReader.Path(), columnsHeaderSize, maxColumnsHeaderSize)
	}
	bb := longTermBufPool.Get()
	bb.B = bytesutil.ResizeNoCopyMayOverallocate(bb.B, int(columnsHeaderSize))
	sr.columnsHeaderReader.MustReadFull(bb.B)

	csh := getColumnsHeader()
	if err := csh.unmarshal(bb.B); err != nil {
		logger.Panicf("FATAL: %s: cannot unmarshal columnsHeader: %s", sr.columnsHeaderReader.Path(), err)
	}
	longTermBufPool.Put(bb)
	chs := csh.columnHeaders
	cds := bd.resizeColumnsData(len(chs))
	for i := range chs {
		cds[i].mustReadFrom(&chs[i], sr, &bd.a)
	}
	bd.constColumns = append(bd.constColumns[:0], csh.constColumns...)
	putColumnsHeader(csh)
}

// timestampsData contains the encoded timestamps data.
type timestampsData struct {
	// data contains packed timestamps data.
	data []byte

	// marshalType is the marshal type for timestamps
	marshalType encoding.MarshalType

	// minTimestamp is the minimum timestamp in the timestamps data
	minTimestamp int64

	// maxTimestamp is the maximum timestamp in the timestamps data
	maxTimestamp int64
}

// reset resets td for subsequent re-use
func (td *timestampsData) reset() {
	td.data = nil
	td.marshalType = 0
	td.minTimestamp = 0
	td.maxTimestamp = 0
}

// copyFrom copies src to td.
func (td *timestampsData) copyFrom(src *timestampsData, a *arena) {
	td.reset()

	td.data = a.copyBytes(src.data)
	td.marshalType = src.marshalType
	td.minTimestamp = src.minTimestamp
	td.maxTimestamp = src.maxTimestamp
}

// mustWriteTo writes td to sw and updates th accordingly
func (td *timestampsData) mustWriteTo(th *timestampsHeader, sw *streamWriters) {
	th.reset()

	th.marshalType = td.marshalType
	th.minTimestamp = td.minTimestamp
	th.maxTimestamp = td.maxTimestamp
	th.blockOffset = sw.timestampsWriter.bytesWritten
	th.blockSize = uint64(len(td.data))
	if th.blockSize > maxTimestampsBlockSize {
		logger.Panicf("BUG: too big timestampsHeader.blockSize: %d bytes; mustn't exceed %d bytes", th.blockSize, maxTimestampsBlockSize)
	}
	sw.timestampsWriter.MustWrite(td.data)
}

// mustReadFrom reads timestamps data associated with th from sr to td.
func (td *timestampsData) mustReadFrom(th *timestampsHeader, sr *streamReaders, a *arena) {
	td.reset()

	td.marshalType = th.marshalType
	td.minTimestamp = th.minTimestamp
	td.maxTimestamp = th.maxTimestamp

	timestampsReader := &sr.timestampsReader
	if th.blockOffset != timestampsReader.bytesRead {
		logger.Panicf("FATAL: %s: unexpected timestampsHeader.blockOffset=%d; must equal to the number of bytes read: %d",
			timestampsReader.Path(), th.blockOffset, timestampsReader.bytesRead)
	}
	timestampsBlockSize := th.blockSize
	if timestampsBlockSize > maxTimestampsBlockSize {
		logger.Panicf("FATAL: %s: too big timestamps block with %d bytes; the maximum supported block size is %d bytes",
			timestampsReader.Path(), timestampsBlockSize, maxTimestampsBlockSize)
	}
	td.data = a.newBytes(int(timestampsBlockSize))
	timestampsReader.MustReadFull(td.data)
}

// columnData contains packed data for a single column.
type columnData struct {
	// name is the column name
	name string

	// valueType is the type of values stored in valuesData
	valueType valueType

	// minValue is the minimum encoded uint* or float64 value in the columnHeader
	//
	// It is used for fast detection of whether the given columnHeader contains values in the given range
	minValue uint64

	// maxValue is the maximum encoded uint* or float64 value in the columnHeader
	//
	// It is used for fast detection of whether the given columnHeader contains values in the given range
	maxValue uint64

	// valuesDict contains unique values for valueType = valueTypeDict
	valuesDict valuesDict

	// valuesData contains packed values data for the given column
	valuesData []byte

	// bloomFilterData contains packed bloomFilter data for the given column
	bloomFilterData []byte
}

// reset rests cd for subsequent re-use
func (cd *columnData) reset() {
	cd.name = ""
	cd.valueType = 0

	cd.minValue = 0
	cd.maxValue = 0
	cd.valuesDict.reset()

	cd.valuesData = nil
	cd.bloomFilterData = nil
}

// copyFrom copies src to cd.
func (cd *columnData) copyFrom(src *columnData, a *arena) {
	cd.reset()

	cd.name = src.name
	cd.valueType = src.valueType

	cd.minValue = src.minValue
	cd.maxValue = src.maxValue
	cd.valuesDict.copyFrom(&src.valuesDict)

	cd.valuesData = a.copyBytes(src.valuesData)
	cd.bloomFilterData = a.copyBytes(src.bloomFilterData)
}

// mustWriteTo writes cd to sw and updates ch accordingly.
func (cd *columnData) mustWriteTo(ch *columnHeader, sw *streamWriters) {
	ch.reset()

	valuesWriter := &sw.fieldValuesWriter
	bloomFilterWriter := &sw.fieldBloomFilterWriter
	if cd.name == "" {
		valuesWriter = &sw.messageValuesWriter
		bloomFilterWriter = &sw.messageBloomFilterWriter
	}

	ch.name = cd.name
	ch.valueType = cd.valueType

	ch.minValue = cd.minValue
	ch.maxValue = cd.maxValue
	ch.valuesDict.copyFrom(&cd.valuesDict)

	// marshal values
	ch.valuesSize = uint64(len(cd.valuesData))
	if ch.valuesSize > maxValuesBlockSize {
		logger.Panicf("BUG: too big valuesSize: %d bytes; mustn't exceed %d bytes", ch.valuesSize, maxValuesBlockSize)
	}
	ch.valuesOffset = valuesWriter.bytesWritten
	valuesWriter.MustWrite(cd.valuesData)

	// marshal bloom filter
	ch.bloomFilterSize = uint64(len(cd.bloomFilterData))
	if ch.bloomFilterSize > maxBloomFilterBlockSize {
		logger.Panicf("BUG: too big bloomFilterSize: %d bytes; mustn't exceed %d bytes", ch.bloomFilterSize, maxBloomFilterBlockSize)
	}
	ch.bloomFilterOffset = bloomFilterWriter.bytesWritten
	bloomFilterWriter.MustWrite(cd.bloomFilterData)
}

// mustReadFrom reads columns data associated with ch from sr to cd.
func (cd *columnData) mustReadFrom(ch *columnHeader, sr *streamReaders, a *arena) {
	cd.reset()

	valuesReader := &sr.fieldValuesReader
	bloomFilterReader := &sr.fieldBloomFilterReader
	if ch.name == "" {
		valuesReader = &sr.messageValuesReader
		bloomFilterReader = &sr.messageBloomFilterReader
	}

	cd.name = ch.name
	cd.valueType = ch.valueType

	cd.minValue = ch.minValue
	cd.maxValue = ch.maxValue
	cd.valuesDict.copyFrom(&ch.valuesDict)

	// read values
	if ch.valuesOffset != valuesReader.bytesRead {
		logger.Panicf("FATAL: %s: unexpected columnHeader.valuesOffset=%d; must equal to the number of bytes read: %d",
			valuesReader.Path(), ch.valuesOffset, valuesReader.bytesRead)
	}
	valuesSize := ch.valuesSize
	if valuesSize > maxValuesBlockSize {
		logger.Panicf("FATAL: %s: values block size cannot exceed %d bytes; got %d bytes", valuesReader.Path(), maxValuesBlockSize, valuesSize)
	}
	cd.valuesData = a.newBytes(int(valuesSize))
	valuesReader.MustReadFull(cd.valuesData)

	// read bloom filter
	// bloom filter is missing in valueTypeDict.
	if ch.valueType != valueTypeDict {
		if ch.bloomFilterOffset != bloomFilterReader.bytesRead {
			logger.Panicf("FATAL: %s: unexpected columnHeader.bloomFilterOffset=%d; must equal to the number of bytes read: %d",
				bloomFilterReader.Path(), ch.bloomFilterOffset, bloomFilterReader.bytesRead)
		}
		bloomFilterSize := ch.bloomFilterSize
		if bloomFilterSize > maxBloomFilterBlockSize {
			logger.Panicf("FATAL: %s: bloom filter block size cannot exceed %d bytes; got %d bytes", bloomFilterReader.Path(), maxBloomFilterBlockSize, bloomFilterSize)
		}
		cd.bloomFilterData = a.newBytes(int(bloomFilterSize))
		bloomFilterReader.MustReadFull(cd.bloomFilterData)
	}
}
