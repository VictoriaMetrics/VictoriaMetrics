package roaring64

import (
	"fmt"
	"io"
	"math/big"
	"runtime"
	"sort"
	"sync"
)

// BSI is at its simplest is an array of bitmaps that represent an encoded
// binary value.  The advantage of a BSI is that comparisons can be made
// across ranges of values whereas a bitmap can only represent the existence
// of a single value for a given column ID.  Another usage scenario involves
// storage of high cardinality values.
//
// It depends upon the bitmap libraries.  It is not thread safe, so
// upstream concurrency guards must be provided.
type BSI struct {
	bA           []Bitmap
	eBM          Bitmap // Existence BitMap
	MaxValue     int64
	MinValue     int64
	runOptimized bool
}

// BSIValuePair is a column ID and its BSI value.
type BSIValuePair struct {
	ColumnID uint64
	Value    int64
}

// NewBSI constructs a new BSI. Note that it is your responsibility to ensure that
// the min/max values are set correctly. Queries CompareValue, MinMax, etc. will not
// work correctly if the min/max values are not set correctly.
func NewBSI(maxValue int64, minValue int64) *BSI {

	bitszmin := big.NewInt(minValue).BitLen() + 1
	bitszmax := big.NewInt(maxValue).BitLen() + 1
	bitsz := bitszmin
	if bitszmax > bitsz {
		bitsz = bitszmax
	}
	ba := make([]Bitmap, bitsz)
	return &BSI{bA: ba, MaxValue: maxValue, MinValue: minValue}
}

// NewDefaultBSI constructs an auto-sized BSI
func NewDefaultBSI() *BSI {
	return NewBSI(int64(0), int64(0))
}

// RunOptimize attempts to further compress the runs of consecutive values found in the bitmap
func (b *BSI) RunOptimize() {
	b.eBM.RunOptimize()
	for i := 0; i < len(b.bA); i++ {
		b.bA[i].RunOptimize()
	}
	b.runOptimized = true
}

// HasRunCompression returns true if the bitmap benefits from run compression
func (b *BSI) HasRunCompression() bool {
	return b.runOptimized
}

// GetExistenceBitmap returns a pointer to the underlying existence bitmap of the BSI
func (b *BSI) GetExistenceBitmap() *Bitmap {
	return &b.eBM
}

// ValueExists tests whether the value exists.
func (b *BSI) ValueExists(columnID uint64) bool {

	return b.eBM.Contains(columnID)
}

// GetCardinality returns a count of unique column IDs for which a value has been set.
func (b *BSI) GetCardinality() uint64 {
	return b.eBM.GetCardinality()
}

// BitCount returns the number of bits needed to represent values.
func (b *BSI) BitCount() int {
	return len(b.bA) - 1 // Exclude sign bit
}

// IsBigUInt returns the number of bits needed to represent values.
func (b *BSI) isBig() bool {
	return len(b.bA) > 64
}

// IsNegative returns true for negative values
func (b *BSI) IsNegative(columnID uint64) bool {
	if len(b.bA) == 0 {
		return false
	}
	return b.bA[b.BitCount()].Contains(columnID)
}

// SetBigValue sets a value that exceeds 64 bits
func (b *BSI) SetBigValue(columnID uint64, value *big.Int) {
	// If max/min values are set to zero then automatically determine bit array size
	if b.MaxValue == 0 && b.MinValue == 0 {
		minBits := value.BitLen() + 1
		if minBits == 1 {
			minBits = 2
		}
		if len(b.bA) < minBits {
			oldSignPos := len(b.bA) - 1
			for len(b.bA) < minBits {
				b.bA = append(b.bA, Bitmap{})
			}
			// When bA grows, the sign slot shifts from oldSignPos to the new end
			// of bA. For existing negative entries (whose sign bit is set in
			// bA[oldSignPos]), sign-extension requires that all intermediate bit
			// positions between oldSignPos and the new sign position also be set.
			// Copy the old sign bitmap into every new slot (sign extension).
			newSignPos := len(b.bA) - 1
			for i := oldSignPos + 1; i <= newSignPos; i++ {
				b.bA[i].Or(&b.bA[oldSignPos])
			}
		}
	}

	for i := b.BitCount(); i >= 0; i-- {
		if value.Bit(i) == 0 {
			b.bA[i].Remove(columnID)
		} else {
			b.bA[i].Add(columnID)
		}
	}
	b.eBM.Add(columnID)
}

// SetBigMany sets value for all columns in foundSet.
func (b *BSI) SetBigMany(foundSet *Bitmap, value *big.Int) {
	// If max/min values are set to zero then automatically determine bit array size
	if b.MaxValue == 0 && b.MinValue == 0 {
		minBits := value.BitLen() + 1
		if minBits == 1 {
			minBits = 2
		}
		if len(b.bA) < minBits {
			oldSignPos := len(b.bA) - 1
			for len(b.bA) < minBits {
				b.bA = append(b.bA, Bitmap{})
			}
			// Sign-extend existing negative entries into the new bit slots.
			newSignPos := len(b.bA) - 1
			for i := oldSignPos + 1; i <= newSignPos; i++ {
				b.bA[i].Or(&b.bA[oldSignPos])
			}
		}
	}
	for i := b.BitCount(); i >= 0; i-- {
		if value.Bit(i) == 0 {
			b.bA[i].AndNot(foundSet)
		} else {
			b.bA[i].Or(foundSet)
		}
	}
	b.eBM.Or(foundSet)
}

// SetValue sets a value for a given columnID.
func (b *BSI) SetValue(columnID uint64, value int64) {
	b.SetBigValue(columnID, big.NewInt(value))
}

// SetMany sets a value for all columns in foundSet
func (b *BSI) SetMany(foundSet *Bitmap, value int64) {
	b.SetBigMany(foundSet, big.NewInt(value))
}

// GetValue gets the value at the column ID. Second param will be false for non-existent values.
func (b *BSI) GetValue(columnID uint64) (value int64, exists bool) {
	bv, exists := b.GetBigValue(columnID)
	if !exists {
		return
	}
	if !bv.IsInt64() {
		if bv.Sign() == -1 {
			msg := fmt.Errorf("can't represent a negative %d bit value as an int64", b.BitCount())
			panic(msg)
		}
		if bv.Sign() == 1 {
			msg := fmt.Errorf("can't represent a positive %d bit value as an int64", b.BitCount())
			panic(msg)
		}
	}
	return bv.Int64(), exists
}

// GetBigValue gets the value at the column ID. Second param will be false for non-existent values.
func (b *BSI) GetBigValue(columnID uint64) (value *big.Int, exists bool) {
	exists = b.eBM.Contains(columnID)
	if !exists {
		return
	}
	val := big.NewInt(0)
	for i := b.BitCount(); i >= 0; i-- {
		if b.bA[i].Contains(columnID) {
			bigBit := big.NewInt(1)
			bigBit.Lsh(bigBit, uint(i))
			val.Or(val, bigBit)
		}
	}

	if b.IsNegative(columnID) {
		val = negativeTwosComplementToInt(val)
	}
	return val, exists
}

