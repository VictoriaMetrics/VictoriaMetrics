package logstorage

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// block represents a block of log entries.
type block struct {
	// timestamps contains timestamps for log entries.
	timestamps []int64

	// columns contains values for fields seen in log entries.
	columns []column

	// constColumns contains fields with constant values across all the block entries.
	constColumns []Field
}

func (b *block) reset() {
	b.timestamps = b.timestamps[:0]

	cs := b.columns
	for i := range cs {
		cs[i].reset()
	}
	b.columns = cs[:0]

	ccs := b.constColumns
	for i := range ccs {
		ccs[i].Reset()
	}
	b.constColumns = ccs[:0]
}

// uncompressedSizeBytes returns the total size of the origianl log entries stored in b.
//
// It is supposed that every log entry has the following format:
//
// 2006-01-02T15:04:05.999999999Z07:00 field1=value1 ... fieldN=valueN
func (b *block) uncompressedSizeBytes() uint64 {
	rowsCount := uint64(b.Len())

	// Take into account timestamps
	n := rowsCount * uint64(len(time.RFC3339Nano))

	// Take into account columns
	cs := b.columns
	for i := range cs {
		c := &cs[i]
		nameLen := uint64(len(c.name))
		if nameLen == 0 {
			nameLen = uint64(len("_msg"))
		}
		for _, v := range c.values {
			if len(v) > 0 {
				n += nameLen + 2 + uint64(len(v))
			}
		}
	}

	// Take into account constColumns
	ccs := b.constColumns
	for i := range ccs {
		cc := &ccs[i]
		nameLen := uint64(len(cc.Name))
		if nameLen == 0 {
			nameLen = uint64(len("_msg"))
		}
		n += rowsCount * (2 + nameLen + uint64(len(cc.Value)))
	}

	return n
}

// uncompressedRowsSizeBytes returns the size of the uncompressed rows.
//
// It is supposed that every row has the following format:
//
// 2006-01-02T15:04:05.999999999Z07:00 field1=value1 ... fieldN=valueN
func uncompressedRowsSizeBytes(rows [][]Field) uint64 {
	n := uint64(0)
	for _, fields := range rows {
		n += uncompressedRowSizeBytes(fields)
	}
	return n
}

// uncompressedRowSizeBytes returns the size of uncompressed row.
//
// It is supposed that the row has the following format:
//
// 2006-01-02T15:04:05.999999999Z07:00 field1=value1 ... fieldN=valueN
func uncompressedRowSizeBytes(fields []Field) uint64 {
	n := uint64(len(time.RFC3339Nano)) // log timestamp
	for _, f := range fields {
		nameLen := len(f.Name)
		if nameLen == 0 {
			nameLen = len("_msg")
		}
		n += uint64(2 + nameLen + len(f.Value))
	}
	return n
}

// column contains values for the given field name seen in log entries.
type column struct {
	// name is the field name
	name string

	// values is the values seen for the given log entries.
	values []string
}

func (c *column) reset() {
	c.name = ""

	values := c.values
	for i := range values {
		values[i] = ""
	}
	c.values = values[:0]
}

func (c *column) canStoreInConstColumn() bool {
	values := c.values
	if len(values) == 0 {
		return true
	}
	value := values[0]
	if len(value) > maxConstColumnValueSize {
		return false
	}
	for _, v := range values[1:] {
		if value != v {
			return false
		}
	}
	return true
}

func (c *column) resizeValues(valuesLen int) []string {
	values := c.values
	if n := valuesLen - cap(values); n > 0 {
		values = append(values[:cap(values)], make([]string, n)...)
	}
	values = values[:valuesLen]
	c.values = values
	return values
}

