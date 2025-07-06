package logstorage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// The number of blocks to search at once by a single worker
//
// This number must be increased on systems with many CPU cores in order to amortize
// the overhead for passing the blockSearchWork to worker goroutines.
const blockSearchWorksPerBatch = 64

type blockSearchWork struct {
	// p is the part where the block belongs to.
	p *part

	// so contains search options for the block search.
	so *searchOptions

	// bh is the header of the block to search.
	bh blockHeader
}

func (bsw *blockSearchWork) reset() {
	bsw.p = nil
	bsw.so = nil
	bsw.bh.reset()
}

type blockSearchWorkBatch struct {
	bsws []blockSearchWork
}

func (bswb *blockSearchWorkBatch) reset() {
	bsws := bswb.bsws
	for i := range bsws {
		bsws[i].reset()
	}
	bswb.bsws = bsws[:0]
}

func getBlockSearchWorkBatch() *blockSearchWorkBatch {
	v := blockSearchWorkBatchPool.Get()
	if v == nil {
		return &blockSearchWorkBatch{
			bsws: make([]blockSearchWork, 0, blockSearchWorksPerBatch),
		}
	}
	return v.(*blockSearchWorkBatch)
}

func putBlockSearchWorkBatch(bswb *blockSearchWorkBatch) {
	bswb.reset()
	blockSearchWorkBatchPool.Put(bswb)
}

var blockSearchWorkBatchPool sync.Pool

func (bswb *blockSearchWorkBatch) appendBlockSearchWork(p *part, so *searchOptions, bh *blockHeader) bool {
	bsws := bswb.bsws

	bsws = append(bsws, blockSearchWork{
		p:  p,
		so: so,
	})
	bsw := &bsws[len(bsws)-1]
	bsw.bh.copyFrom(bh)

	bswb.bsws = bsws

	return len(bsws) < cap(bsws)
}

func getBlockSearch() *blockSearch {
	v := blockSearchPool.Get()
	if v == nil {
		return &blockSearch{}
	}
	return v.(*blockSearch)
}

func putBlockSearch(bs *blockSearch) {
	bs.reset()

	// reset seenStreams before returning bs to the pool in order to reduce memory usage.
	bs.seenStreams = nil

	blockSearchPool.Put(bs)
}

var blockSearchPool sync.Pool

type blockSearch struct {
	// bsw is the actual work to perform on the given block pointed by bsw.ph
	bsw *blockSearchWork

	// br contains result for the search in the block after search() call
	br blockResult

	// timestampsCache contains cached timestamps for the given block.
	timestampsCache *encoding.Int64s

	// bloomFilterCache contains cached bloom filters for requested columns in the given block
	bloomFilterCache map[string]*bloomFilter

	// valuesCache contains cached values for requested columns in the given block
	valuesCache map[string]*stringBucket

	// sbu is used for unmarshaling local columns
	sbu stringsBlockUnmarshaler

	// cshIndexBlockCache holds columnsHeaderIndex data for the given block.
	//
	// It is initialized lazily by calling getColumnsHeaderIndex().
	cshIndexBlockCache []byte

	// cshBlockCache holds columnsHeader data for the given block.
	//
	// It is initialized lazily by calling getColumnsHeaderBlock().
	cshBlockCache       []byte
	cshBlockInitialized bool

	// ccsCache is the cache for accessed const columns
	ccsCache []Field

	// chsCache is the cache for accessed column headers
	chsCache []columnHeader

	// cshIndexCache is the columnsHeaderIndex associated with the given block.
	//
	// It is initialized lazily by calling getColumnsHeaderIndex().
	cshIndexCache *columnsHeaderIndex

	// cshCache is the columnsHeader associated with the given block.
	//
	// It is initialized lazily by calling getColumnsHeader().
	cshCache *columnsHeader

	// seenStreams contains seen streamIDs for the recent searches.
	//
	// It is used for speeding up fetching _stream column.
	seenStreams map[u128]string
}