// GetBigValues gets values for the column IDs. Returned values are aligned with
// columnIDs, and a nil entry means the corresponding column ID has no value.
func (b *BSI) GetBigValues(columnIDs []uint64) []*big.Int {
	values := make([]*big.Int, len(columnIDs))
	if len(columnIDs) == 0 {
		return values
	}
	if len(columnIDs) == 1 {
		if value, ok := b.GetBigValue(columnIDs[0]); ok {
			values[0] = value
		}
		return values
	}
	request := newBSIGetBigValuesRequest(columnIDs)
	if !b.isBig() {
		return b.getBigValuesInt64(request, values)
	}
	return b.getBigValuesGeneric(request, values)
}

// GetValues gets int64 values for the column IDs. Returned values are aligned
// with columnIDs, and a false exists entry means the corresponding column ID
// has no value. This fast path is available only for BSIs whose values fit in
// int64; it panics with the same representability semantics as GetValue when
// a value cannot be represented as int64.
func (b *BSI) GetValues(columnIDs []uint64) ([]int64, []bool) {
	values := make([]int64, len(columnIDs))
	exists := make([]bool, len(columnIDs))
	if len(columnIDs) == 0 {
		return values, exists
	}
	if len(columnIDs) == 1 {
		if value, ok := b.GetValue(columnIDs[0]); ok {
			values[0] = value
			exists[0] = true
		}
		return values, exists
	}
	request := newBSIGetBigValuesRequest(columnIDs)
	if b.isBig() {
		bigValues := b.getBigValuesGeneric(request, make([]*big.Int, len(columnIDs)))
		for i, value := range bigValues {
			if value == nil {
				continue
			}
			if !value.IsInt64() {
				if value.Sign() == -1 {
					msg := fmt.Errorf("can't represent a negative %d bit value as an int64", b.BitCount())
					panic(msg)
				}
				if value.Sign() == 1 {
					msg := fmt.Errorf("can't represent a positive %d bit value as an int64", b.BitCount())
					panic(msg)
				}
			}
			values[i] = value.Int64()
			exists[i] = true
		}
		return values, exists
	}
	return b.getValuesInt64(request, values, exists)
}

type bsiGetBigValuesRequest struct {
	foundSet           *Bitmap
	positions          map[uint64]int
	duplicatePositions map[uint64][]int
}

func newBSIGetBigValuesRequest(columnIDs []uint64) bsiGetBigValuesRequest {
	foundSet := NewBitmap()
	positions := make(map[uint64]int, len(columnIDs))
	var duplicatePositions map[uint64][]int
	for position, columnID := range columnIDs {
		if _, ok := positions[columnID]; ok {
			if duplicatePositions == nil {
				duplicatePositions = make(map[uint64][]int)
			}
			duplicatePositions[columnID] = append(duplicatePositions[columnID], position)
			continue
		}
		positions[columnID] = position
		foundSet.Add(columnID)
	}
	return bsiGetBigValuesRequest{
		foundSet:           foundSet,
		positions:          positions,
		duplicatePositions: duplicatePositions,
	}
}

func (b *BSI) getBigValuesInt64(request bsiGetBigValuesRequest, values []*big.Int) []*big.Int {
	existing := And(&b.eBM, request.foundSet)
	if existing.IsEmpty() {
		return values
	}

	rawValues := make([]uint64, len(values))
	signBit := b.BitCount()
	for bit := 0; bit <= signBit; bit++ {
		bitSet := And(&b.bA[bit], existing)
		iter := bitSet.Iterator()
		for iter.HasNext() {
			columnID := iter.Next()
			rawValues[request.positions[columnID]] |= uint64(1) << uint(bit)
		}
	}

	width := uint(signBit + 1)
	signMask := uint64(1) << uint(signBit)
	iter := existing.Iterator()
	for iter.HasNext() {
		columnID := iter.Next()
		position := request.positions[columnID]
		rawValue := rawValues[position]
		if rawValue&signMask != 0 && width < 64 {
			rawValue |= ^uint64(0) << width
		}
		values[position] = big.NewInt(int64(rawValue))
	}
	fillDuplicateBigValues(values, request)
	return values
}

func (b *BSI) getValuesInt64(request bsiGetBigValuesRequest, values []int64, exists []bool) ([]int64, []bool) {
	existing := And(&b.eBM, request.foundSet)
	if existing.IsEmpty() {
		return values, exists
	}

	rawValues := make([]uint64, len(values))
	signBit := b.BitCount()
	for bit := 0; bit <= signBit; bit++ {
		bitSet := And(&b.bA[bit], existing)
		iter := bitSet.Iterator()
		for iter.HasNext() {
			columnID := iter.Next()
			rawValues[request.positions[columnID]] |= uint64(1) << uint(bit)
		}
	}

	width := uint(signBit + 1)
	signMask := uint64(1) << uint(signBit)
	iter := existing.Iterator()
	for iter.HasNext() {
		columnID := iter.Next()
		position := request.positions[columnID]
		rawValue := rawValues[position]
		if rawValue&signMask != 0 && width < 64 {
			rawValue |= ^uint64(0) << width
		}
		values[position] = int64(rawValue)
		exists[position] = true
	}
	fillDuplicateValues(values, exists, request)
	return values, exists
}

func (b *BSI) getBigValuesGeneric(request bsiGetBigValuesRequest, values []*big.Int) []*big.Int {
	existing := And(&b.eBM, request.foundSet)
	if existing.IsEmpty() {
		return values
	}

	iter := existing.Iterator()
	for iter.HasNext() {
		values[request.positions[iter.Next()]] = big.NewInt(0)
	}
	for bit := b.BitCount(); bit >= 0; bit-- {
		bitSet := And(&b.bA[bit], existing)
		iter := bitSet.Iterator()
		for iter.HasNext() {
			columnID := iter.Next()
			position := request.positions[columnID]
			values[position].SetBit(values[position], bit, 1)
		}
	}

	signBit := b.BitCount()
	negativeSet := And(&b.bA[signBit], existing)
	iter = negativeSet.Iterator()
	for iter.HasNext() {
		position := request.positions[iter.Next()]
		values[position] = negativeTwosComplementToInt(values[position])
	}

	fillDuplicateBigValues(values, request)
	return values
}

func fillDuplicateBigValues(values []*big.Int, request bsiGetBigValuesRequest) {
	for columnID, extraPositions := range request.duplicatePositions {
		value := values[request.positions[columnID]]
		if value == nil {
			continue
		}
		for _, position := range extraPositions {
			values[position] = new(big.Int).Set(value)
		}
	}
}

func fillDuplicateValues(values []int64, exists []bool, request bsiGetBigValuesRequest) {
	for columnID, extraPositions := range request.duplicatePositions {
		position := request.positions[columnID]
		if !exists[position] {
			continue
		}
		for _, extraPosition := range extraPositions {
			values[extraPosition] = values[position]
			exists[extraPosition] = true
		}
	}
}

func negativeTwosComplementToInt(val *big.Int) *big.Int {
	inverted := new(big.Int).Not(val)
	mask := new(big.Int).Lsh(big.NewInt(1), uint(val.BitLen()))
	inverted.And(inverted, mask.Sub(mask, big.NewInt(1)))
	inverted.Add(inverted, big.NewInt(1))
	val.Neg(inverted)
	return val
}

type action func(t *task, batch []uint64, resultsChan chan *Bitmap, wg *sync.WaitGroup)

