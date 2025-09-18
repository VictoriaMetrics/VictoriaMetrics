package mergeset

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// Item represents a single item for storing in a mergeset.
type Item struct {
	// Start is start offset for the item in data.
	Start uint32

	// End is end offset for the item in data.
	End uint32
}

// Bytes returns bytes representation of it obtained from data.
//
// The returned bytes representation belongs to data.
func (it Item) Bytes(data []byte) []byte {
	n := it.End - it.Start
	if n == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(data)), it.Start)), n)
}

// String returns string representation of it obtained from data.
//
// The returned string representation belongs to data.
func (it Item) String(data []byte) string {
	n := it.End - it.Start
	if n == 0 {
		return ""
	}
	return unsafe.String((*byte)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(data)), it.Start)), n)
}

func (ib *inmemoryBlock) Len() int {
	return len(ib.items)
}

func (ib *inmemoryBlock) Less(i, j int) bool {
	items := ib.items
	a := items[i]
	b := items[j]
	cpLen := uint32(len(ib.commonPrefix))
	a.Start += cpLen
	b.Start += cpLen
	data := ib.data
	return a.String(data) < b.String(data)
}

func (ib *inmemoryBlock) Swap(i, j int) {
	items := ib.items
	items[i], items[j] = items[j], items[i]
}

type inmemoryBlock struct {
	// commonPrefix contains common prefix for all the items stored in the inmemoryBlock
	commonPrefix []byte

	// data contains source data for items
	data []byte

	// items contains items stored in inmemoryBlock.
	// Every item contains the prefix specified at commonPrefix.
	items []Item
}

func (ib *inmemoryBlock) CopyFrom(src *inmemoryBlock) {
	ib.commonPrefix = append(ib.commonPrefix[:0], src.commonPrefix...)
	ib.data = append(ib.data[:0], src.data...)
	ib.items = append(ib.items[:0], src.items...)
}

func (ib *inmemoryBlock) SortItems() {
	if !ib.isSorted() {
		ib.updateCommonPrefixUnsorted()
		sort.Sort(ib)
	} else {
		ib.updateCommonPrefixSorted()
	}
}

func (ib *inmemoryBlock) SizeBytes() int {
	return int(unsafe.Sizeof(*ib)) + cap(ib.commonPrefix) + cap(ib.data) + cap(ib.items)*int(unsafe.Sizeof(Item{}))
}

func (ib *inmemoryBlock) Reset() {
	ib.commonPrefix = ib.commonPrefix[:0]
	ib.data = ib.data[:0]
	ib.items = ib.items[:0]
}

func (ib *inmemoryBlock) updateCommonPrefixSorted() {
	items := ib.items
	if len(items) <= 1 {
		// There is no sense in duplicating a single item or zero items into commonPrefix,
		// since this only can increase blockHeader size without any benefits.
		ib.commonPrefix = ib.commonPrefix[:0]
		return
	}

	data := ib.data
	cp := items[0].Bytes(data)
	cpLen := commonPrefixLen(cp, items[len(items)-1].Bytes(data))
	cp = cp[:cpLen]
	ib.commonPrefix = append(ib.commonPrefix[:0], cp...)
}

func (ib *inmemoryBlock) updateCommonPrefixUnsorted() {
	ib.commonPrefix = ib.commonPrefix[:0]
	items := ib.items
	if len(items) == 0 {
		return
	}
	data := ib.data
	cp := items[0].Bytes(data)
	for _, it := range items[1:] {
		item := it.Bytes(data)
		if bytes.HasPrefix(item, cp) {
			continue
		}
		cpLen := commonPrefixLen(cp, item)
		if cpLen == 0 {
			return
		}
		cp = cp[:cpLen]
	}
	ib.commonPrefix = append(ib.commonPrefix[:0], cp...)
}

