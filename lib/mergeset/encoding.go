package mergeset

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type byteSliceSorter [][]byte

func (s byteSliceSorter) Len() int { return len(s) }
func (s byteSliceSorter) Less(i, j int) bool {
	return string(s[i]) < string(s[j])
}
func (s byteSliceSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type inmemoryBlock struct {
	commonPrefix []byte
	data         []byte
	items        byteSliceSorter
}

func (ib *inmemoryBlock) Reset() {
	ib.commonPrefix = ib.commonPrefix[:0]
	ib.data = ib.data[:0]
	ib.items = ib.items[:0]
}

func (ib *inmemoryBlock) updateCommonPrefix() {
	ib.commonPrefix = ib.commonPrefix[:0]
	if len(ib.items) == 0 {
		return
	}
	cp := ib.items[0]
	if len(cp) == 0 {
		return
	}
	for _, item := range ib.items[1:] {
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
// false is returned if x isn't added to ib due to block size contraints.
func (ib *inmemoryBlock) Add(x []byte) bool {
	if len(x)+len(ib.data) > maxInmemoryBlockSize {
		return false
	}
	if cap(ib.data) < maxInmemoryBlockSize {
		dataLen := len(ib.data)
		ib.data = bytesutil.Resize(ib.data, maxInmemoryBlockSize)[:dataLen]
	}
	ib.data = append(ib.data, x...)
	ib.items = append(ib.items, ib.data[len(ib.data)-len(x):])
	return true
}

// maxInmemoryBlockSize is the maximum inmemoryBlock.data size.
//
// It must fit CPU cache size, i.e. 64KB for the current CPUs.
const maxInmemoryBlockSize = 64 * 1024

func (ib *inmemoryBlock) sort() {
	// Use sort.Sort instead of sort.Slice in order to eliminate memory allocation.
	sort.Sort(&ib.items)
	bb := bbPool.Get()
	b := bytesutil.Resize(bb.B, len(ib.data))
	b = b[:0]
	for i, item := range ib.items {
		b = append(b, item...)
		ib.items[i] = b[len(b)-len(item):]
	}
	bb.B, ib.data = ib.data, b
	bbPool.Put(bb)
}

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
	return sort.IsSorted(&ib.items)
}

// MarshalUnsortedData marshals unsorted items from ib to sb.
//
// It also:
// - appends first item to firstItemDst and returns the result.
// - appends common prefix for all the items to commonPrefixDst and returns the result.
// - returns the number of items encoded including the first item.
// - returns the marshal type used for the encoding.
func (ib *inmemoryBlock) MarshalUnsortedData(sb *storageBlock, firstItemDst, commonPrefixDst []byte, compressLevel int) ([]byte, []byte, uint32, marshalType) {
	if !ib.isSorted() {
		ib.sort()
	}
	ib.updateCommonPrefix()
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
	ib.updateCommonPrefix()
	return ib.marshalData(sb, firstItemDst, commonPrefixDst, compressLevel)
}

func (ib *inmemoryBlock) debugItemsString() string {
	var sb strings.Builder
	var prevItem []byte
	for i, item := range ib.items {
		if string(item) < string(prevItem) {
			fmt.Fprintf(&sb, "!!! the next item is smaller than the previous item !!!\n")
		}
		fmt.Fprintf(&sb, "%05d %X\n", i, item)
		prevItem = item
	}
	return sb.String()
}

// Preconditions:
// - ib.items must be sorted.
// - updateCommonPrefix must be called.
func (ib *inmemoryBlock) marshalData(sb *storageBlock, firstItemDst, commonPrefixDst []byte, compressLevel int) ([]byte, []byte, uint32, marshalType) {
	if len(ib.items) <= 0 {
		logger.Panicf("BUG: inmemoryBlock.marshalData must be called on non-empty blocks only")
	}
	if uint64(len(ib.items)) >= 1<<32 {
		logger.Panicf("BUG: the number of items in the block must be smaller than %d; got %d items", uint64(1<<32), len(ib.items))
	}

	firstItemDst = append(firstItemDst, ib.items[0]...)
	commonPrefixDst = append(commonPrefixDst, ib.commonPrefix...)

	if len(ib.data)-len(ib.commonPrefix)*len(ib.items) < 64 || len(ib.items) < 2 {
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
	prevItem := ib.items[0][cpLen:]
	prevPrefixLen := uint64(0)
	for i, item := range ib.items[1:] {
		item := item[cpLen:]
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
	prevItemLen := uint64(len(ib.items[0]) - cpLen)
	for i, item := range ib.items[1:] {
		itemLen := uint64(len(item) - cpLen)
		xLen := itemLen ^ prevItemLen
		prevItemLen = itemLen

		xs.A[i] = xLen
	}
	bLens = encoding.MarshalVarUint64s(bLens, xs.A)
	sb.lensData = encoding.CompressZSTDLevel(sb.lensData[:0], bLens, compressLevel)

	bbLens.B = bLens
	bbPool.Put(bbLens)

	if float64(len(sb.itemsData)) > 0.9*float64(len(ib.data)-len(ib.commonPrefix)*len(ib.items)) {
		// Bad compression rate. It is cheaper to use plain encoding.
		ib.marshalDataPlain(sb)
		return firstItemDst, commonPrefixDst, uint32(len(ib.items)), marshalTypePlain
	}

	// Good compression rate.
	return firstItemDst, commonPrefixDst, uint32(len(ib.items)), marshalTypeZSTD
}

// UnmarshalData decodes itemsCount items from sb and firstItem and stores
// them to ib.
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
	dataLen := uint64(len(commonPrefix) * int(itemsCount))
	dataLen += lens[0]
	for i, xLen := range is.A {
		itemLen := xLen ^ lens[i]
		lens[i+1] = itemLen
		dataLen += itemLen
	}

	// Unmarshal items data.
	bb.B, err = encoding.DecompressZSTD(bb.B[:0], sb.itemsData)
	if err != nil {
		return fmt.Errorf("cannot decompress lensData: %w", err)
	}
	data := bytesutil.Resize(ib.data, maxInmemoryBlockSize)
	if n := int(itemsCount) - cap(ib.items); n > 0 {
		ib.items = append(ib.items[:cap(ib.items)], make([][]byte, n)...)
	}
	ib.items = ib.items[:itemsCount]
	data = append(data[:0], firstItem...)
	ib.items[0] = data
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
		data = append(data, commonPrefix...)

		if prefixLen > uint64(len(prevItem)) {
			return fmt.Errorf("prefixLen cannot exceed %d; got %d", len(prevItem), prefixLen)
		}
		data = append(data, prevItem[:prefixLen]...)
		data = append(data, b[:suffixLen]...)
		item := data[len(data)-int(itemLen)-len(commonPrefix):]
		ib.items[i] = item
		b = b[suffixLen:]
		prevItem = item[len(commonPrefix):]
	}
	if len(b) > 0 {
		return fmt.Errorf("unexpected tail left after itemsData with len %d: %q", len(b), b)
	}
	if uint64(len(data)) != dataLen {
		return fmt.Errorf("unexpected data len; got %d; want %d", len(data), dataLen)
	}
	if !ib.isSorted() {
		return fmt.Errorf("decoded data block contains unsorted items; items:\n%s", ib.debugItemsString())
	}
	ib.data = data
	return nil
}

var bbPool bytesutil.ByteBufferPool

func (ib *inmemoryBlock) marshalDataPlain(sb *storageBlock) {
	// Marshal items data.
	// There is no need in marshaling the first item, since it is returned
	// to the caller in marshalData.
	cpLen := len(ib.commonPrefix)
	b := sb.itemsData[:0]
	for _, item := range ib.items[1:] {
		b = append(b, item[cpLen:]...)
	}
	sb.itemsData = b

	// Marshal length data.
	b = sb.lensData[:0]
	for _, item := range ib.items[1:] {
		b = encoding.MarshalUint64(b, uint64(len(item)-cpLen))
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
	ib.data = bytesutil.Resize(ib.data, len(firstItem)+len(sb.itemsData)+len(commonPrefix)*int(itemsCount))
	ib.data = append(ib.data[:0], firstItem...)
	ib.items = append(ib.items[:0], ib.data)

	b = sb.itemsData
	for i := 1; i < int(itemsCount); i++ {
		itemLen := lb.lens[i]
		if uint64(len(b)) < itemLen {
			return fmt.Errorf("not enough data for decoding item from itemsData; want %d bytes; remained %d bytes", itemLen, len(b))
		}
		ib.data = append(ib.data, commonPrefix...)
		ib.data = append(ib.data, b[:itemLen]...)
		item := ib.data[len(ib.data)-int(itemLen)-len(commonPrefix):]
		ib.items = append(ib.items, item)
		b = b[itemLen:]
	}
	if len(b) > 0 {
		return fmt.Errorf("unexpected tail left after itemsData with len %d: %q", len(b), b)
	}

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
	if nn := n - cap(lb.lens); nn > 0 {
		lb.lens = append(lb.lens[:cap(lb.lens)], make([]uint64, nn)...)
	}
	lb.lens = lb.lens[:n]
	return lb
}

func putLensBuffer(lb *lensBuffer) {
	lensBufferPool.Put(lb)
}

func getInmemoryBlock() *inmemoryBlock {
	v := ibPool.Get()
	if v == nil {
		return &inmemoryBlock{}
	}
	return v.(*inmemoryBlock)
}

func putInmemoryBlock(ib *inmemoryBlock) {
	ib.Reset()
	ibPool.Put(ib)
}

var ibPool sync.Pool