func parallelExecutor(parallelism int, t *task, e action, foundSet *Bitmap) *Bitmap {

	var n int = parallelism
	if n == 0 {
		n = runtime.NumCPU()
	}

	resultsChan := make(chan *Bitmap, n)

	card := foundSet.GetCardinality()
	x := card / uint64(n)

	remainder := card - (x * uint64(n))
	var batch []uint64
	var wg sync.WaitGroup
	iter := foundSet.ManyIterator()
	for i := 0; i < n; i++ {
		if i == n-1 {
			batch = make([]uint64, x+remainder)
		} else {
			batch = make([]uint64, x)
		}
		iter.NextMany(batch)
		wg.Add(1)
		go e(t, batch, resultsChan, &wg)
	}

	wg.Wait()

	close(resultsChan)

	ba := make([]*Bitmap, 0)
	for bm := range resultsChan {
		ba = append(ba, bm)
	}

	return ParOr(0, ba...)

}

type bsiAction func(input *BSI, filterSet *Bitmap, batch []uint64, resultsChan chan *BSI, wg *sync.WaitGroup)

func parallelExecutorBSIResults(parallelism int, input *BSI, e bsiAction, foundSet, filterSet *Bitmap, sumResults bool) *BSI {

	var n int = parallelism
	if n == 0 {
		n = runtime.NumCPU()
	}

	resultsChan := make(chan *BSI, n)

	card := foundSet.GetCardinality()
	x := card / uint64(n)

	remainder := card - (x * uint64(n))
	var batch []uint64
	var wg sync.WaitGroup
	iter := foundSet.ManyIterator()
	for i := 0; i < n; i++ {
		if i == n-1 {
			batch = make([]uint64, x+remainder)
		} else {
			batch = make([]uint64, x)
		}
		iter.NextMany(batch)
		wg.Add(1)
		go e(input, filterSet, batch, resultsChan, &wg)
	}

	wg.Wait()

	close(resultsChan)

	ba := make([]*BSI, 0)
	for bm := range resultsChan {
		ba = append(ba, bm)
	}

	results := NewDefaultBSI()
	if sumResults {
		for _, v := range ba {
			results.Add(v)
		}
	} else {
		results.ParOr(0, ba...)
	}
	return results

}

// Operation identifier
type Operation int

const (
	// LT less than
	LT Operation = 1 + iota
	// LE less than or equal
	LE
	// EQ equal
	EQ
	// GE greater than or equal
	GE
	// GT greater than
	GT
	// RANGE range
	RANGE
	// MIN find minimum
	MIN
	// MAX find maximum
	MAX
)

type task struct {
	bsi          *BSI
	op           Operation
	valueOrStart *big.Int
	end          *big.Int
	values       map[string]struct{}
	bits         *Bitmap
}

// CompareValue compares value.
// Values should be in the range of the BSI (max, min).  If the value is outside the range, the result
// might erroneous.  The operation parameter indicates the type of comparison to be made.
// For all operations with the exception of RANGE, the value to be compared is specified by valueOrStart.
// For the RANGE parameter the comparison criteria is >= valueOrStart and <= end.
// The parallelism parameter indicates the number of CPU threads to be applied for processing.  A value
// of zero indicates that all available CPU resources will be potentially utilized.
func (b *BSI) CompareValue(parallelism int, op Operation, valueOrStart, end int64,
	foundSet *Bitmap) *Bitmap {

	if result, ok := b.compareInt64Value(parallelism, op, valueOrStart, end, foundSet); ok {
		return result
	}
	return b.CompareBigValue(parallelism, op, big.NewInt(valueOrStart), big.NewInt(end), foundSet)
}

// CompareBSI compares values from two BSIs by column ID and returns the column
// IDs where b[columnID] op other[columnID] is true. Only column IDs present in
// both existence bitmaps are considered. When foundSet is not nil, it further
// restricts the comparison universe.
func (b *BSI) CompareBSI(op Operation, other *BSI, foundSet *Bitmap) *Bitmap {
	if b == nil || other == nil || b.eBM.IsEmpty() || other.eBM.IsEmpty() {
		return NewBitmap()
	}
	universe := b.eBM.Clone()
	universe.And(&other.eBM)
	if foundSet != nil {
		universe.And(foundSet)
	}
	if universe.IsEmpty() {
		return universe
	}

	commonSign := b.BitCount()
	if other.BitCount() > commonSign {
		commonSign = other.BitCount()
	}
	less, equal := b.compareBSILessAndEqual(other, commonSign, universe)

	switch op {
	case LT:
		return less
	case LE:
		less.Or(equal)
		return less
	case EQ:
		return equal
	case GE:
		universe.AndNot(less)
		return universe
	case GT:
		less.Or(equal)
		universe.AndNot(less)
		return universe
	default:
		panic(fmt.Sprintf("Operation [%v] not supported for BSI comparison", op))
	}
}

func (b *BSI) compareBSILessAndEqual(other *BSI, commonSign int, universe *Bitmap) (*Bitmap, *Bitmap) {
	less := NewBitmap()
	equalPrefix := universe.Clone()
	for i := commonSign; i >= 0; i-- {
		leftOnes := b.compareBSIPlaneChild(equalPrefix, i, commonSign, true, false)
		rightOnes := other.compareBSIPlaneChild(equalPrefix, i, commonSign, true, false)

		rightOnly := rightOnes.Clone()
		rightOnly.AndNot(leftOnes)
		less.Or(rightOnly)

		leftOnly := leftOnes
		leftOnly.AndNot(rightOnes)
		rightOnly.Or(leftOnly)
		equalPrefix.AndNot(rightOnly)
		if equalPrefix.IsEmpty() {
			break
		}
	}
	return less, equalPrefix
}

func (b *BSI) compareBSIPlaneChild(prefix *Bitmap, planeIndex, commonSign int, set, owned bool) *Bitmap {
	sourcePlane := planeIndex
	if sourcePlane > b.BitCount() {
		sourcePlane = b.BitCount()
	}
	rawSet := set
	if planeIndex == commonSign {
		rawSet = !rawSet
	}
	return bsi64PlaneChild(prefix, &b.bA[sourcePlane], rawSet, owned)
}

func (b *BSI) compareInt64Value(parallelism int, op Operation, valueOrStart, end int64, foundSet *Bitmap) (*Bitmap, bool) {
	bitCount := b.BitCount()
	if bitCount > 63 || !bsi64ValueFitsBitCount(valueOrStart, bitCount) {
		return nil, false
	}
	if op == EQ {
		result := b.BatchEqual(parallelism, []int64{valueOrStart})
		if foundSet != nil {
			result.And(foundSet)
		}
		return result, true
	}
	if op == RANGE && !bsi64ValueFitsBitCount(end, bitCount) {
		return nil, false
	}

	universe := b.eBM.Clone()
	if foundSet != nil {
		universe.And(foundSet)
	}
	if universe.IsEmpty() {
		return universe, true
	}

	start := transformBSI64SignedEncoding(encodeBSI64Value(valueOrStart, bitCount), bitCount)
	less, equal := b.compareInt64LessAndEqual(start, universe)

	switch op {
	case LT:
		return less, true
	case LE:
		less.Or(equal)
		return less, true
	case GE:
		universe.AndNot(less)
		return universe, true
	case GT:
		less.Or(equal)
		universe.AndNot(less)
		return universe, true
	case RANGE:
		if valueOrStart > end {
			return NewBitmap(), true
		}
		universe.AndNot(less)
		finish := transformBSI64SignedEncoding(encodeBSI64Value(end, bitCount), bitCount)
		rangeLess, rangeEqual := b.compareInt64LessAndEqual(finish, universe)
		rangeLess.Or(rangeEqual)
		return rangeLess, true
	default:
		return nil, false
	}
}