func commonPrefixLen(a, b []byte) int {
	i := 0
	if len(a) > len(b) {
		for i < len(b) && a[i] == b[i] {
			i++
		}
	} else {
		for i < len(a) && a[i] == b[i] {
			i++
		}
	}
	return i
}

// Add adds x to the end of ib.
//
// false is returned if x isn't added to ib due to block size constraints.
func (ib *inmemoryBlock) Add(x []byte) bool {
	data := ib.data
	if len(x)+len(data) > maxInmemoryBlockSize {
		return false
	}
	if cap(data) == 0 {
		// Pre-allocate data and items in order to reduce memory allocations
		data = make([]byte, 0, maxInmemoryBlockSize)
		ib.items = make([]Item, 0, 512)
	}
	dataLen := len(data)
	data = append(data, x...)
	ib.items = append(ib.items, Item{
		Start: uint32(dataLen),
		End:   uint32(len(data)),
	})
	ib.data = data
	return true
}

// maxInmemoryBlockSize is the maximum inmemoryBlock.data size.
//
// It must fit CPU cache size, i.e. 64KB for the current CPUs.
const maxInmemoryBlockSize = 64 * 1024

// storageBlock represents a block of data on the storage.
type storageBlock struct {
	itemsData []byte
	lensData  []byte
}

func (sb *storageBlock) Reset() {
	sb.itemsData = sb.itemsData[:0]
	sb.lensData = sb.lensData[:0]
}

type marshalType uint8

const (
	marshalTypePlain = marshalType(0)
	marshalTypeZSTD  = marshalType(1)
)

func checkMarshalType(mt marshalType) error {
	if mt < 0 || mt > 1 {
		return fmt.Errorf("marshalType must be in the range [0..1]; got %d", mt)
	}
	return nil
}

func (ib *inmemoryBlock) isSorted() bool {
	// Use sort.IsSorted instead of sort.SliceIsSorted in order to eliminate memory allocation.
	return sort.IsSorted(ib)
}

// MarshalUnsortedData marshals unsorted items from ib to sb.
//
// It also:
// - appends first item to firstItemDst and returns the result.
// - appends common prefix for all the items to commonPrefixDst and returns the result.
// - returns the number of items encoded including the first item.
// - returns the marshal type used for the encoding.
func (ib *inmemoryBlock) MarshalUnsortedData(sb *storageBlock, firstItemDst, commonPrefixDst []byte, compressLevel int) ([]byte, []byte, uint32, marshalType) {
	ib.SortItems()
	return ib.marshalData(sb, firstItemDst, commonPrefixDst, compressLevel)
}

var isInTest = func() bool {
	return strings.HasSuffix(os.Args[0], ".test")
}()

// MarshalSortedData marshals sorted items from ib to sb.
//
// It also:
// - appends first item to firstItemDst and returns the result.
// - appends common prefix for all the items to commonPrefixDst and returns the result.
// - returns the number of items encoded including the first item.
// - returns the marshal type used for the encoding.
func (ib *inmemoryBlock) MarshalSortedData(sb *storageBlock, firstItemDst, commonPrefixDst []byte, compressLevel int) ([]byte, []byte, uint32, marshalType) {
	if isInTest && !ib.isSorted() {
		logger.Panicf("BUG: %d items must be sorted; items:\n%s", len(ib.items), ib.debugItemsString())
	}
	ib.updateCommonPrefixSorted()
	return ib.marshalData(sb, firstItemDst, commonPrefixDst, compressLevel)
}

func (ib *inmemoryBlock) debugItemsString() string {
	var sb strings.Builder
	var prevItem string
	data := ib.data
	for i, it := range ib.items {
		item := it.String(data)
		if item < prevItem {
			fmt.Fprintf(&sb, "!!! the next item is smaller than the previous item !!!\n")
		}
		fmt.Fprintf(&sb, "%05d %X\n", i, item)
		prevItem = item
	}
	return sb.String()
}