// mustWriteTo writes c to sw and updates ch accordingly.
func (c *column) mustWriteTo(ch *columnHeader, sw *streamWriters) {
	ch.reset()

	valuesWriter := &sw.fieldValuesWriter
	bloomFilterWriter := &sw.fieldBloomFilterWriter
	if c.name == "" {
		valuesWriter = &sw.messageValuesWriter
		bloomFilterWriter = &sw.messageBloomFilterWriter
	}

	ch.name = c.name

	// encode values
	ve := getValuesEncoder()
	ch.valueType, ch.minValue, ch.maxValue = ve.encode(c.values, &ch.valuesDict)

	bb := longTermBufPool.Get()
	defer longTermBufPool.Put(bb)

	// marshal values
	bb.B = marshalStringsBlock(bb.B[:0], ve.values)
	putValuesEncoder(ve)
	ch.valuesSize = uint64(len(bb.B))
	if ch.valuesSize > maxValuesBlockSize {
		logger.Panicf("BUG: too valuesSize: %d bytes; mustn't exceed %d bytes", ch.valuesSize, maxValuesBlockSize)
	}
	ch.valuesOffset = valuesWriter.bytesWritten
	valuesWriter.MustWrite(bb.B)

	// create and marshal bloom filter for c.values
	if ch.valueType != valueTypeDict {
		tokensBuf := getTokensBuf()
		tokensBuf.A = tokenizeStrings(tokensBuf.A[:0], c.values)
		bb.B = bloomFilterMarshal(bb.B[:0], tokensBuf.A)
		putTokensBuf(tokensBuf)
	} else {
		// there is no need in ecoding bloom filter for dictiory type,
		// since it isn't used during querying - all the dictionary values are available in ch.valuesDict
		bb.B = bb.B[:0]
	}
	ch.bloomFilterSize = uint64(len(bb.B))
	if ch.bloomFilterSize > maxBloomFilterBlockSize {
		logger.Panicf("BUG: too big bloomFilterSize: %d bytes; mustn't exceed %d bytes", ch.bloomFilterSize, maxBloomFilterBlockSize)
	}
	ch.bloomFilterOffset = bloomFilterWriter.bytesWritten
	bloomFilterWriter.MustWrite(bb.B)
}

func (b *block) assertValid() {
	// Check that timestamps are in ascending order
	timestamps := b.timestamps
	for i := 1; i < len(timestamps); i++ {
		if timestamps[i-1] > timestamps[i] {
			logger.Panicf("BUG: log entries must be sorted by timestamp; got the previous entry with bigger timestamp %d than the current entry with timestamp %d",
				timestamps[i-1], timestamps[i])
		}
	}

	// Check that the number of items in each column matches the number of items in the block.
	itemsCount := len(timestamps)
	columns := b.columns
	for _, c := range columns {
		if len(c.values) != itemsCount {
			logger.Panicf("BUG: unexpected number of values for column %q: got %d; want %d", c.name, len(c.values), itemsCount)
		}
	}
}

// MustInitFromRows initializes b from the given timestamps and rows.
//
// It is expected that timestamps are sorted.
func (b *block) MustInitFromRows(timestamps []int64, rows [][]Field) {
	b.reset()

	assertTimestampsSorted(timestamps)
	b.timestamps = append(b.timestamps, timestamps...)
	b.mustInitFromRows(rows)
	b.sortColumnsByName()
}

func (b *block) mustInitFromRows(rows [][]Field) {
	rowsLen := len(rows)
	if rowsLen == 0 {
		// Nothing to do
		return
	}

	if areSameFieldsInRows(rows) {
		// Fast path - all the log entries have the same fields
		fields := rows[0]
		for i := range fields {
			f := &fields[i]
			if canStoreInConstColumn(rows, i) {
				cc := b.extendConstColumns()
				cc.Name = f.Name
				cc.Value = f.Value
			} else {
				c := b.extendColumns()
				c.name = f.Name
				values := c.resizeValues(rowsLen)
				for j := range rows {
					values[j] = rows[j][i].Value
				}
			}
		}
		return
	}

	// Slow path - log entries contain different set of fields

	// Determine indexes for columns
	columnIdxs := getColumnIdxs()
	for i := range rows {
		fields := rows[i]
		for j := range fields {
			name := fields[j].Name
			if _, ok := columnIdxs[name]; !ok {
				columnIdxs[name] = len(columnIdxs)
			}
		}
	}

	// Initialize columns
	cs := b.resizeColumns(len(columnIdxs))
	for name, idx := range columnIdxs {
		c := &cs[idx]
		c.name = name
		c.resizeValues(rowsLen)
	}

	// Write rows to block
	for i := range rows {
		for _, f := range rows[i] {
			idx := columnIdxs[f.Name]
			cs[idx].values[i] = f.Value
		}
	}
	putColumnIdxs(columnIdxs)

	// Detect const columns
	for i := len(cs) - 1; i >= 0; i-- {
		c := &cs[i]
		if !c.canStoreInConstColumn() {
			continue
		}
		cc := b.extendConstColumns()
		cc.Name = c.name
		cc.Value = c.values[0]

		c.reset()
		if i < len(cs)-1 {
			swapColumns(c, &cs[len(cs)-1])
		}
		cs = cs[:len(cs)-1]
	}
	b.columns = cs
}

func swapColumns(a, b *column) {
	*a, *b = *b, *a
}

func canStoreInConstColumn(rows [][]Field, colIdx int) bool {
	if len(rows) == 0 {
		return true
	}
	value := rows[0][colIdx].Value
	if len(value) > maxConstColumnValueSize {
		return false
	}
	rows = rows[1:]
	for i := range rows {
		if value != rows[i][colIdx].Value {
			return false
		}
	}
	return true
}