func transformBSI64SignedEncoding(encoded uint64, bitCount int) uint64 {
	return encoded ^ (uint64(1) << uint(bitCount))
}

func (b *BSI) compareInt64LessAndEqual(target uint64, universe *Bitmap) (*Bitmap, *Bitmap) {
	less := NewBitmap()
	equalPrefix := universe.Clone()
	for i := b.BitCount(); i >= 0; i-- {
		targetBitSet := target&(uint64(1)<<uint(i)) != 0
		if targetBitSet {
			less.Or(b.bsi64TransformedPlaneChild(equalPrefix, i, false, false))
			equalPrefix = b.bsi64TransformedPlaneChild(equalPrefix, i, true, true)
		} else {
			equalPrefix = b.bsi64TransformedPlaneChild(equalPrefix, i, false, true)
		}
		if equalPrefix.IsEmpty() {
			break
		}
	}
	return less, equalPrefix
}

func (b *BSI) bsi64TransformedPlaneChild(prefix *Bitmap, planeIndex int, set, owned bool) *Bitmap {
	planeSet := set
	if planeIndex == b.BitCount() {
		planeSet = !planeSet
	}
	return bsi64PlaneChild(prefix, &b.bA[planeIndex], planeSet, owned)
}

// CompareBigValue compares value.
// Values should be in the range of the BSI (max, min).  If the value is outside the range, the result
// might erroneous.  The operation parameter indicates the type of comparison to be made.
// For all operations with the exception of RANGE, the value to be compared is specified by valueOrStart.
// For the RANGE parameter the comparison criteria is >= valueOrStart and <= end.
// The parallelism parameter indicates the number of CPU threads to be applied for processing.  A value
// of zero indicates that all available CPU resources will be potentially utilized.
func (b *BSI) CompareBigValue(parallelism int, op Operation, valueOrStart, end *big.Int,
	foundSet *Bitmap) *Bitmap {

	if valueOrStart == nil {
		valueOrStart = b.MinMaxBig(parallelism, MIN, &b.eBM)
	}
	if end == nil && op == RANGE {
		end = b.MinMaxBig(parallelism, MAX, &b.eBM)
	}

	if result, ok := b.compareBigValueAsInt64(parallelism, op, valueOrStart, end, foundSet); ok {
		return result
	}

	comp := &task{bsi: b, op: op, valueOrStart: valueOrStart, end: end}
	if foundSet == nil {
		return parallelExecutor(parallelism, comp, compareValue, &b.eBM)
	}
	return parallelExecutor(parallelism, comp, compareValue, foundSet)
}

func (b *BSI) compareBigValueAsInt64(parallelism int, op Operation, valueOrStart, end *big.Int, foundSet *Bitmap) (*Bitmap, bool) {
	if valueOrStart == nil || !valueOrStart.IsInt64() {
		return nil, false
	}
	endValue := int64(0)
	if op == RANGE {
		if end == nil || !end.IsInt64() {
			return nil, false
		}
		endValue = end.Int64()
	}
	return b.compareInt64Value(parallelism, op, valueOrStart.Int64(), endValue, foundSet)
}

// Returns a twos complement value given a value, the return will be bit extended to 'bits' length
// if the value is negative
func twosComplement(num *big.Int, bitCount int) *big.Int {
	// Check if the number is negative
	isNegative := num.Sign() < 0

	// Get the absolute value if negative
	abs := new(big.Int).Abs(num)

	// Convert to binary string
	binStr := abs.Text(2)

	// Pad with zeros to the left
	if len(binStr) < bitCount {
		binStr = fmt.Sprintf("%0*s", bitCount, binStr)
	}

	// If negative, calculate two's complement
	if isNegative {
		// Invert bits
		inverted := make([]byte, len(binStr))
		for i := range binStr {
			if binStr[i] == '0' {
				inverted[i] = '1'
			} else {
				inverted[i] = '0'
			}
		}

		// Add 1
		carry := byte(1)
		for i := len(inverted) - 1; i >= 0; i-- {
			inverted[i] += carry
			if inverted[i] == '2' {
				inverted[i] = '0'
			} else {
				break
			}
		}
		binStr = string(inverted)
	}

	bigInt := new(big.Int)
	_, _ = bigInt.SetString(binStr, 2)
	return bigInt
}

func compareValue(e *task, batch []uint64, resultsChan chan *Bitmap, wg *sync.WaitGroup) {

	defer wg.Done()

	results := NewBitmap()
	if e.bsi.runOptimized {
		results.RunOptimize()
	}

	startIsNegative := e.valueOrStart.Sign() == -1
	endIsNegative := true
	if e.end != nil {
		endIsNegative = e.end.Sign() == -1
	}

	for i := 0; i < len(batch); i++ {
		cID := batch[i]
		eq1, eq2 := true, true
		lt1, lt2, gt1 := false, false, false
		j := e.bsi.BitCount()
		isNegative := e.bsi.IsNegative(cID)
		compStartValue := e.valueOrStart
		compEndValue := e.end
		if isNegative != startIsNegative {
			compStartValue = twosComplement(e.valueOrStart, e.bsi.BitCount()+1)
		}
		if isNegative != endIsNegative && e.end != nil {
			compEndValue = twosComplement(e.end, e.bsi.BitCount()+1)
		}

		for ; j >= 0; j-- {
			sliceContainsBit := e.bsi.bA[j].Contains(cID)

			if compStartValue.Bit(j) == 1 {
				// BIT in value is SET
				if !sliceContainsBit {
					if eq1 {
						if (e.op == GT || e.op == GE || e.op == RANGE) && startIsNegative && !isNegative {
							gt1 = true
						}
						if e.op == LT || e.op == LE {
							if !startIsNegative || (startIsNegative == isNegative) {
								lt1 = true
							}
						}
						eq1 = false
						if e.op != RANGE {
							break
						}
					}
				}
			} else {
				// BIT in value is CLEAR
				if sliceContainsBit {
					if eq1 {
						if (e.op == LT || e.op == LE) && isNegative && !startIsNegative {
							lt1 = true
						}
						if e.op == GT || e.op == GE || e.op == RANGE {
							if startIsNegative || (startIsNegative == isNegative) {
								gt1 = true
							}
						}
						eq1 = false

						if e.op != RANGE {
							break
						}
					}
				}
			}

			if e.op == RANGE && compEndValue.Bit(j) == 1 {
				// BIT in value is SET
				if !sliceContainsBit {
					if eq2 {
						if !endIsNegative || (endIsNegative == isNegative) {
							lt2 = true
						}
						eq2 = false
						if startIsNegative && !endIsNegative {
							break
						}
					}
				}
			} else if e.op == RANGE {
				// BIT in value is CLEAR
				if sliceContainsBit {
					if eq2 {
						if isNegative && !endIsNegative {
							lt2 = true
						}
						eq2 = false
					}
				}
			}
		}

		switch e.op {
		case LT:
			if lt1 {
				results.Add(cID)
			}
		case LE:
			if lt1 || (eq1 && (!startIsNegative || (startIsNegative && isNegative))) {
				results.Add(cID)
			}
		case EQ:
			if eq1 {
				results.Add(cID)
			}
		case GE:
			if gt1 || (eq1 && (startIsNegative || (!startIsNegative && !isNegative))) {
				results.Add(cID)
			}
		case GT:
			if gt1 {
				results.Add(cID)
			}
		case RANGE:
			if (eq1 || gt1) && (eq2 || lt2) {
				results.Add(cID)
			}
		default:
			panic(fmt.Sprintf("Operation [%v] not supported here", e.op))
		}
	}

	resultsChan <- results
}