// Preconditions:
// - ib.items must be sorted.
// - updateCommonPrefix* must be called.
func (ib *inmemoryBlock) marshalData(sb *storageBlock, firstItemDst, commonPrefixDst []byte, compressLevel int) ([]byte, []byte, uint32, marshalType) {
	if len(ib.items) <= 0 {
		logger.Panicf("BUG: inmemoryBlock.marshalData must be called on non-empty blocks only")
	}
	if uint64(len(ib.items)) >= 1<<32 {
		logger.Panicf("BUG: the number of items in the block must be smaller than %d; got %d items", uint64(1<<32), len(ib.items))
	}

	data := ib.data
	firstItem := ib.items[0].Bytes(data)
	firstItemDst = append(firstItemDst, firstItem...)
	commonPrefixDst = append(commonPrefixDst, ib.commonPrefix...)

	if len(data)-len(ib.commonPrefix)*len(ib.items) < 64 || len(ib.items) < 2 {
		// Use plain encoding form small block, since it is cheaper.
		ib.marshalDataPlain(sb)
		return firstItemDst, commonPrefixDst, uint32(len(ib.items)), marshalTypePlain
	}

	bbItems := bbPool.Get()
	bItems := bbItems.B[:0]

	bbLens := bbPool.Get()
	bLens := bbLens.B[:0]

	// Marshal items data.
	xs := encoding.GetUint64s(len(ib.items) - 1)
	defer encoding.PutUint64s(xs)

	cpLen := len(ib.commonPrefix)
	prevItem := firstItem[cpLen:]
	prevPrefixLen := uint64(0)
	for i, it := range ib.items[1:] {
		it.Start += uint32(cpLen)
		item := it.Bytes(data)
		prefixLen := uint64(commonPrefixLen(prevItem, item))
		bItems = append(bItems, item[prefixLen:]...)
		xLen := prefixLen ^ prevPrefixLen
		prevItem = item
		prevPrefixLen = prefixLen

		xs.A[i] = xLen
	}
	bLens = encoding.MarshalVarUint64s(bLens, xs.A)
	sb.itemsData = encoding.CompressZSTDLevel(sb.itemsData[:0], bItems, compressLevel)

	bbItems.B = bItems
	bbPool.Put(bbItems)

	// Marshal lens data.
	prevItemLen := uint64(len(firstItem) - cpLen)
	for i, it := range ib.items[1:] {
		itemLen := uint64(int(it.End-it.Start) - cpLen)
		xLen := itemLen ^ prevItemLen
		prevItemLen = itemLen

		xs.A[i] = xLen
	}
	bLens = encoding.MarshalVarUint64s(bLens, xs.A)
	sb.lensData = encoding.CompressZSTDLevel(sb.lensData[:0], bLens, compressLevel)

	bbLens.B = bLens
	bbPool.Put(bbLens)

	if float64(len(sb.itemsData)) > 0.9*float64(len(data)-len(ib.commonPrefix)*len(ib.items)) {
		// Bad compression rate. It is cheaper to use plain encoding.
		ib.marshalDataPlain(sb)
		return firstItemDst, commonPrefixDst, uint32(len(ib.items)), marshalTypePlain
	}

	// Good compression rate.
	return firstItemDst, commonPrefixDst, uint32(len(ib.items)), marshalTypeZSTD
}