func (bs *blockSearch) reset() {
	bs.bsw = nil
	bs.br.reset()

	if bs.timestampsCache != nil {
		encoding.PutInt64s(bs.timestampsCache)
		bs.timestampsCache = nil
	}

	bloomFilterCache := bs.bloomFilterCache
	for k, bf := range bloomFilterCache {
		putBloomFilter(bf)
		delete(bloomFilterCache, k)
	}

	valuesCache := bs.valuesCache
	for k, values := range valuesCache {
		putStringBucket(values)
		delete(valuesCache, k)
	}

	bs.sbu.reset()

	bs.cshIndexBlockCache = bs.cshIndexBlockCache[:0]

	bs.cshBlockCache = bs.cshBlockCache[:0]
	bs.cshBlockInitialized = false

	ccsCache := bs.ccsCache
	for i := range ccsCache {
		ccsCache[i].Reset()
	}
	bs.ccsCache = ccsCache[:0]

	chsCache := bs.chsCache
	for i := range chsCache {
		chsCache[i].reset()
	}
	bs.chsCache = chsCache[:0]

	if bs.cshIndexCache != nil {
		putColumnsHeaderIndex(bs.cshIndexCache)
		bs.cshIndexCache = nil
	}

	if bs.cshCache != nil {
		putColumnsHeader(bs.cshCache)
		bs.cshCache = nil
	}

	// Do not reset seenStreams, since its' lifetime is managed by blockResult.addStreamColumn() code.
}

func (bs *blockSearch) partPath() string {
	return bs.bsw.p.path
}

func (bs *blockSearch) search(bsw *blockSearchWork, bm *bitmap) {
	bs.reset()

	bs.bsw = bsw

	// search rows matching the given filter
	bm.init(int(bsw.bh.rowsCount))
	bm.setBits()
	bs.bsw.so.filter.applyToBlockSearch(bs, bm)

	if bm.isZero() {
		// The filter doesn't match any logs in the current block.
		return
	}

	bs.br.mustInit(bs, bm)

	// fetch the requested columns to bs.br.
	bs.br.initColumns(bsw.so.fieldsFilter)
}

func (bs *blockSearch) partFormatVersion() uint {
	return bs.bsw.p.ph.FormatVersion
}

func (bs *blockSearch) getConstColumnValue(name string) string {
	name = getCanonicalFieldName(name)

	if bs.partFormatVersion() < 1 {
		csh := bs.getColumnsHeader()
		for _, cc := range csh.constColumns {
			if cc.Name == name {
				return cc.Value
			}
		}
		return ""
	}

	columnNameID, ok := bs.getColumnNameID(name)
	if !ok {
		return ""
	}

	for i := range bs.ccsCache {
		if bs.ccsCache[i].Name == name {
			return bs.ccsCache[i].Value
		}
	}

	cshIndex := bs.getColumnsHeaderIndex()
	for _, cr := range cshIndex.constColumnsRefs {
		if cr.columnNameID != columnNameID {
			continue
		}

		b := bs.getColumnsHeaderBlock()
		if cr.offset > uint64(len(b)) {
			logger.Panicf("FATAL: %s: header offset for const column %q cannot exceed %d bytes; got %d bytes", bs.bsw.p.path, name, len(b), cr.offset)
		}
		b = b[cr.offset:]
		bs.ccsCache = slicesutil.SetLength(bs.ccsCache, len(bs.ccsCache)+1)
		cc := &bs.ccsCache[len(bs.ccsCache)-1]
		if _, err := cc.unmarshalInplace(b, false); err != nil {
			logger.Panicf("FATAL: %s: cannot unmarshal header for const column %q: %s", bs.bsw.p.path, name, err)
		}
		cc.Name = bs.getColumnNameByID(columnNameID)
		return cc.Value
	}
	return ""
}