// MinMax - Find minimum or maximum int64 value.
func (b *BSI) MinMax(parallelism int, op Operation, foundSet *Bitmap) int64 {
	return b.MinMaxBig(parallelism, op, foundSet).Int64()
}

// MinMaxBig - Find minimum or maximum value.
func (b *BSI) MinMaxBig(parallelism int, op Operation, foundSet *Bitmap) *big.Int {
	if foundSet == nil {
		foundSet = &b.eBM
	}

	candidates := And(foundSet, &b.eBM)
	var minMax *big.Int
	minSigned, maxSigned := minMaxSignedInt(b.BitCount() + 1)
	if op == MAX {
		minMax = minSigned
	} else {
		minMax = maxSigned
	}
	if candidates.IsEmpty() {
		return minMax
	}

	return b.minMaxBigByPlanes(op, candidates)
}

func (b *BSI) minMaxBigByPlanes(op Operation, candidates *Bitmap) *big.Int {
	signPlane := &b.bA[b.BitCount()]
	switch op {
	case MIN:
		negative := And(candidates, signPlane)
		if !negative.IsEmpty() {
			candidates = negative
		}
		for bit := b.BitCount() - 1; bit >= 0; bit-- {
			unset := AndNot(candidates, &b.bA[bit])
			if !unset.IsEmpty() {
				candidates = unset
				continue
			}
			candidates = And(candidates, &b.bA[bit])
		}
	case MAX:
		nonNegative := AndNot(candidates, signPlane)
		if !nonNegative.IsEmpty() {
			candidates = nonNegative
		}
		for bit := b.BitCount() - 1; bit >= 0; bit-- {
			set := And(candidates, &b.bA[bit])
			if !set.IsEmpty() {
				candidates = set
				continue
			}
			candidates = AndNot(candidates, &b.bA[bit])
		}
	default:
		panic(fmt.Sprintf("Operation [%v] not supported here", op))
	}
	value, _ := b.GetBigValue(candidates.Minimum())
	return value
}

func minMaxSignedInt(bits int) (*big.Int, *big.Int) {
	// Calculate the maximum value
	max := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	max.Sub(max, big.NewInt(1))

	// Calculate the minimum value
	min := new(big.Int).Neg(max)
	min.Sub(min, big.NewInt(1))

	return min, max
}

func (b *BSI) minOrMax(op Operation, batch []uint64, resultsChan chan *big.Int, wg *sync.WaitGroup) {

	defer wg.Done()

	x := b.BitCount() + 1
	var value *big.Int
	minSigned, maxSigned := minMaxSignedInt(x)
	if op == MAX {
		value = minSigned
	} else {
		value = maxSigned
	}

	for i := 0; i < len(batch); i++ {
		cID := batch[i]
		eq := true
		lt, gt := false, false
		j := b.BitCount()
		cVal := new(big.Int)
		valueIsNegative := value.Sign() == -1
		isNegative := b.IsNegative(cID)

		compValue := value
		if isNegative != valueIsNegative {
			// convert compValue to twos complement
			inverted := new(big.Int).Not(compValue)
			mask := new(big.Int).Lsh(big.NewInt(1), uint(compValue.BitLen()))
			inverted.And(inverted, mask.Sub(mask, big.NewInt(1)))
			inverted.Add(inverted, big.NewInt(1))
		}

		done := false
		for ; j >= 0; j-- {
			sliceContainsBit := b.bA[j].Contains(cID)
			if sliceContainsBit {
				bigBit := big.NewInt(1)
				bigBit.Lsh(bigBit, uint(j))
				cVal.Or(cVal, bigBit)
				if isNegative {
					cVal = negativeTwosComplementToInt(cVal)
				}
			}
			if done {
				continue
			}
			if compValue.Bit(j) == 1 {
				// BIT in value is SET
				if !sliceContainsBit {
					if eq {
						eq = false
						if op == MAX && valueIsNegative && !isNegative {
							gt = true
							done = true
						}
						if op == MIN && (!valueIsNegative || (valueIsNegative == isNegative)) {
							lt = true
						}
					}
				}
			} else {
				// BIT in value is CLEAR
				if sliceContainsBit {
					if eq {
						eq = false
						if op == MIN && isNegative && !valueIsNegative {
							lt = true
						}
						if op == MAX && (valueIsNegative || (valueIsNegative == isNegative)) {
							gt = true
							done = true
						}
					}
				}
			}
		}

		if lt || gt {
			value = cVal
		}
	}

	resultsChan <- value
}

// Sum all values contained within the foundSet.   As a convenience, the cardinality of the foundSet
// is also returned (for calculating the average).
func (b *BSI) Sum(foundSet *Bitmap) (int64, uint64) {
	val, count := b.SumBigValues(foundSet)
	return val.Int64(), count
}

// SumBigValues - Sum all values contained within the foundSet.   As a convenience, the cardinality of the foundSet
// is also returned (for calculating the average).   This method will sum arbitrarily large values.
func (b *BSI) SumBigValues(foundSet *Bitmap) (sum *big.Int, count uint64) {
	if foundSet == nil {
		foundSet = &b.eBM
	}
	sum = new(big.Int)
	count = foundSet.GetCardinality()
	resultsChan := make(chan int64, b.BitCount())
	var wg sync.WaitGroup
	for i := 0; i < b.BitCount(); i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			resultsChan <- int64(foundSet.AndCardinality(&b.bA[j]) << uint(j))
		}(i)
	}
	wg.Wait()
	close(resultsChan)

	for val := range resultsChan {
		sum.Add(sum, big.NewInt(val))
	}
	sum.Sub(sum, big.NewInt(int64(foundSet.AndCardinality(&b.bA[b.BitCount()])<<uint(b.BitCount()))))

	return sum, count
}

// Transpose calls b.IntersectAndTranspose(0, b.eBM)
func (b *BSI) Transpose() *Bitmap {
	return b.IntersectAndTranspose(0, &b.eBM)
}

// IntersectAndTranspose is a matrix transpose function.  Return a bitmap such that the values are represented as column IDs
// in the returned bitmap. This is accomplished by iterating over the foundSet and only including
// the column IDs in the source (foundSet) as compared with this BSI.  This can be useful for
// vectoring one set of integers to another.
//
// TODO: This implementation is functional but not performant, needs to be re-written perhaps using SIMD SSE2 instructions.
func (b *BSI) IntersectAndTranspose(parallelism int, foundSet *Bitmap) *Bitmap {
	if foundSet == nil {
		foundSet = &b.eBM
	}
	trans := &task{bsi: b}
	return parallelExecutor(parallelism, trans, transpose, foundSet)
}

func transpose(e *task, batch []uint64, resultsChan chan *Bitmap, wg *sync.WaitGroup) {

	defer wg.Done()

	results := NewBitmap()
	if e.bsi.runOptimized {
		results.RunOptimize()
	}
	for _, cID := range batch {
		if value, ok := e.bsi.GetValue(cID); ok {
			results.Add(uint64(value))
		}
	}
	resultsChan <- results
}