func assertTimestampsSorted(timestamps []int64) {
	for i := range timestamps {
		if i > 0 && timestamps[i-1] > timestamps[i] {
			logger.Panicf("BUG: log entries must be sorted by timestamp; got the previous entry with bigger timestamp %d than the current entry with timestamp %d",
				timestamps[i-1], timestamps[i])
		}
	}
}

func (b *block) extendConstColumns() *Field {
	ccs := b.constColumns
	if cap(ccs) > len(ccs) {
		ccs = ccs[:len(ccs)+1]
	} else {
		ccs = append(ccs, Field{})
	}
	b.constColumns = ccs
	return &ccs[len(ccs)-1]
}

func (b *block) extendColumns() *column {
	cs := b.columns
	if cap(cs) > len(cs) {
		cs = cs[:len(cs)+1]
	} else {
		cs = append(cs, column{})
	}
	b.columns = cs
	return &cs[len(cs)-1]
}

func (b *block) resizeColumns(columnsLen int) []column {
	cs := b.columns[:0]
	if n := columnsLen - cap(cs); n > 0 {
		cs = append(cs[:cap(cs)], make([]column, n)...)
	}
	cs = cs[:columnsLen]
	b.columns = cs
	return cs
}

func (b *block) sortColumnsByName() {
	if len(b.columns)+len(b.constColumns) > maxColumnsPerBlock {
		logger.Panicf("BUG: too big number of columns detected in the block: %d; the number of columns mustn't exceed %d",
			len(b.columns)+len(b.constColumns), maxColumnsPerBlock)
	}

	cs := getColumnsSorter()
	cs.columns = b.columns
	sort.Sort(cs)
	putColumnsSorter(cs)

	ccs := getConstColumnsSorter()
	ccs.columns = b.constColumns
	sort.Sort(ccs)
	putConstColumnsSorter(ccs)
}

// Len returns the number of log entries in b.
func (b *block) Len() int {
	return len(b.timestamps)
}

// InitFromBlockData unmarshals bd to b.
//
// sbu and vd are used as a temporary storage for unmarshaled column values.
//
// The b becomes outdated after sbu or vd is reset.
func (b *block) InitFromBlockData(bd *blockData, sbu *stringsBlockUnmarshaler, vd *valuesDecoder) error {
	b.reset()

	if bd.rowsCount > maxRowsPerBlock {
		return fmt.Errorf("too many entries found in the block: %d; mustn't exceed %d", bd.rowsCount, maxRowsPerBlock)
	}
	rowsCount := int(bd.rowsCount)

	// unmarshal timestamps
	td := &bd.timestampsData
	var err error
	b.timestamps, err = encoding.UnmarshalTimestamps(b.timestamps[:0], td.data, td.marshalType, td.minTimestamp, rowsCount)
	if err != nil {
		return fmt.Errorf("cannot unmarshal timestamps: %w", err)
	}

	// unmarshal columns
	cds := bd.columnsData
	cs := b.resizeColumns(len(cds))
	for i := range cds {
		cd := &cds[i]
		c := &cs[i]
		c.name = cd.name
		c.values, err = sbu.unmarshal(c.values[:0], cd.valuesData, uint64(rowsCount))
		if err != nil {
			return fmt.Errorf("cannot unmarshal column %d: %w", i, err)
		}
		if err = vd.decodeInplace(c.values, cd.valueType, &cd.valuesDict); err != nil {
			return fmt.Errorf("cannot decode column values: %w", err)
		}
	}

	// unmarshal constColumns
	b.constColumns = append(b.constColumns[:0], bd.constColumns...)

	return nil
}