func (bs *blockSearch) getColumnHeader(name string) *columnHeader {
	name = getCanonicalFieldName(name)

	if bs.partFormatVersion() < 1 {
		csh := bs.getColumnsHeader()
		chs := csh.columnHeaders
		for i := range chs {
			ch := &chs[i]
			if ch.name == name {
				return ch
			}
		}
		return nil
	}

	columnNameID, ok := bs.getColumnNameID(name)
	if !ok {
		return nil
	}

	for i := range bs.chsCache {
		if bs.chsCache[i].name == name {
			return &bs.chsCache[i]
		}
	}

	cshIndex := bs.getColumnsHeaderIndex()
	for _, cr := range cshIndex.columnHeadersRefs {
		if cr.columnNameID != columnNameID {
			continue
		}

		b := bs.getColumnsHeaderBlock()
		if cr.offset > uint64(len(b)) {
			logger.Panicf("FATAL: %s: header offset for column %q cannot exceed %d bytes; got %d bytes", bs.bsw.p.path, name, len(b), cr.offset)
		}
		b = b[cr.offset:]
		bs.chsCache = slicesutil.SetLength(bs.chsCache, len(bs.chsCache)+1)
		ch := &bs.chsCache[len(bs.chsCache)-1]
		if _, err := ch.unmarshalInplace(b, partFormatLatestVersion); err != nil {
			logger.Panicf("FATAL: %s: cannot unmarshal header for column %q: %s", bs.bsw.p.path, name, err)
		}
		ch.name = bs.getColumnNameByID(columnNameID)
		return ch
	}
	return nil
}

func (bs *blockSearch) getColumnNameID(name string) (uint64, bool) {
	id, ok := bs.bsw.p.columnNameIDs[name]
	return id, ok
}

func (bs *blockSearch) getColumnNameByID(columnNameID uint64) string {
	columnNames := bs.bsw.p.columnNames
	if columnNameID >= uint64(len(columnNames)) {
		logger.Panicf("FATAL: %s: too big columnNameID=%d; it must be smaller than %d", bs.bsw.p.path, columnNameID, len(columnNames))
	}
	return columnNames[columnNameID]
}

func (bs *blockSearch) getColumnsHeaderIndex() *columnsHeaderIndex {
	if bs.partFormatVersion() < 1 {
		logger.Panicf("BUG: getColumnsHeaderIndex() can be called only for part encoding v1+, while it has been called for v%d", bs.partFormatVersion())
	}

	if bs.cshIndexCache == nil {
		bs.cshIndexBlockCache = readColumnsHeaderIndexBlock(bs.cshIndexBlockCache[:0], bs.bsw.p, &bs.bsw.bh)

		bs.cshIndexCache = getColumnsHeaderIndex()
		if err := bs.cshIndexCache.unmarshalInplace(bs.cshIndexBlockCache); err != nil {
			logger.Panicf("FATAL: %s: cannot unmarshal columns header index: %s", bs.bsw.p.path, err)
		}
	}
	return bs.cshIndexCache
}

func (bs *blockSearch) getColumnsHeader() *columnsHeader {
	if bs.cshCache == nil {
		b := bs.getColumnsHeaderBlock()

		csh := getColumnsHeader()
		partFormatVersion := bs.partFormatVersion()
		if err := csh.unmarshalInplace(b, partFormatVersion); err != nil {
			logger.Panicf("FATAL: %s: cannot unmarshal columns header: %s", bs.bsw.p.path, err)
		}
		if partFormatVersion >= 1 {
			cshIndex := bs.getColumnsHeaderIndex()
			if err := csh.setColumnNames(cshIndex, bs.bsw.p.columnNames); err != nil {
				logger.Panicf("FATAL: %s: %s", bs.bsw.p.path, err)
			}
		}

		bs.cshCache = csh
	}
	return bs.cshCache
}

func (bs *blockSearch) getColumnsHeaderBlock() []byte {
	if !bs.cshBlockInitialized {
		bs.cshBlockCache = readColumnsHeaderBlock(bs.cshBlockCache[:0], bs.bsw.p, &bs.bsw.bh)
		bs.cshBlockInitialized = true
	}
	return bs.cshBlockCache
}