// ParOr is intended primarily to be a concatenation function to be used during bulk load operations.
// Care should be taken to make sure that columnIDs do not overlap (unless overlapping values are
// identical).
func (b *BSI) ParOr(parallelism int, bsis ...*BSI) {

	// Consolidate sets
	bits := len(b.bA)
	for i := 0; i < len(bsis); i++ {
		if len(bsis[i].bA) > bits {
			bits = len(bsis[i].bA)
		}
	}

	// Make sure we have enough bit slices
	for bits > len(b.bA) {
		bm := Bitmap{}
		bm.RunOptimize()
		b.bA = append(b.bA, bm)
	}

	a := make([][]*Bitmap, bits)
	for i := range a {
		a[i] = make([]*Bitmap, 0)
		for _, x := range bsis {
			if len(x.bA) > i {
				a[i] = append(a[i], &x.bA[i])
			} else {
				if b.runOptimized {
					a[i][0].RunOptimize()
				}
			}
		}
	}

	// Consolidate existence bit maps
	ebms := make([]*Bitmap, len(bsis))
	for i := range ebms {
		ebms[i] = &bsis[i].eBM
	}

	// First merge all the bit slices from all bsi maps that exist in target
	var wg sync.WaitGroup
	for i := 0; i < bits; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			x := []*Bitmap{&b.bA[j]}
			x = append(x, a[j]...)
			b.bA[j] = *ParOr(parallelism, x...)
		}(i)
	}
	wg.Wait()

	// merge all the EBM maps
	x := []*Bitmap{&b.eBM}
	x = append(x, ebms...)
	b.eBM = *ParOr(parallelism, x...)
}

// FromBitmaps initializes the BSI from a pre-built slice of bitmaps.
// bms[0] is the existence bitmap (eBM); bms[1:] are the bit planes in
// least-to-most-significant order (bit position 0 first), matching the
// layout used by MarshalBinary/UnmarshalBinary.
//
// The caller transfers ownership of the slice and all bitmaps within it;
// the BSI aliases the slice directly without copying. The caller must not
// modify the slice or any of its elements after this call.
//
// The no-copy design is intentional. The primary use case is deserialization
// pipelines where the existence bitmap is not stored on disk but reconstructed
// by ORing the bit planes, and all bitmaps are freshly allocated from the
// stream. Copying at that point would be wasteful. The caller's slice goes out
// of scope immediately after the call, so aliasing is safe.
//
// Panics if len(bms) < 1.
func (b *BSI) FromBitmaps(bms []Bitmap) {
	if len(bms) < 1 {
		panic("FromBitmaps: bms must have at least one element (the existence bitmap)")
	}
	b.eBM = bms[0]
	b.bA = bms[1:]
}

// UnmarshalBinary de-serialize a BSI.  The value at bitData[0] is the EBM.  Other indices are in least to most
// significance order starting at bitData[1] (bit position 0).
func (b *BSI) UnmarshalBinary(bitData [][]byte) error {

	for i := 1; i < len(bitData); i++ {
		if bitData == nil || len(bitData[i]) == 0 {
			continue
		}
		if b.BitCount() < i {
			newBm := Bitmap{}
			if b.runOptimized {
				newBm.RunOptimize()
			}
			b.bA = append(b.bA, newBm)
		}
		if err := b.bA[i-1].UnmarshalBinary(bitData[i]); err != nil {
			return err
		}
		if b.runOptimized {
			b.bA[i-1].RunOptimize()
		}

	}
	// First element of bitData is the EBM
	if bitData[0] == nil {
		b.eBM = Bitmap{}
		if b.runOptimized {
			b.eBM.RunOptimize()
		}
		return nil
	}
	if err := b.eBM.UnmarshalBinary(bitData[0]); err != nil {
		return err
	}
	if b.runOptimized {
		b.eBM.RunOptimize()
	}
	return nil
}

// ReadFrom reads a serialized version of this BSI from stream.
func (b *BSI) ReadFrom(stream io.Reader) (p int64, err error) {
	bm, n, err := readBSIContainerFromStream(stream)
	p += n
	if err != nil {
		err = fmt.Errorf("reading existence bitmap: %w", err)
		return
	}
	b.eBM = bm
	b.bA = b.bA[:0]
	for {
		// This forces a new memory location to be allocated and if we're lucky it only escapes if
		// there's no error.
		var bm Bitmap
		bm, n, err = readBSIContainerFromStream(stream)
		p += n
		if err == io.EOF {
			err = nil
			return
		}
		if err != nil {
			err = fmt.Errorf("reading bit slice index %v: %w", len(b.bA), err)
			return
		}
		b.bA = append(b.bA, bm)
	}
}

func readBSIContainerFromStream(r io.Reader) (bm Bitmap, p int64, err error) {
	p, err = bm.ReadFrom(r)
	return
}