// UnmarshalData decodes itemsCount items from sb and firstItem and stores them to ib.
func (ib *inmemoryBlock) UnmarshalData(sb *storageBlock, firstItem, commonPrefix []byte, itemsCount uint32, mt marshalType) error {
	ib.Reset()

	if itemsCount <= 0 {
		logger.Panicf("BUG: cannot unmarshal zero items")
	}

	ib.commonPrefix = append(ib.commonPrefix[:0], commonPrefix...)

	switch mt {
	case marshalTypePlain:
		if err := ib.unmarshalDataPlain(sb, firstItem, itemsCount); err != nil {
			return fmt.Errorf("cannot unmarshal plain data: %w", err)
		}
		if !ib.isSorted() {
			return fmt.Errorf("plain data block contains unsorted items; items:\n%s", ib.debugItemsString())
		}
		return nil
	case marshalTypeZSTD:
		// it is handled below.
	default:
		return fmt.Errorf("unknown marshalType=%d", mt)
	}

	// Unmarshal mt = marshalTypeZSTD

	bb := bbPool.Get()
	defer bbPool.Put(bb)

	var err error

	// Unmarshal lens data.
	bb.B, err = encoding.DecompressZSTD(bb.B[:0], sb.lensData)
	if err != nil {
		return fmt.Errorf("cannot decompress lensData: %w", err)
	}

	lb := getLensBuffer(int(2 * itemsCount))
	defer putLensBuffer(lb)

	prefixLens := lb.lens[:itemsCount]
	lens := lb.lens[itemsCount:]

	is := encoding.GetUint64s(int(itemsCount) - 1)
	defer encoding.PutUint64s(is)

	// Unmarshal prefixLens
	tail, err := encoding.UnmarshalVarUint64s(is.A, bb.B)
	if err != nil {
		return fmt.Errorf("cannot unmarshal prefixLens from lensData: %w", err)
	}
	prefixLens[0] = 0
	for i, xLen := range is.A {
		prefixLens[i+1] = xLen ^ prefixLens[i]
	}

	// Unmarshal lens
	tail, err = encoding.UnmarshalVarUint64s(is.A, tail)
	if err != nil {
		return fmt.Errorf("cannot unmarshal lens from lensData: %w", err)
	}
	if len(tail) > 0 {
		return fmt.Errorf("unexpected tail left unmarshaling %d lens; tail size=%d; contents=%X", itemsCount, len(tail), tail)
	}
	lens[0] = uint64(len(firstItem) - len(commonPrefix))
	dataLen := len(commonPrefix) * int(itemsCount)
	dataLen += int(lens[0])
	for i, xLen := range is.A {
		itemLen := xLen ^ lens[i]
		lens[i+1] = itemLen
		dataLen += int(itemLen)
	}

	// Unmarshal items data.
	bb.B, err = encoding.DecompressZSTD(bb.B[:0], sb.itemsData)
	if err != nil {
		return fmt.Errorf("cannot decompress lensData: %w", err)
	}
	// Resize ib.data to dataLen instead of maxInmemoryBlockSize,
	// since the data isn't going to be resized after unmarshaling.
	// This may save memory for caching the unmarshaled block.
	data := bytesutil.ResizeNoCopyNoOverallocate(ib.data, dataLen)
	ib.items = slicesutil.SetLength(ib.items, int(itemsCount))
	data = append(data[:0], firstItem...)
	items := ib.items
	items[0] = Item{
		Start: 0,
		End:   uint32(len(data)),
	}
	prevItem := data[len(commonPrefix):]
	b := bb.B
	for i := 1; i < int(itemsCount); i++ {
		itemLen := lens[i]
		prefixLen := prefixLens[i]
		if prefixLen > itemLen {
			return fmt.Errorf("prefixLen=%d exceeds itemLen=%d", prefixLen, itemLen)
		}
		suffixLen := itemLen - prefixLen
		if uint64(len(b)) < suffixLen {
			return fmt.Errorf("not enough data for decoding item from itemsData; want %d bytes; remained %d bytes", suffixLen, len(b))
		}
		if prefixLen > uint64(len(prevItem)) {
			return fmt.Errorf("prefixLen cannot exceed %d; got %d", len(prevItem), prefixLen)
		}
		dataStart := len(data)
		data = append(data, commonPrefix...)
		data = append(data, prevItem[:prefixLen]...)
		data = append(data, b[:suffixLen]...)
		items[i] = Item{
			Start: uint32(dataStart),
			End:   uint32(len(data)),
		}
		b = b[suffixLen:]
		prevItem = data[len(data)-int(itemLen):]
	}
	if len(b) > 0 {
		return fmt.Errorf("unexpected tail left after itemsData with len %d: %q", len(b), b)
	}
	if len(data) != dataLen {
		return fmt.Errorf("unexpected data len; got %d; want %d", len(data), dataLen)
	}
	ib.data = data
	if !ib.isSorted() {
		return fmt.Errorf("decoded data block contains unsorted items; items:\n%s", ib.debugItemsString())
	}
	return nil
}