func readColumnsHeaderIndexBlock(dst []byte, p *part, bh *blockHeader) []byte {
	n := bh.columnsHeaderIndexSize
	if n > maxColumnsHeaderIndexSize {
		logger.Panicf("FATAL: %s: columns header index size cannot exceed %d bytes; got %d bytes", p.path, maxColumnsHeaderIndexSize, n)
	}

	dstLen := len(dst)
	dst = bytesutil.ResizeNoCopyMayOverallocate(dst, int(n)+dstLen)
	p.columnsHeaderIndexFile.MustReadAt(dst[dstLen:], int64(bh.columnsHeaderIndexOffset))

	return dst
}

func readColumnsHeaderBlock(dst []byte, p *part, bh *blockHeader) []byte {
	n := bh.columnsHeaderSize
	if n > maxColumnsHeaderSize {
		logger.Panicf("FATAL: %s: columns header size cannot exceed %d bytes; got %d bytes", p.path, maxColumnsHeaderSize, n)
	}
	dstLen := len(dst)
	dst = bytesutil.ResizeNoCopyMayOverallocate(dst, int(n)+dstLen)
	p.columnsHeaderFile.MustReadAt(dst[dstLen:], int64(bh.columnsHeaderOffset))
	return dst
}

// getBloomFilterForColumn returns bloom filter for the given ch.
//
// The returned bloom filter belongs to bs, so it becomes invalid after bs reset.
func (bs *blockSearch) getBloomFilterForColumn(ch *columnHeader) *bloomFilter {
	bf := bs.bloomFilterCache[ch.name]
	if bf != nil {
		return bf
	}

	p := bs.bsw.p
	bloomValuesFile := p.getBloomValuesFileForColumnName(ch.name)

	bb := longTermBufPool.Get()
	bloomFilterSize := ch.bloomFilterSize
	if bloomFilterSize > maxBloomFilterBlockSize {
		logger.Panicf("FATAL: %s: bloom filter block size cannot exceed %d bytes; got %d bytes", bs.partPath(), maxBloomFilterBlockSize, bloomFilterSize)
	}
	bb.B = bytesutil.ResizeNoCopyMayOverallocate(bb.B, int(bloomFilterSize))

	bloomValuesFile.bloom.MustReadAt(bb.B, int64(ch.bloomFilterOffset))
	bf = getBloomFilter()
	if err := bf.unmarshal(bb.B); err != nil {
		logger.Panicf("FATAL: %s: cannot unmarshal bloom filter: %s", bs.partPath(), err)
	}
	longTermBufPool.Put(bb)

	if bs.bloomFilterCache == nil {
		bs.bloomFilterCache = make(map[string]*bloomFilter)
	}
	bs.bloomFilterCache[ch.name] = bf
	return bf
}

// getValuesForColumn returns block values for the given ch.
//
// The returned values belong to bs, so they become invalid after bs reset.
func (bs *blockSearch) getValuesForColumn(ch *columnHeader) []string {
	values := bs.valuesCache[ch.name]
	if values != nil {
		return values.a
	}

	p := bs.bsw.p
	bloomValuesFile := p.getBloomValuesFileForColumnName(ch.name)

	bb := longTermBufPool.Get()
	valuesSize := ch.valuesSize
	if valuesSize > maxValuesBlockSize {
		logger.Panicf("FATAL: %s: values block size cannot exceed %d bytes; got %d bytes", bs.partPath(), maxValuesBlockSize, valuesSize)
	}
	bb.B = bytesutil.ResizeNoCopyMayOverallocate(bb.B, int(valuesSize))
	bloomValuesFile.values.MustReadAt(bb.B, int64(ch.valuesOffset))

	values = getStringBucket()
	var err error
	values.a, err = bs.sbu.unmarshal(values.a[:0], bb.B, bs.bsw.bh.rowsCount)
	longTermBufPool.Put(bb)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot unmarshal column %q: %s", bs.partPath(), ch.name, err)
	}

	if bs.valuesCache == nil {
		bs.valuesCache = make(map[string]*stringBucket)
	}
	bs.valuesCache[ch.name] = values
	return values.a
}