// MarshalBinary serializes a BSI
func (b *BSI) MarshalBinary() ([][]byte, error) {

	var err error
	data := make([][]byte, b.BitCount()+1)
	// Add extra element for EBM (BitCount() + 1)
	for i := 1; i < b.BitCount()+1; i++ {
		data[i], err = b.bA[i-1].MarshalBinary()
		if err != nil {
			return nil, err
		}
	}
	// Marshal EBM
	data[0], err = b.eBM.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteTo writes a serialized version of this BSI to stream.
func (b *BSI) WriteTo(w io.Writer) (n int64, err error) {
	n1, err := b.eBM.WriteTo(w)
	n += n1
	if err != nil {
		return
	}
	for _, bm := range b.bA {
		n1, err = bm.WriteTo(w)
		n += n1
		if err != nil {
			return
		}
	}
	return
}

// BatchEqual returns a bitmap containing the column IDs where the values are contained within the list of values provided.
func (b *BSI) BatchEqual(parallelism int, values []int64) *Bitmap {
	if b.eBM.IsEmpty() || len(values) == 0 {
		return NewBitmap()
	}

	bitCount := b.BitCount()
	if bitCount >= 64 {
		// Fall back to the arbitrary-precision path when the BSI has more than
		// int64's finite bit width. This preserves correctness for big-value BSIs.
		bigValues := make([]*big.Int, len(values))
		for i, v := range values {
			bigValues[i] = big.NewInt(v)
		}
		return b.BatchEqualBig(parallelism, bigValues)
	}

	vals := b.batchEqualInt64Values(values, bitCount)
	if len(vals) == 0 {
		return NewBitmap()
	}

	if result, ok := b.matchInt64Cube(vals, bitCount); ok {
		if b.runOptimized {
			result.RunOptimize()
		}
		return result
	}
	result := b.matchInt64Trie(vals, bitCount, &b.eBM, false)
	if b.runOptimized {
		result.RunOptimize()
	}
	return result
}

// BatchEqualValues returns column IDs and values where the BSI value is
// contained in values. When foundSet is not nil, only column IDs in foundSet are
// considered. Result order is not guaranteed.
func (b *BSI) BatchEqualValues(parallelism int, values []int64, foundSet *Bitmap) []BSIValuePair {
	if b.eBM.IsEmpty() || len(values) == 0 {
		return nil
	}

	bitCount := b.BitCount()
	if bitCount >= 64 {
		matched := b.BatchEqual(parallelism, values)
		if foundSet != nil {
			matched.And(foundSet)
		}
		return b.bsiValuePairsFromBitmap(matched)
	}

	vals := b.batchEqualInt64Values(values, bitCount)
	if len(vals) == 0 {
		return nil
	}

	var universe *Bitmap
	owned := false
	if foundSet == nil {
		universe = &b.eBM
	} else {
		universe = And(&b.eBM, foundSet)
		owned = true
	}
	if universe.IsEmpty() {
		return nil
	}

	pairs := make([]BSIValuePair, 0)
	b.matchInt64TrieValues(vals, bitCount, universe, owned, 0, &pairs)
	return pairs
}

func (b *BSI) batchEqualInt64Values(values []int64, bitCount int) []uint64 {
	seen := make(map[uint64]struct{}, len(values))
	vals := make([]uint64, 0, len(values))
	for _, v := range values {
		if !bsi64ValueFitsBitCount(v, bitCount) {
			continue
		}
		encoded := encodeBSI64Value(v, bitCount)
		if _, ok := seen[encoded]; ok {
			continue
		}
		seen[encoded] = struct{}{}
		vals = append(vals, encoded)
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	return vals
}

func bsi64ValueFitsBitCount(value int64, bitCount int) bool {
	if bitCount >= 63 {
		return true
	}
	min := -(int64(1) << uint(bitCount))
	max := (int64(1) << uint(bitCount)) - 1
	return value >= min && value <= max
}

func encodeBSI64Value(value int64, bitCount int) uint64 {
	if bitCount >= 63 {
		return uint64(value)
	}
	mask := (uint64(1) << uint(bitCount+1)) - 1
	return uint64(value) & mask
}

func decodeBSI64Value(encoded uint64, bitCount int) int64 {
	if bitCount >= 63 {
		return int64(encoded)
	}
	width := uint(bitCount + 1)
	signMask := uint64(1) << uint(bitCount)
	if encoded&signMask != 0 && width < 64 {
		encoded |= ^uint64(0) << width
	}
	return int64(encoded)
}

func (b *BSI) matchInt64Cube(vals []uint64, bitCount int) (*Bitmap, bool) {
	if bitCount >= 63 {
		return nil, false
	}
	widthMask := (uint64(1) << uint(bitCount+1)) - 1
	fixedOnes := vals[0] & widthMask
	fixedZeros := ^vals[0] & widthMask
	for _, v := range vals[1:] {
		fixedOnes &= v
		fixedZeros &= ^v & widthMask
	}

	variableMask := ^(fixedOnes | fixedZeros) & widthMask
	combinations := uint64(1) << uint(countBSI64Bits(variableMask))
	if uint64(len(vals)) != combinations {
		return nil, false
	}
	for _, v := range vals {
		if v&fixedOnes != fixedOnes || (^v)&fixedZeros != fixedZeros {
			return nil, false
		}
	}

	result := b.eBM.Clone()
	for i := 0; i <= bitCount; i++ {
		bit := uint64(1) << uint(i)
		if variableMask&bit != 0 {
			continue
		}
		if fixedOnes&bit != 0 {
			result.And(&b.bA[i])
		} else {
			result.AndNot(&b.bA[i])
		}
		if result.IsEmpty() {
			break
		}
	}
	return result, true
}

func countBSI64Bits(value uint64) int {
	count := 0
	for value != 0 {
		value &= value - 1
		count++
	}
	return count
}

func (b *BSI) matchInt64Trie(vals []uint64, p int, prefix *Bitmap, owned bool) *Bitmap {
	if prefix.IsEmpty() {
		if owned {
			return prefix
		}
		return NewBitmap()
	}
	if p < 0 || (p < 63 && uint64(len(vals)) == uint64(1)<<uint(p+1)) {
		if owned {
			return prefix
		}
		return prefix.Clone()
	}

	mask := uint64(1) << uint(p)
	cut := sort.Search(len(vals), func(i int) bool { return vals[i]&mask != 0 })
	lo, hi := vals[:cut], vals[cut:]
	switch {
	case len(hi) == 0:
		return b.matchInt64Trie(lo, p-1, bsi64PlaneChild(prefix, &b.bA[p], false, owned), true)
	case len(lo) == 0:
		return b.matchInt64Trie(hi, p-1, bsi64PlaneChild(prefix, &b.bA[p], true, owned), true)
	default:
		hiBM := And(prefix, &b.bA[p])
		result := b.matchInt64Trie(lo, p-1, bsi64PlaneChild(prefix, &b.bA[p], false, owned), true)
		result.Or(b.matchInt64Trie(hi, p-1, hiBM, true))
		return result
	}
}

func (b *BSI) matchInt64TrieValues(vals []uint64, p int, prefix *Bitmap, owned bool, encoded uint64, pairs *[]BSIValuePair) {
	if prefix.IsEmpty() {
		return
	}
	if p < 0 {
		value := decodeBSI64Value(encoded, b.BitCount())
		iter := prefix.Iterator()
		for iter.HasNext() {
			*pairs = append(*pairs, BSIValuePair{
				ColumnID: iter.Next(),
				Value:    value,
			})
		}
		return
	}

	mask := uint64(1) << uint(p)
	cut := sort.Search(len(vals), func(i int) bool { return vals[i]&mask != 0 })
	lo, hi := vals[:cut], vals[cut:]
	switch {
	case len(hi) == 0:
		b.matchInt64TrieValues(lo, p-1, bsi64PlaneChild(prefix, &b.bA[p], false, owned), true, encoded, pairs)
	case len(lo) == 0:
		b.matchInt64TrieValues(hi, p-1, bsi64PlaneChild(prefix, &b.bA[p], true, owned), true, encoded|mask, pairs)
	default:
		hiBM := And(prefix, &b.bA[p])
		b.matchInt64TrieValues(lo, p-1, bsi64PlaneChild(prefix, &b.bA[p], false, owned), true, encoded, pairs)
		b.matchInt64TrieValues(hi, p-1, hiBM, true, encoded|mask, pairs)
	}
}

func (b *BSI) bsiValuePairsFromBitmap(matched *Bitmap) []BSIValuePair {
	if matched == nil || matched.IsEmpty() {
		return nil
	}
	columnIDs := matched.ToArray()
	values := b.GetBigValues(columnIDs)
	pairs := make([]BSIValuePair, 0, len(columnIDs))
	for i, columnID := range columnIDs {
		if i >= len(values) || values[i] == nil {
			continue
		}
		pairs = append(pairs, BSIValuePair{
			ColumnID: columnID,
			Value:    values[i].Int64(),
		})
	}
	return pairs
}

func bsi64PlaneChild(prefix, plane *Bitmap, set, owned bool) *Bitmap {
	if owned {
		if set {
			prefix.And(plane)
		} else {
			prefix.AndNot(plane)
		}
		return prefix
	}
	if set {
		return And(prefix, plane)
	}
	return AndNot(prefix, plane)
}

// BatchEqualBig returns a bitmap containing the column IDs where the values are contained within the list of values provided.
func (b *BSI) BatchEqualBig(parallelism int, values []*big.Int) *Bitmap {
	if b.eBM.IsEmpty() || len(values) == 0 {
		return NewBitmap()
	}

	if intValues, ok := b.batchEqualBigValuesAsInt64(values); ok {
		return b.BatchEqual(parallelism, intValues)
	}

	valMap := make(map[string]struct{}, len(values))
	for i := 0; i < len(values); i++ {
		if values[i] == nil {
			continue
		}
		valMap[batchEqualBigKey(values[i])] = struct{}{}
	}
	if len(valMap) == 0 {
		return NewBitmap()
	}
	comp := &task{bsi: b, values: valMap}
	return parallelExecutor(parallelism, comp, batchEqual, &b.eBM)
}

func batchEqualBigKey(value *big.Int) string {
	bytes := value.Bytes()
	key := make([]byte, len(bytes)+1)
	key[0] = byte(value.Sign() + 1)
	copy(key[1:], bytes)
	return string(key)
}

func (b *BSI) batchEqualBigValuesAsInt64(values []*big.Int) ([]int64, bool) {
	if b.BitCount() > 63 {
		return nil, false
	}
	intValues := make([]int64, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		if !value.IsInt64() {
			return nil, false
		}
		intValues = append(intValues, value.Int64())
	}
	if len(intValues) == 0 {
		return nil, false
	}
	return intValues, true
}

func batchEqual(e *task, batch []uint64, resultsChan chan *Bitmap,
	wg *sync.WaitGroup) {

	defer wg.Done()

	results := NewBitmap()
	if e.bsi.runOptimized {
		results.RunOptimize()
	}

	for i := 0; i < len(batch); i++ {
		cID := batch[i]
		if value, ok := e.bsi.GetBigValue(cID); ok {
			if _, yes := e.values[batchEqualBigKey(value)]; yes {
				results.Add(cID)
			}
		}
	}
	resultsChan <- results
}

// ClearBits cleared the bits that exist in the target if they are also in the found set.
func ClearBits(foundSet, target *Bitmap) {
	target.AndNot(foundSet)
}

// ClearValues removes from the BSI all values whose column IDs are in
// foundSet, modifying the BSI in place.
//
// The implementation is intentionally serial. A previous goroutine-per-bit-plane
// approach was slower in practice: goroutine creation overhead dominated for
// typical BSI sizes, and the cost compounds when ClearValues is called in a
// tight loop (e.g. once per term across an entire index during a deletion pass).
func (b *BSI) ClearValues(foundSet *Bitmap) {
	b.eBM.AndNot(foundSet)
	for i := range b.bA {
		b.bA[i].AndNot(foundSet)
	}
}

// Retain removes from the BSI all values whose column IDs are not in retain,
// modifying the BSI in place. It returns the number of column IDs dropped.
//
// This is the in-place equivalent of NewBSIRetainSet. Prefer it when no copy
// is needed, such as when the BSI will be immediately re-serialized — it
// avoids the allocation of a new BSI and all its bit planes.
//
// The bit planes (bA) are only updated when the existence bitmap actually
// shrinks. This is safe because BSI consistency guarantees that bA contains no
// set bits for column IDs absent from eBM; if eBM is unchanged after the
// intersection then retain covers all existing column IDs and bA needs no
// update.
func (b *BSI) Retain(retain *Bitmap) (dropped uint64) {
	preCard := b.eBM.GetCardinality()
	b.eBM.And(retain)
	dropped = preCard - b.eBM.GetCardinality()
	if dropped == 0 {
		return
	}
	for i := range b.bA {
		b.bA[i].And(retain)
	}
	return
}

// NewBSIRetainSet - Construct a new BSI from a clone of existing BSI, retain only values contained in foundSet
func (b *BSI) NewBSIRetainSet(foundSet *Bitmap) *BSI {

	newBSI := NewDefaultBSI()
	newBSI.bA = make([]Bitmap, b.BitCount()+1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		newBSI.eBM = *b.eBM.Clone()
		newBSI.eBM.And(foundSet)
	}()
	for i := 0; i < b.BitCount(); i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			newBSI.bA[j] = *b.bA[j].Clone()
			newBSI.bA[j].And(foundSet)
		}(i)
	}
	wg.Wait()
	return newBSI
}