var bbPool bytesutil.ByteBufferPool

func (ib *inmemoryBlock) marshalDataPlain(sb *storageBlock) {
	data := ib.data

	// Marshal items data.
	// There is no need in marshaling the first item, since it is returned
	// to the caller in marshalData.
	cpLen := len(ib.commonPrefix)
	b := sb.itemsData[:0]
	for _, it := range ib.items[1:] {
		it.Start += uint32(cpLen)
		b = append(b, it.String(data)...)
	}
	sb.itemsData = b

	// Marshal length data.
	b = sb.lensData[:0]
	for _, it := range ib.items[1:] {
		b = encoding.MarshalUint64(b, uint64(int(it.End-it.Start)-cpLen))
	}
	sb.lensData = b
}

func (ib *inmemoryBlock) unmarshalDataPlain(sb *storageBlock, firstItem []byte, itemsCount uint32) error {
	commonPrefix := ib.commonPrefix

	// Unmarshal lens data.
	lb := getLensBuffer(int(itemsCount))
	defer putLensBuffer(lb)

	lb.lens[0] = uint64(len(firstItem) - len(commonPrefix))
	b := sb.lensData
	for i := 1; i < int(itemsCount); i++ {
		if len(b) < 8 {
			return fmt.Errorf("too short tail for decoding len from lensData; got %d bytes; want at least %d bytes", len(b), 8)
		}
		iLen := encoding.UnmarshalUint64(b)
		b = b[8:]
		lb.lens[i] = iLen
	}
	if len(b) > 0 {
		return fmt.Errorf("unexpected tail left after lensData with len %d: %q", len(b), b)
	}

	// Unmarshal items data.
	data := ib.data
	items := ib.items
	dataLen := len(firstItem) + len(sb.itemsData) + len(commonPrefix)*(int(itemsCount)-1)
	data = bytesutil.ResizeNoCopyNoOverallocate(data, dataLen)
	data = append(data[:0], firstItem...)
	items = append(items[:0], Item{
		Start: 0,
		End:   uint32(len(data)),
	})
	b = sb.itemsData
	for i := 1; i < int(itemsCount); i++ {
		itemLen := lb.lens[i]
		if uint64(len(b)) < itemLen {
			return fmt.Errorf("not enough data for decoding item from itemsData; want %d bytes; remained %d bytes", itemLen, len(b))
		}
		dataStart := len(data)
		data = append(data, commonPrefix...)
		data = append(data, b[:itemLen]...)
		items = append(items, Item{
			Start: uint32(dataStart),
			End:   uint32(len(data)),
		})
		b = b[itemLen:]
	}
	if len(b) > 0 {
		return fmt.Errorf("unexpected tail left after itemsData with len %d: %q", len(b), b)
	}
	if len(data) != dataLen {
		return fmt.Errorf("unexpected data len; got %d; want %d", len(data), dataLen)
	}
	ib.data = data
	ib.items = items
	return nil
}

type lensBuffer struct {
	lens []uint64
}

var lensBufferPool sync.Pool

func getLensBuffer(n int) *lensBuffer {
	v := lensBufferPool.Get()
	if v == nil {
		v = &lensBuffer{}
	}
	lb := v.(*lensBuffer)
	lb.lens = slicesutil.SetLength(lb.lens, n)
	return lb
}

func putLensBuffer(lb *lensBuffer) {
	lensBufferPool.Put(lb)
}