// mustWriteTo writes b with the given sid to sw and updates bh accordingly
func (b *block) mustWriteTo(sid *streamID, bh *blockHeader, sw *streamWriters) {
	// Do not store the version used for encoding directly in the block data, since:
	// - all the blocks in the same part use the same encoding
	// - the block encoding version can be put in metadata file for the part (aka metadataFilename)

	b.assertValid()
	bh.reset()

	bh.streamID = *sid
	bh.uncompressedSizeBytes = b.uncompressedSizeBytes()
	bh.rowsCount = uint64(b.Len())

	// Marshal timestamps
	mustWriteTimestampsTo(&bh.timestampsHeader, b.timestamps, sw)

	// Marshal columns
	cs := b.columns
	csh := getColumnsHeader()
	chs := csh.resizeColumnHeaders(len(cs))
	for i := range cs {
		cs[i].mustWriteTo(&chs[i], sw)
	}
	csh.constColumns = append(csh.constColumns[:0], b.constColumns...)

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

// appendRowsTo appends log entries from b to dst.
func (b *block) appendRowsTo(dst *rows) {
	// copy timestamps
	dst.timestamps = append(dst.timestamps, b.timestamps...)

	// copy columns
	fieldsBuf := dst.fieldsBuf
	ccs := b.constColumns
	cs := b.columns
	for i := range b.timestamps {
		fieldsLen := len(fieldsBuf)
		// copy const columns
		for j := range ccs {
			cc := &ccs[j]
			fieldsBuf = append(fieldsBuf, Field{
				Name:  cc.Name,
				Value: cc.Value,
			})
		}
		// copy other columns
		for j := range cs {
			c := &cs[j]
			value := c.values[i]
			if len(value) == 0 {
				continue
			}
			fieldsBuf = append(fieldsBuf, Field{
				Name:  c.name,
				Value: value,
			})
		}
		dst.rows = append(dst.rows, fieldsBuf[fieldsLen:])
	}
	dst.fieldsBuf = fieldsBuf
}

func areSameFieldsInRows(rows [][]Field) bool {
	if len(rows) < 2 {
		return true
	}
	fields := rows[0]

	// Verify that all the field names are unique
	m := make(map[string]struct{}, len(fields))
	for i := range fields {
		f := &fields[i]
		if _, ok := m[f.Name]; ok {
			// Field name isn't unique
			return false
		}
		m[f.Name] = struct{}{}
	}

	// Verify that all the fields are the same across rows
	rows = rows[1:]
	for i := range rows {
		leFields := rows[i]
		if len(fields) != len(leFields) {
			return false
		}
		for j := range leFields {
			if leFields[j].Name != fields[j].Name {
				return false
			}
		}
	}
	return true
}

var columnIdxsPool sync.Pool

func getColumnIdxs() map[string]int {
	v := columnIdxsPool.Get()
	if v == nil {
		return make(map[string]int)
	}
	return v.(map[string]int)
}

func putColumnIdxs(m map[string]int) {
	for k := range m {
		delete(m, k)
	}
	columnIdxsPool.Put(m)
}

func getBlock() *block {
	v := blockPool.Get()
	if v == nil {
		return &block{}
	}
	return v.(*block)
}

func putBlock(b *block) {
	b.reset()
	blockPool.Put(b)
}

var blockPool sync.Pool

type columnsSorter struct {
	columns []column
}

func (cs *columnsSorter) reset() {
	cs.columns = nil
}

func (cs *columnsSorter) Len() int {
	return len(cs.columns)
}

func (cs *columnsSorter) Less(i, j int) bool {
	columns := cs.columns
	return columns[i].name < columns[j].name
}

func (cs *columnsSorter) Swap(i, j int) {
	columns := cs.columns
	columns[i], columns[j] = columns[j], columns[i]
}

func getColumnsSorter() *columnsSorter {
	v := columnsSorterPool.Get()
	if v == nil {
		return &columnsSorter{}
	}
	return v.(*columnsSorter)
}

func putColumnsSorter(cs *columnsSorter) {
	cs.reset()
	columnsSorterPool.Put(cs)
}

var columnsSorterPool sync.Pool

type constColumnsSorter struct {
	columns []Field
}

func (ccs *constColumnsSorter) reset() {
	ccs.columns = nil
}

func (ccs *constColumnsSorter) Len() int {
	return len(ccs.columns)
}

func (ccs *constColumnsSorter) Less(i, j int) bool {
	columns := ccs.columns
	return columns[i].Name < columns[j].Name
}

func (ccs *constColumnsSorter) Swap(i, j int) {
	columns := ccs.columns
	columns[i], columns[j] = columns[j], columns[i]
}

func getConstColumnsSorter() *constColumnsSorter {
	v := constColumnsSorterPool.Get()
	if v == nil {
		return &constColumnsSorter{}
	}
	return v.(*constColumnsSorter)
}

func putConstColumnsSorter(ccs *constColumnsSorter) {
	ccs.reset()
	constColumnsSorterPool.Put(ccs)
}

var constColumnsSorterPool sync.Pool

// mustWriteTimestampsTo writes timestamps to sw and updates th accordingly
func mustWriteTimestampsTo(th *timestampsHeader, timestamps []int64, sw *streamWriters) {
	th.reset()

	bb := longTermBufPool.Get()
	bb.B, th.marshalType, th.minTimestamp = encoding.MarshalTimestamps(bb.B[:0], timestamps, 64)
	if len(bb.B) > maxTimestampsBlockSize {
		logger.Panicf("BUG: too big block with timestamps: %d bytes; the maximum supported size is %d bytes", len(bb.B), maxTimestampsBlockSize)
	}
	th.maxTimestamp = timestamps[len(timestamps)-1]
	th.blockOffset = sw.timestampsWriter.bytesWritten
	th.blockSize = uint64(len(bb.B))
	sw.timestampsWriter.MustWrite(bb.B)
	longTermBufPool.Put(bb)
}