// Clone performs a deep copy of BSI contents.
func (b *BSI) Clone() *BSI {
	return b.NewBSIRetainSet(&b.eBM)
}

// Add - In-place sum the contents of another BSI with this BSI, column wise.
func (b *BSI) Add(other *BSI) {

	b.eBM.Or(&other.eBM)
	for i := 0; i < len(other.bA); i++ {
		b.addDigit(&other.bA[i], i)
	}
}

func (b *BSI) addDigit(foundSet *Bitmap, i int) {

	if i >= b.BitCount()+1 || b.BitCount() == 0 {
		b.bA = append(b.bA, Bitmap{})
	}
	carry := And(&b.bA[i], foundSet)
	b.bA[i].Xor(foundSet)
	if !carry.IsEmpty() {
		if i+1 >= b.BitCount() {
			b.bA = append(b.bA, Bitmap{})
		}
		b.addDigit(carry, i+1)
	}
}

// TransposeWithCounts is a matrix transpose function that returns a BSI that has a columnID system defined by the values
// contained within the input BSI.   Given that for BSIs, different columnIDs can have the same value.  TransposeWithCounts
// is useful for situations where there is a one-to-many relationship between the vectored integer sets.  The resulting BSI
// contains the number of times a particular value appeared in the input BSI.
func (b *BSI) TransposeWithCounts(parallelism int, foundSet, filterSet *Bitmap) *BSI {
	if foundSet == nil {
		foundSet = &b.eBM
	}
	if filterSet == nil {
		filterSet = &b.eBM
	}
	return parallelExecutorBSIResults(parallelism, b, transposeWithCounts, foundSet, filterSet, true)
}

func transposeWithCounts(input *BSI, filterSet *Bitmap, batch []uint64, resultsChan chan *BSI, wg *sync.WaitGroup) {

	defer wg.Done()

	results := NewDefaultBSI()
	if input.runOptimized {
		results.RunOptimize()
	}
	for _, cID := range batch {
		if value, ok := input.GetValue(cID); ok {
			if !filterSet.Contains(uint64(value)) {
				continue
			}
			if val, ok2 := results.GetValue(uint64(value)); !ok2 {
				results.SetValue(uint64(value), 1)
			} else {
				val++
				results.SetValue(uint64(value), val)
			}
		}
	}
	resultsChan <- results
}

// Increment - In-place increment of values in a BSI.  Found set select columns for incrementing.
func (b *BSI) Increment(foundSet *Bitmap) {
	if foundSet == nil {
		foundSet = &b.eBM
	}
	b.addDigit(foundSet, 0)
	b.eBM.Or(foundSet)
}

// IncrementAll - In-place increment of all values in a BSI.
func (b *BSI) IncrementAll() {
	b.Increment(b.GetExistenceBitmap())
}

// Equals - Check for semantic equality of two BSIs.
func (b *BSI) Equals(other *BSI) bool {
	if !b.eBM.Equals(&other.eBM) {
		return false
	}
	for i := 0; i < len(b.bA) || i < len(other.bA); i++ {
		if i >= len(b.bA) {
			if !other.bA[i].IsEmpty() {
				return false
			}
		} else if i >= len(other.bA) {
			if !b.bA[i].IsEmpty() {
				return false
			}
		} else {
			if !b.bA[i].Equals(&other.bA[i]) {
				return false
			}
		}
	}
	return true
}

// GetSizeInBytes - the size in bytes of the data structure
func (b *BSI) GetSizeInBytes() int {
	size := b.eBM.GetSizeInBytes()
	for _, bm := range b.bA {
		size += bm.GetSizeInBytes()
	}
	return int(size)
}