// getTimestamps returns timestamps for the given bs.
//
// The returned timestamps belong to bs, so they become invalid after bs reset.
func (bs *blockSearch) getTimestamps() []int64 {
	timestamps := bs.timestampsCache
	if timestamps != nil {
		return timestamps.A
	}

	p := bs.bsw.p

	bb := longTermBufPool.Get()
	th := &bs.bsw.bh.timestampsHeader
	blockSize := th.blockSize
	if blockSize > maxTimestampsBlockSize {
		logger.Panicf("FATAL: %s: timestamps block size cannot exceed %d bytes; got %d bytes", bs.partPath(), maxTimestampsBlockSize, blockSize)
	}
	bb.B = bytesutil.ResizeNoCopyMayOverallocate(bb.B, int(blockSize))
	p.timestampsFile.MustReadAt(bb.B, int64(th.blockOffset))

	rowsCount := int(bs.bsw.bh.rowsCount)
	timestamps = encoding.GetInt64s(rowsCount)
	var err error
	timestamps.A, err = encoding.UnmarshalTimestamps(timestamps.A[:0], bb.B, th.marshalType, th.minTimestamp, rowsCount)
	longTermBufPool.Put(bb)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot unmarshal timestamps: %s", bs.partPath(), err)
	}
	bs.timestampsCache = timestamps
	return timestamps.A
}

// mustReadBlockHeaders reads ih block headers from p, appends them to dst and returns the result.
func (ih *indexBlockHeader) mustReadBlockHeaders(dst []blockHeader, p *part) []blockHeader {
	bbCompressed := longTermBufPool.Get()
	indexBlockSize := ih.indexBlockSize
	if indexBlockSize > maxIndexBlockSize {
		logger.Panicf("FATAL: %s: index block size cannot exceed %d bytes; got %d bytes", p.indexFile.Path(), maxIndexBlockSize, indexBlockSize)
	}
	bbCompressed.B = bytesutil.ResizeNoCopyMayOverallocate(bbCompressed.B, int(indexBlockSize))
	p.indexFile.MustReadAt(bbCompressed.B, int64(ih.indexBlockOffset))

	bb := longTermBufPool.Get()
	var err error
	bb.B, err = encoding.DecompressZSTD(bb.B, bbCompressed.B)
	longTermBufPool.Put(bbCompressed)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot decompress indexBlock read at offset %d with size %d: %s", p.indexFile.Path(), ih.indexBlockOffset, ih.indexBlockSize, err)
	}

	dst, err = unmarshalBlockHeaders(dst, bb.B, p.ph.FormatVersion)
	longTermBufPool.Put(bb)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot unmarshal block headers read at offset %d with size %d: %s", p.indexFile.Path(), ih.indexBlockOffset, ih.indexBlockSize, err)
	}

	return dst
}

// getStreamStr returns _stream value for the given block at bs.
func (bs *blockSearch) getStreamStr() string {
	sid := bs.bsw.bh.streamID.id
	streamStr := bs.seenStreams[sid]
	if streamStr != "" {
		// Fast path - streamStr is found in the seenStreams.
		return streamStr
	}

	// Slow path - load streamStr from the storage.
	streamStr = bs.getStreamStrSlow()
	if streamStr != "" {
		// Store the found streamStr in seenStreams.
		if len(bs.seenStreams) > 20_000 {
			bs.seenStreams = nil
		}
		if bs.seenStreams == nil {
			bs.seenStreams = make(map[u128]string)
		}
		bs.seenStreams[sid] = streamStr
	}
	return streamStr
}

func (bs *blockSearch) getStreamStrSlow() string {
	bb := bbPool.Get()
	defer bbPool.Put(bb)

	bb.B = bs.bsw.p.pt.idb.appendStreamTagsByStreamID(bb.B[:0], &bs.bsw.bh.streamID)
	if len(bb.B) == 0 {
		// Couldn't find stream tags by sid. This may be the case when the corresponding log stream
		// was recently registered and its tags aren't visible to search yet.
		// The stream tags must become visible in a few seconds.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6042
		return ""
	}

	st := GetStreamTags()
	streamTagsCanonical := bytesutil.ToUnsafeString(bb.B)
	mustUnmarshalStreamTags(st, streamTagsCanonical)
	bb.B = st.marshalString(bb.B[:0])
	PutStreamTags(st)

	return string(bb.B)
}
