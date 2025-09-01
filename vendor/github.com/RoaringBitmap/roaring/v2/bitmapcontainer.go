package roaring

import (
	"errors"
	"fmt"
	"math/bits"
	"unsafe"
)

type bitmapContainer struct {
	cardinality int
	bitmap      []uint64
}

func (bc bitmapContainer) String() string {
	var s string
	for it := bc.getShortIterator(); it.hasNext(); {
		s += fmt.Sprintf("%v, ", it.next())
	}
	return s
}

func newBitmapContainer() *bitmapContainer {
	p := new(bitmapContainer)
	size := (1 << 16) / 64
	p.bitmap = make([]uint64, size, size)
	return p
}

func newBitmapContainerwithRange(firstOfRun, lastOfRun int) *bitmapContainer {
	bc := newBitmapContainer()
	bc.cardinality = lastOfRun - firstOfRun + 1
	if bc.cardinality == maxCapacity {
		fill(bc.bitmap, uint64(0xffffffffffffffff))
	} else {
		firstWord := firstOfRun / 64
		lastWord := lastOfRun / 64
		zeroPrefixLength := uint64(firstOfRun & 63)
		zeroSuffixLength := uint64(63 - (lastOfRun & 63))

		fillRange(bc.bitmap, firstWord, lastWord+1, uint64(0xffffffffffffffff))
		bc.bitmap[firstWord] ^= ((uint64(1) << zeroPrefixLength) - 1)
		blockOfOnes := (uint64(1) << zeroSuffixLength) - 1
		maskOnLeft := blockOfOnes << (uint64(64) - zeroSuffixLength)
		bc.bitmap[lastWord] ^= maskOnLeft
	}
	return bc
}

func (bc *bitmapContainer) minimum() uint16 {
	for i := 0; i < len(bc.bitmap); i++ {
		w := bc.bitmap[i]
		if w != 0 {
			r := countTrailingZeros(w)
			return uint16(r + i*64)
		}
	}
	return MaxUint16
}

func (bc *bitmapContainer) safeMinimum() (uint16, error) {
	if len(bc.bitmap) == 0 {
		return 0, errors.New("Empty bitmap")
	}
	val := bc.minimum()
	if val == MaxUint16 {
		return 0, errors.New("Empty bitmap")
	}
	return val, nil
}

// i should be non-zero
func clz(i uint64) int {
	n := 1
	x := uint32(i >> 32)
	if x == 0 {
		n += 32
		x = uint32(i)
	}
	if x>>16 == 0 {
		n += 16
		x = x << 16
	}
	if x>>24 == 0 {
		n += 8
		x = x << 8
	}
	if x>>28 == 0 {
		n += 4
		x = x << 4
	}
	if x>>30 == 0 {
		n += 2
		x = x << 2
	}
	return n - int(x>>31)
}

func (bc *bitmapContainer) maximum() uint16 {
	for i := len(bc.bitmap); i > 0; i-- {
		w := bc.bitmap[i-1]
		if w != 0 {
			r := clz(w)
			return uint16((i-1)*64 + 63 - r)
		}
	}
	return uint16(0)
}

func (bc *bitmapContainer) safeMaximum() (uint16, error) {
	if len(bc.bitmap) == 0 {
		return 0, errors.New("Empty bitmap")
	}
	val := bc.maximum()
	if val == uint16(0) {
		return 0, errors.New("Empty bitmap")
	}
	return val, nil
}

func (bc *bitmapContainer) iterate(cb func(x uint16) bool) bool {
	iterator := bitmapContainerShortIterator{bc, bc.NextSetBit(0)}

	for iterator.hasNext() {
		if !cb(iterator.next()) {
			return false
		}
	}

	return true
}

type bitmapContainerShortIterator struct {
	ptr *bitmapContainer
	i   int
}

func (bcsi *bitmapContainerShortIterator) next() uint16 {
	j := bcsi.i
	bcsi.i = bcsi.ptr.NextSetBit(uint(bcsi.i) + 1)
	return uint16(j)
}

func (bcsi *bitmapContainerShortIterator) hasNext() bool {
	return bcsi.i >= 0
}

func (bcsi *bitmapContainerShortIterator) peekNext() uint16 {
	return uint16(bcsi.i)
}

func (bcsi *bitmapContainerShortIterator) advanceIfNeeded(minval uint16) {
	if bcsi.hasNext() && bcsi.peekNext() < minval {
		bcsi.i = bcsi.ptr.NextSetBit(uint(minval))
	}
}

func newBitmapContainerShortIterator(a *bitmapContainer) *bitmapContainerShortIterator {
	return &bitmapContainerShortIterator{a, a.NextSetBit(0)}
}

func (bc *bitmapContainer) getShortIterator() shortPeekable {
	return newBitmapContainerShortIterator(bc)
}

type reverseBitmapContainerShortIterator struct {
	ptr *bitmapContainer
	i   int
}

func (bcsi *reverseBitmapContainerShortIterator) next() uint16 {
	if bcsi.i == -1 {
		panic("reverseBitmapContainerShortIterator.next() going beyond what is available")
	}

	j := bcsi.i
	bcsi.i = bcsi.ptr.PrevSetBit(bcsi.i - 1)
	return uint16(j)
}

func (bcsi *reverseBitmapContainerShortIterator) hasNext() bool {
	return bcsi.i >= 0
}

func newReverseBitmapContainerShortIterator(a *bitmapContainer) *reverseBitmapContainerShortIterator {
	if a.cardinality == 0 {
		return &reverseBitmapContainerShortIterator{a, -1}
	}
	return &reverseBitmapContainerShortIterator{a, int(a.maximum())}
}

func (bc *bitmapContainer) getReverseIterator() shortIterable {
	return newReverseBitmapContainerShortIterator(bc)
}

type bitmapContainerManyIterator struct {
	ptr    *bitmapContainer
	base   int
	bitset uint64
}

func (bcmi *bitmapContainerManyIterator) nextMany(hs uint32, buf []uint32) int {
	n := 0
	base := bcmi.base
	bitset := bcmi.bitset

	for n < len(buf) {
		if bitset == 0 {
			base++
			if base >= len(bcmi.ptr.bitmap) {
				bcmi.base = base
				bcmi.bitset = bitset
				return n
			}
			bitset = bcmi.ptr.bitmap[base]
			continue
		}
		t := bitset & -bitset
		buf[n] = uint32(((base * 64) + int(popcount(t-1)))) | hs
		n = n + 1
		bitset ^= t
	}

	bcmi.base = base
	bcmi.bitset = bitset
	return n
}

// nextMany64 returns the number of values added to the buffer
func (bcmi *bitmapContainerManyIterator) nextMany64(hs uint64, buf []uint64) int {
	n := 0
	base := bcmi.base
	bitset := bcmi.bitset

	for n < len(buf) {
		if bitset == 0 {
			base++
			if base >= len(bcmi.ptr.bitmap) {
				bcmi.base = base
				bcmi.bitset = bitset
				return n
			}
			bitset = bcmi.ptr.bitmap[base]
			continue
		}
		t := bitset & -bitset
		buf[n] = uint64(((base * 64) + int(popcount(t-1)))) | hs
		n = n + 1
		bitset ^= t
	}

	bcmi.base = base
	bcmi.bitset = bitset
	return n
}

func newBitmapContainerManyIterator(a *bitmapContainer) *bitmapContainerManyIterator {
	return &bitmapContainerManyIterator{a, -1, 0}
}

func (bc *bitmapContainer) getManyIterator() manyIterable {
	return newBitmapContainerManyIterator(bc)
}

func (bc *bitmapContainer) getSizeInBytes() int {
	return len(bc.bitmap) * 8
}

func (bc *bitmapContainer) serializedSizeInBytes() int {
	return len(bc.bitmap) * 8
}

const bcBaseBytes = int(unsafe.Sizeof(bitmapContainer{}))

// bitmapContainer doesn't depend on card, always fully allocated
func bitmapContainerSizeInBytes() int {
	return bcBaseBytes + (1<<16)/8
}

func bitmapEquals(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func (bc *bitmapContainer) fillLeastSignificant16bits(x []uint32, i int, mask uint32) int {
	// TODO: should be written as optimized assembly
	pos := i
	base := mask
	for k := 0; k < len(bc.bitmap); k++ {
		bitset := bc.bitmap[k]
		for bitset != 0 {
			t := bitset & -bitset
			x[pos] = base + uint32(popcount(t-1))
			pos++
			bitset ^= t
		}
		base += 64
	}
	return pos
}

func (bc *bitmapContainer) equals(o container) bool {
	srb, ok := o.(*bitmapContainer)
	if ok {
		if srb.cardinality != bc.cardinality {
			return false
		}
		return bitmapEquals(bc.bitmap, srb.bitmap)
	}

	// use generic comparison
	if bc.getCardinality() != o.getCardinality() {
		return false
	}
	ait := o.getShortIterator()
	bit := bc.getShortIterator()

	for ait.hasNext() {
		if bit.next() != ait.next() {
			return false
		}
	}
	return true
}

func (bc *bitmapContainer) iaddReturnMinimized(i uint16) container {
	bc.iadd(i)
	if bc.isFull() {
		return newRunContainer16Range(0, MaxUint16)
	}
	return bc
}

// iadd adds the arg i, returning true if not already present
func (bc *bitmapContainer) iadd(i uint16) bool {
	x := int(i)
	previous := bc.bitmap[x/64]
	mask := uint64(1) << (uint(x) % 64)
	newb := previous | mask
	bc.bitmap[x/64] = newb
	bc.cardinality += int((previous ^ newb) >> (uint(x) % 64))
	return newb != previous
}

func (bc *bitmapContainer) iremoveReturnMinimized(i uint16) container {
	if bc.iremove(i) {
		if bc.cardinality == arrayDefaultMaxSize {
			return bc.toArrayContainer()
		}
	}
	return bc
}

// iremove returns true if i was found.
func (bc *bitmapContainer) iremove(i uint16) bool {
	if bc.contains(i) {
		bc.cardinality--
		bc.bitmap[i/64] &^= (uint64(1) << (i % 64))
		return true
	}
	return false
}

func (bc *bitmapContainer) isFull() bool {
	return bc.cardinality == int(MaxUint16)+1
}

func (bc *bitmapContainer) getCardinality() int {
	return bc.cardinality
}

func (bc *bitmapContainer) isEmpty() bool {
	return bc.cardinality == 0
}

func (bc *bitmapContainer) clone() container {
	ptr := bitmapContainer{bc.cardinality, make([]uint64, len(bc.bitmap))}
	copy(ptr.bitmap, bc.bitmap[:])
	return &ptr
}

// add all values in range [firstOfRange,lastOfRange)
func (bc *bitmapContainer) iaddRange(firstOfRange, lastOfRange int) container {
	bc.cardinality += setBitmapRangeAndCardinalityChange(bc.bitmap, firstOfRange, lastOfRange)
	return bc
}

// remove all values in range [firstOfRange,lastOfRange)
func (bc *bitmapContainer) iremoveRange(firstOfRange, lastOfRange int) container {
	bc.cardinality += resetBitmapRangeAndCardinalityChange(bc.bitmap, firstOfRange, lastOfRange)
	if bc.getCardinality() <= arrayDefaultMaxSize {
		return bc.toArrayContainer()
	}
	return bc
}

// flip all values in range [firstOfRange,endx)
func (bc *bitmapContainer) inot(firstOfRange, endx int) container {
	if endx-firstOfRange == maxCapacity {
		flipBitmapRange(bc.bitmap, firstOfRange, endx)
		bc.cardinality = maxCapacity - bc.cardinality
	} else if endx-firstOfRange > maxCapacity/2 {
		flipBitmapRange(bc.bitmap, firstOfRange, endx)
		bc.computeCardinality()
	} else {
		bc.cardinality += flipBitmapRangeAndCardinalityChange(bc.bitmap, firstOfRange, endx)
	}
	if bc.getCardinality() <= arrayDefaultMaxSize {
		return bc.toArrayContainer()
	}
	return bc
}

// flip all values in range [firstOfRange,endx)
func (bc *bitmapContainer) not(firstOfRange, endx int) container {
	answer := bc.clone()
	return answer.inot(firstOfRange, endx)
}

func (bc *bitmapContainer) or(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.orArray(x)
	case *bitmapContainer:
		return bc.orBitmap(x)
	case *runContainer16:
		if x.isFull() {
			return x.clone()
		}
		return x.orBitmapContainer(bc)
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) orCardinality(a container) int {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.orArrayCardinality(x)
	case *bitmapContainer:
		return bc.orBitmapCardinality(x)
	case *runContainer16:
		return x.orBitmapContainerCardinality(bc)
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) ior(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.iorArray(x)
	case *bitmapContainer:
		return bc.iorBitmap(x)
	case *runContainer16:
		if x.isFull() {
			return x.clone()
		}
		for i := range x.iv {
			bc.iaddRange(int(x.iv[i].start), int(x.iv[i].last())+1)
		}
		if bc.isFull() {
			return newRunContainer16Range(0, MaxUint16)
		}
		// bc.computeCardinality()
		return bc
	}
	panic(fmt.Errorf("unsupported container type %T", a))
}

func (bc *bitmapContainer) lazyIOR(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.lazyIORArray(x)
	case *bitmapContainer:
		return bc.lazyIORBitmap(x)
	case *runContainer16:
		if x.isFull() {
			return x.clone()
		}

		// Manually inlined setBitmapRange function
		bitmap := bc.bitmap
		for _, iv := range x.iv {
			start := int(iv.start)
			end := int(iv.last()) + 1
			if start >= end {
				continue
			}
			firstword := start / 64
			endword := (end - 1) / 64
			if firstword == endword {
				bitmap[firstword] |= (^uint64(0) << uint(start%64)) & (^uint64(0) >> (uint(-end) % 64))
				continue
			}
			bitmap[firstword] |= ^uint64(0) << uint(start%64)
			for i := firstword + 1; i < endword; i++ {
				bitmap[i] = ^uint64(0)
			}
			bitmap[endword] |= ^uint64(0) >> (uint(-end) % 64)
		}
		bc.cardinality = invalidCardinality
		return bc
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) lazyOR(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.lazyORArray(x)
	case *bitmapContainer:
		return bc.lazyORBitmap(x)
	case *runContainer16:
		if x.isFull() {
			return x.clone()
		}
		// TODO: implement lazy OR
		return x.orBitmapContainer(bc)

	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) orArray(value2 *arrayContainer) container {
	answer := bc.clone().(*bitmapContainer)
	c := value2.getCardinality()
	for k := 0; k < c; k++ {
		v := value2.content[k]
		i := uint(v) >> 6
		bef := answer.bitmap[i]
		aft := bef | (uint64(1) << (v % 64))
		answer.bitmap[i] = aft
		answer.cardinality += int((bef - aft) >> 63)
	}
	return answer
}

func (bc *bitmapContainer) orArrayCardinality(value2 *arrayContainer) int {
	answer := bc.getCardinality()
	c := value2.getCardinality()
	for k := 0; k < c; k++ {
		// branchless:
		v := value2.content[k]
		i := uint(v) >> 6
		bef := bc.bitmap[i]
		aft := bef | (uint64(1) << (v % 64))
		answer += int((bef - aft) >> 63)
	}
	return answer
}

func (bc *bitmapContainer) orBitmap(value2 *bitmapContainer) container {
	answer := newBitmapContainer()
	for k := 0; k < len(answer.bitmap); k++ {
		answer.bitmap[k] = bc.bitmap[k] | value2.bitmap[k]
	}
	answer.computeCardinality()
	if answer.isFull() {
		return newRunContainer16Range(0, MaxUint16)
	}
	return answer
}

func (bc *bitmapContainer) orBitmapCardinality(value2 *bitmapContainer) int {
	return int(popcntOrSlice(bc.bitmap, value2.bitmap))
}

func (bc *bitmapContainer) andBitmapCardinality(value2 *bitmapContainer) int {
	return int(popcntAndSlice(bc.bitmap, value2.bitmap))
}

func (bc *bitmapContainer) computeCardinality() {
	bc.cardinality = int(popcntSlice(bc.bitmap))
}

func (bc *bitmapContainer) iorArray(ac *arrayContainer) container {
	for k := range ac.content {
		vc := ac.content[k]
		i := uint(vc) >> 6
		bef := bc.bitmap[i]
		aft := bef | (uint64(1) << (vc % 64))
		bc.bitmap[i] = aft
		bc.cardinality += int((bef - aft) >> 63)
	}
	if bc.isFull() {
		return newRunContainer16Range(0, MaxUint16)
	}
	return bc
}

func (bc *bitmapContainer) iorBitmap(value2 *bitmapContainer) container {
	answer := bc
	answer.cardinality = 0
	for k := 0; k < len(answer.bitmap); k++ {
		answer.bitmap[k] = bc.bitmap[k] | value2.bitmap[k]
	}
	answer.computeCardinality()
	if bc.isFull() {
		return newRunContainer16Range(0, MaxUint16)
	}
	return answer
}

func (bc *bitmapContainer) lazyIORArray(value2 *arrayContainer) container {
	answer := bc
	c := value2.getCardinality()
	for k := 0; k+3 < c; k += 4 {
		content := (*[4]uint16)(unsafe.Pointer(&value2.content[k]))
		vc0 := content[0]
		i0 := uint(vc0) >> 6
		answer.bitmap[i0] = answer.bitmap[i0] | (uint64(1) << (vc0 % 64))

		vc1 := content[1]
		i1 := uint(vc1) >> 6
		answer.bitmap[i1] = answer.bitmap[i1] | (uint64(1) << (vc1 % 64))

		vc2 := content[2]
		i2 := uint(vc2) >> 6
		answer.bitmap[i2] = answer.bitmap[i2] | (uint64(1) << (vc2 % 64))

		vc3 := content[3]
		i3 := uint(vc3) >> 6
		answer.bitmap[i3] = answer.bitmap[i3] | (uint64(1) << (vc3 % 64))
	}

	for k := c &^ 3; k < c; k++ {
		vc := value2.content[k]
		i := uint(vc) >> 6
		answer.bitmap[i] = answer.bitmap[i] | (uint64(1) << (vc % 64))
	}

	answer.cardinality = invalidCardinality
	return answer
}

func (bc *bitmapContainer) lazyORArray(value2 *arrayContainer) container {
	answer := bc.clone().(*bitmapContainer)
	return answer.lazyIORArray(value2)
}

func (bc *bitmapContainer) lazyIORBitmap(value2 *bitmapContainer) container {
	answer := bc
	for k := 0; k < len(answer.bitmap); k++ {
		answer.bitmap[k] = bc.bitmap[k] | value2.bitmap[k]
	}
	bc.cardinality = invalidCardinality
	return answer
}

func (bc *bitmapContainer) lazyORBitmap(value2 *bitmapContainer) container {
	answer := bc.clone().(*bitmapContainer)
	return answer.lazyIORBitmap(value2)
}

func (bc *bitmapContainer) xor(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.xorArray(x)
	case *bitmapContainer:
		return bc.xorBitmap(x)
	case *runContainer16:
		return x.xorBitmap(bc)
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) xorArray(value2 *arrayContainer) container {
	answer := bc.clone().(*bitmapContainer)
	c := value2.getCardinality()
	for k := 0; k < c; k++ {
		vc := value2.content[k]
		index := uint(vc) >> 6
		abi := answer.bitmap[index]
		mask := uint64(1) << (vc % 64)
		answer.cardinality += 1 - 2*int((abi&mask)>>(vc%64))
		answer.bitmap[index] = abi ^ mask
	}
	if answer.cardinality <= arrayDefaultMaxSize {
		return answer.toArrayContainer()
	}
	return answer
}

func (bc *bitmapContainer) rank(x uint16) int {
	// TODO: rewrite in assembly
	leftover := (uint(x) + 1) & 63
	if leftover == 0 {
		return int(popcntSlice(bc.bitmap[:(uint(x)+1)/64]))
	}
	return int(popcntSlice(bc.bitmap[:(uint(x)+1)/64]) + popcount(bc.bitmap[(uint(x)+1)/64]<<(64-leftover)))
}

func (bc *bitmapContainer) selectInt(x uint16) int {
	remaining := x
	for k := 0; k < len(bc.bitmap); k++ {
		w := popcount(bc.bitmap[k])
		if uint16(w) > remaining {
			return k*64 + selectBitPosition(bc.bitmap[k], int(remaining))
		}
		remaining -= uint16(w)
	}
	return -1
}

func (bc *bitmapContainer) xorBitmap(value2 *bitmapContainer) container {
	newCardinality := int(popcntXorSlice(bc.bitmap, value2.bitmap))

	if newCardinality > arrayDefaultMaxSize {
		answer := newBitmapContainer()
		for k := 0; k < len(answer.bitmap); k++ {
			answer.bitmap[k] = bc.bitmap[k] ^ value2.bitmap[k]
		}
		answer.cardinality = newCardinality
		if answer.isFull() {
			return newRunContainer16Range(0, MaxUint16)
		}
		return answer
	}
	ac := newArrayContainerSize(newCardinality)
	fillArrayXOR(ac.content, bc.bitmap, value2.bitmap)
	ac.content = ac.content[:newCardinality]
	return ac
}

func (bc *bitmapContainer) and(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.andArray(x)
	case *bitmapContainer:
		return bc.andBitmap(x)
	case *runContainer16:
		if x.isFull() {
			return bc.clone()
		}
		return x.andBitmapContainer(bc)
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) andCardinality(a container) int {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.andArrayCardinality(x)
	case *bitmapContainer:
		return bc.andBitmapCardinality(x)
	case *runContainer16:
		return x.andBitmapContainerCardinality(bc)
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) intersects(a container) bool {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.intersectsArray(x)
	case *bitmapContainer:
		return bc.intersectsBitmap(x)
	case *runContainer16:
		return x.intersects(bc)

	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) iand(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.iandArray(x)
	case *bitmapContainer:
		return bc.iandBitmap(x)
	case *runContainer16:
		if x.isFull() {
			return bc.clone()
		}
		return bc.iandRun16(x)
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) iandRun16(rc *runContainer16) container {
	rcb := newBitmapContainerFromRun(rc)
	return bc.iandBitmap(rcb)
}

func (bc *bitmapContainer) iandArray(ac *arrayContainer) container {
	acb := ac.toBitmapContainer()
	return bc.iandBitmap(acb)
}

func (bc *bitmapContainer) andArray(value2 *arrayContainer) *arrayContainer {
	answer := newArrayContainerCapacity(len(value2.content))
	answer.content = answer.content[:cap(answer.content)]
	c := value2.getCardinality()
	pos := 0
	for k := 0; k < c; k++ {
		v := value2.content[k]
		answer.content[pos] = v
		pos += int(bc.bitValue(v))
	}
	answer.content = answer.content[:pos]
	return answer
}

func (bc *bitmapContainer) andArrayCardinality(value2 *arrayContainer) int {
	c := value2.getCardinality()
	pos := 0
	for k := 0; k < c; k++ {
		v := value2.content[k]
		pos += int(bc.bitValue(v))
	}
	return pos
}

func (bc *bitmapContainer) getCardinalityInRange(start, end uint) int {
	if start >= end {
		return 0
	}
	firstword := start / 64
	endword := (end - 1) / 64
	const allones = ^uint64(0)
	if firstword == endword {
		return int(popcount(bc.bitmap[firstword] & ((allones << (start % 64)) & (allones >> ((64 - end) & 63)))))
	}
	answer := popcount(bc.bitmap[firstword] & (allones << (start % 64)))
	answer += popcntSlice(bc.bitmap[firstword+1 : endword])
	answer += popcount(bc.bitmap[endword] & (allones >> ((64 - end) & 63)))
	return int(answer)
}

func (bc *bitmapContainer) andBitmap(value2 *bitmapContainer) container {
	newcardinality := int(popcntAndSlice(bc.bitmap, value2.bitmap))
	if newcardinality > arrayDefaultMaxSize {
		answer := newBitmapContainer()
		for k := 0; k < len(answer.bitmap); k++ {
			answer.bitmap[k] = bc.bitmap[k] & value2.bitmap[k]
		}
		answer.cardinality = newcardinality
		return answer
	}
	ac := newArrayContainerSize(newcardinality)
	fillArrayAND(ac.content, bc.bitmap, value2.bitmap)
	ac.content = ac.content[:newcardinality]
	return ac
}

func (bc *bitmapContainer) intersectsArray(value2 *arrayContainer) bool {
	c := value2.getCardinality()
	for k := 0; k < c; k++ {
		v := value2.content[k]
		if bc.contains(v) {
			return true
		}
	}
	return false
}

func (bc *bitmapContainer) intersectsBitmap(value2 *bitmapContainer) bool {
	for k := 0; k < len(bc.bitmap); k++ {
		if (bc.bitmap[k] & value2.bitmap[k]) != 0 {
			return true
		}
	}
	return false
}

func (bc *bitmapContainer) iandBitmap(value2 *bitmapContainer) container {
	newcardinality := int(popcntAndSlice(bc.bitmap, value2.bitmap))
	for k := 0; k < len(bc.bitmap); k++ {
		bc.bitmap[k] = bc.bitmap[k] & value2.bitmap[k]
	}
	bc.cardinality = newcardinality

	if newcardinality <= arrayDefaultMaxSize {
		return newArrayContainerFromBitmap(bc)
	}
	return bc
}

func (bc *bitmapContainer) andNot(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.andNotArray(x)
	case *bitmapContainer:
		return bc.andNotBitmap(x)
	case *runContainer16:
		return bc.andNotRun16(x)
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) andNotRun16(rc *runContainer16) container {
	rcb := rc.toBitmapContainer()
	return bc.andNotBitmap(rcb)
}

func (bc *bitmapContainer) iandNot(a container) container {
	switch x := a.(type) {
	case *arrayContainer:
		return bc.iandNotArray(x)
	case *bitmapContainer:
		return bc.iandNotBitmapSurely(x)
	case *runContainer16:
		return bc.iandNotRun16(x)
	}
	panic("unsupported container type")
}

func (bc *bitmapContainer) iandNotArray(ac *arrayContainer) container {
	if ac.isEmpty() || bc.isEmpty() {
		// Nothing to do.
		return bc
	}

	// Word by word, we remove the elements in ac from bc. The approach is to build
	// a mask of the elements to remove, and then apply it to the bitmap.
	wordIdx := uint16(0)
	mask := uint64(0)
	for i, v := range ac.content {
		if v/64 != wordIdx {
			// Flush the current word.
			if i != 0 {
				// We're removing bits that are set in the mask and in the current word.
				// To figure out the cardinality change, we count the number of bits that
				// are set in the mask and in the current word.
				mask &= bc.bitmap[wordIdx]
				bc.bitmap[wordIdx] &= ^mask
				bc.cardinality -= int(popcount(mask))
			}

			wordIdx = v / 64
			mask = 0
		}
		mask |= 1 << (v % 64)
	}

	// Flush the last word.
	mask &= bc.bitmap[wordIdx]
	bc.bitmap[wordIdx] &= ^mask
	bc.cardinality -= int(popcount(mask))

	if bc.getCardinality() <= arrayDefaultMaxSize {
		return bc.toArrayContainer()
	}
	return bc
}

func (bc *bitmapContainer) iandNotRun16(rc *runContainer16) container {
	if rc.isEmpty() || bc.isEmpty() {
		// Nothing to do.
		return bc
	}

	wordRangeStart := rc.iv[0].start / 64
	wordRangeEnd := (rc.iv[len(rc.iv)-1].last()) / 64 // inclusive

	cardinalityChange := popcntSlice(bc.bitmap[wordRangeStart : wordRangeEnd+1]) // before cardinality - after cardinality (for word range)

	for _, iv := range rc.iv {
		resetBitmapRange(bc.bitmap, int(iv.start), int(iv.last())+1)
	}

	cardinalityChange -= popcntSlice(bc.bitmap[wordRangeStart : wordRangeEnd+1])

	bc.cardinality -= int(cardinalityChange)

	if bc.getCardinality() <= arrayDefaultMaxSize {
		return bc.toArrayContainer()
	}
	return bc
}

func (bc *bitmapContainer) andNotArray(value2 *arrayContainer) container {
	answer := bc.clone().(*bitmapContainer)
	c := value2.getCardinality()
	for k := 0; k < c; k++ {
		vc := value2.content[k]
		i := uint(vc) >> 6
		oldv := answer.bitmap[i]
		newv := oldv &^ (uint64(1) << (vc % 64))
		answer.bitmap[i] = newv
		answer.cardinality -= int((oldv ^ newv) >> (vc % 64))
	}
	if answer.cardinality <= arrayDefaultMaxSize {
		return answer.toArrayContainer()
	}
	return answer
}

func (bc *bitmapContainer) andNotBitmap(value2 *bitmapContainer) container {
	newCardinality := int(popcntMaskSlice(bc.bitmap, value2.bitmap))
	if newCardinality > arrayDefaultMaxSize {
		answer := newBitmapContainer()
		for k := 0; k < len(answer.bitmap); k++ {
			answer.bitmap[k] = bc.bitmap[k] &^ value2.bitmap[k]
		}
		answer.cardinality = newCardinality
		return answer
	}
	ac := newArrayContainerSize(newCardinality)
	fillArrayANDNOT(ac.content, bc.bitmap, value2.bitmap)
	return ac
}

func (bc *bitmapContainer) iandNotBitmapSurely(value2 *bitmapContainer) container {
	newCardinality := int(popcntMaskSlice(bc.bitmap, value2.bitmap))
	for k := 0; k < len(bc.bitmap); k++ {
		bc.bitmap[k] = bc.bitmap[k] &^ value2.bitmap[k]
	}
	bc.cardinality = newCardinality
	if bc.getCardinality() <= arrayDefaultMaxSize {
		return bc.toArrayContainer()
	}
	return bc
}

func (bc *bitmapContainer) contains(i uint16) bool { // testbit
	x := uint(i)
	w := bc.bitmap[x>>6]
	mask := uint64(1) << (x & 63)
	return (w & mask) != 0
}

func (bc *bitmapContainer) bitValue(i uint16) uint64 {
	x := uint(i)
	w := bc.bitmap[x>>6]
	return (w >> (x & 63)) & 1
}

func (bc *bitmapContainer) loadData(arrayContainer *arrayContainer) {
	bc.cardinality = arrayContainer.getCardinality()
	c := arrayContainer.getCardinality()
	for k := 0; k < c; k++ {
		x := arrayContainer.content[k]
		i := int(x) / 64
		bc.bitmap[i] |= (uint64(1) << uint(x%64))
	}
}

func (bc *bitmapContainer) resetTo(a container) {
	switch x := a.(type) {
	case *arrayContainer:
		fill(bc.bitmap, 0)
		bc.loadData(x)

	case *bitmapContainer:
		bc.cardinality = x.cardinality
		copy(bc.bitmap, x.bitmap)

	case *runContainer16:
		bc.cardinality = len(x.iv)
		lastEnd := 0
		for _, r := range x.iv {
			bc.cardinality += int(r.length)
			resetBitmapRange(bc.bitmap, lastEnd, int(r.start))
			lastEnd = int(r.start+r.length) + 1
			setBitmapRange(bc.bitmap, int(r.start), lastEnd)
		}
		resetBitmapRange(bc.bitmap, lastEnd, maxCapacity)

	default:
		panic("unsupported container type")
	}
}

func (bc *bitmapContainer) toArrayContainer() *arrayContainer {
	ac := &arrayContainer{}
	ac.loadData(bc)
	return ac
}

func (bc *bitmapContainer) fillArray(container []uint16) {
	// TODO: rewrite in assembly
	pos := 0
	base := 0
	for k := 0; k < len(bc.bitmap); k++ {
		bitset := bc.bitmap[k]
		for bitset != 0 {
			t := bitset & -bitset
			container[pos] = uint16((base + int(popcount(t-1))))
			pos = pos + 1
			bitset ^= t
		}
		base += 64
	}
}

// NextSetBit returns the next set bit e.g the next int packed into the bitmaparray
func (bc *bitmapContainer) NextSetBit(i uint) int {
	var (
		x      = i / 64
		length = uint(len(bc.bitmap))
	)
	if x >= length {
		return -1
	}
	w := bc.bitmap[x]
	w = w >> uint(i%64)
	if w != 0 {
		return int(i) + countTrailingZeros(w)
	}
	x++
	for ; x < length; x++ {
		if bc.bitmap[x] != 0 {
			return int(x*64) + countTrailingZeros(bc.bitmap[x])
		}
	}
	return -1
}

// PrevSetBit returns the previous set bit e.g the previous int packed into the bitmaparray
func (bc *bitmapContainer) PrevSetBit(i int) int {
	if i < 0 {
		return -1
	}

	return bc.uPrevSetBit(uint(i))
}

func (bc *bitmapContainer) uPrevSetBit(i uint) int {
	var (
		x      = i >> 6
		length = uint(len(bc.bitmap))
	)

	if x >= length {
		return -1
	}

	w := bc.bitmap[x]

	b := i % 64

	w = w << uint(63-b)
	if w != 0 {
		return int(i) - countLeadingZeros(w)
	}
	orig := x
	x--
	if x > orig {
		return -1
	}
	for ; x < orig; x-- {
		if bc.bitmap[x] != 0 {
			return int((x*64)+63) - countLeadingZeros(bc.bitmap[x])
		}
	}
	return -1
}

// reference the java implementation
// https://github.com/RoaringBitmap/RoaringBitmap/blob/master/src/main/java/org/roaringbitmap/BitmapContainer.java#L875-L892
func (bc *bitmapContainer) numberOfRuns() int {
	if bc.cardinality == 0 {
		return 0
	}

	var numRuns uint64
	nextWord := bc.bitmap[0]

	for i := 0; i < len(bc.bitmap)-1; i++ {
		word := nextWord
		nextWord = bc.bitmap[i+1]
		numRuns += popcount((^word)&(word<<1)) + ((word >> 63) &^ nextWord)
	}

	word := nextWord
	numRuns += popcount((^word) & (word << 1))
	if (word & 0x8000000000000000) != 0 {
		numRuns++
	}

	return int(numRuns)
}

// convert to run or array *if needed*
func (bc *bitmapContainer) toEfficientContainer() container {
	numRuns := bc.numberOfRuns()

	sizeAsRunContainer := runContainer16SerializedSizeInBytes(numRuns)
	sizeAsBitmapContainer := bitmapContainerSizeInBytes()
	card := bc.getCardinality()
	sizeAsArrayContainer := arrayContainerSizeInBytes(card)

	if sizeAsRunContainer < minOfInt(sizeAsBitmapContainer, sizeAsArrayContainer) {
		return newRunContainer16FromBitmapContainer(bc)
	}
	if card <= arrayDefaultMaxSize {
		return bc.toArrayContainer()
	}
	return bc
}

func newBitmapContainerFromRun(rc *runContainer16) *bitmapContainer {
	if len(rc.iv) == 1 {
		return newBitmapContainerwithRange(int(rc.iv[0].start), int(rc.iv[0].last()))
	}

	bc := newBitmapContainer()
	for i := range rc.iv {
		setBitmapRange(bc.bitmap, int(rc.iv[i].start), int(rc.iv[i].last())+1)
		bc.cardinality += int(rc.iv[i].last()) + 1 - int(rc.iv[i].start)
	}
	// bc.computeCardinality()
	return bc
}

func (bc *bitmapContainer) containerType() contype {
	return bitmapContype
}

func (bc *bitmapContainer) addOffset(x uint16) (container, container) {
	var low, high *bitmapContainer

	if bc.cardinality == 0 {
		return nil, nil
	}

	b := uint32(x) >> 6
	i := uint32(x) % 64
	end := uint32(1024) - b

	low = newBitmapContainer()
	if i == 0 {
		copy(low.bitmap[b:], bc.bitmap[:end])
	} else {
		low.bitmap[b] = bc.bitmap[0] << i
		for k := uint32(1); k < end; k++ {
			newval := bc.bitmap[k] << i
			newval |= bc.bitmap[k-1] >> (64 - i)
			low.bitmap[b+k] = newval
		}
	}
	low.computeCardinality()

	if low.cardinality == bc.cardinality {
		// All elements from bc ended up in low, meaning high will be empty.
		return low, nil
	}

	if low.cardinality == 0 {
		// low is empty, let's reuse the container for high.
		high = low
		low = nil
	} else {
		// None of the containers will be empty, so allocate both.
		high = newBitmapContainer()
	}

	if i == 0 {
		copy(high.bitmap[:b], bc.bitmap[end:])
	} else {
		for k := end; k < 1024; k++ {
			newval := bc.bitmap[k] << i
			newval |= bc.bitmap[k-1] >> (64 - i)
			high.bitmap[k-end] = newval
		}
		high.bitmap[b] = bc.bitmap[1023] >> (64 - i)
	}
	high.computeCardinality()

	// Ensure proper nil interface.
	if low == nil {
		return nil, high
	}

	return low, high
}

// nextValue returns either the `target` if found or the next largest value.
// if the target is out of bounds a -1 is returned
//
// Example :
// Suppose the bitmap container represents the following slice
// [1,2,10,11,100]
// target=0 returns 1
// target=1 returns 1
// target=10 returns 10
// target=90 returns 100
func (bc *bitmapContainer) nextValue(target uint16) int {
	if bc.cardinality == 0 {
		return -1
	}

	return bc.NextSetBit(uint(target))
}

// nextAbsentValue returns the next absent value.
// if the target is out of bounds a -1 is returned
func (bc *bitmapContainer) nextAbsentValue(target uint16) int {
	if bc.cardinality == 0 {
		return -1
	}

	var (
		x      = target >> 6
		length = uint(len(bc.bitmap))
	)
	if uint(x) >= length {
		return -1
	}
	w := bc.bitmap[x]
	w = w >> uint(target%64)
	if w == 0 {
		return int(target)
	}

	// Check if all 1's
	// if statement - we skip the if we have all ones [1,1,1,1...1]
	if ^w != 0 {

		if countTrailingZeros(w) > 0 {
			// we have something like [X,Y,Z, 0,0,0]. This means the target bit is zero
			return int(target)
		}

		// other wise something like [X,Y,0,1,1,1..1], where x and y can be either 1 or 0.

		trailing := countTrailingOnes(w)
		return int(target) + trailing

	}
	x++
	for ; uint(x) < length; x++ {
		if bc.bitmap[x] == 0 {
			return int(x * 64)
		}
		if ^bc.bitmap[x] != 0 {
			trailing := countTrailingOnes(bc.bitmap[x])
			return int(x*64) + trailing
		}

	}
	return -1
}

// previousValue returns either the `target` if found or the previous largest value.
// if the target is out of bounds a -1 is returned

// Example :
// Suppose the bitmap container represents the following slice
// [1,2,10,11,100]
// target=0 returns -1
// target=1 returns -1
// target=2 returns -1
// target=10 returns 9
// target=50 returns 10
// target=100 returns 99
func (bc *bitmapContainer) previousValue(target uint16) int {
	if bc.cardinality == 0 {
		return -1
	}
	return bc.uPrevSetBit(uint(target))
}

// previousAbsentValue returns the next absent value.
func (bc *bitmapContainer) previousAbsentValue(target uint16) int {
	if bc.cardinality == 0 {
		return -1
	}

	var (
		x      = target >> 6
		length = uint(len(bc.bitmap))
	)
	if uint(x) >= length {
		return -1
	}
	w := bc.bitmap[x]
	shifted := w >> uint(target%64)
	if shifted == 0 {
		return int(target)
	}

	// Check if all 1's
	// if statement - we skip if we have all ones [1,1,1,1...1] as no value is absent
	if ^shifted != 0 {

		if countTrailingZeros(shifted) > 0 {
			// we have something like shifted=[X,Y,Z,..., 0,0,0]. This means the target bit is zero
			return int(target)
		}

		// The rotate will rotate the target bit into the leading position.
		// We know the target bit is not zero because of the countTrailingZero check above
		// We then shift the target bit out of the way.
		// Assume a structure like an original structure like [X,Y,Z,..., Target, A, B,C...]
		// shifted will be [X,Y,Z...Target]
		// shiftedRotated will be [A,B,C....]
		// If countLeadingZeros > 0 then A is zero, if not at least A is 1 return
		// Else count the number of ones's until a 0
		shiftedRotated := bits.RotateLeft64(w, int(64-uint(target%64))-1) << 1
		leadingZeros := countLeadingZeros(shiftedRotated)
		if leadingZeros > 0 {
			return int(target) - 1
		}
		leadingOnes := countLeadingOnes(shiftedRotated)
		if leadingOnes > 0 {
			return int(target) - leadingOnes - 1
		}

	}
	x++
	for ; uint(x) < length; x++ {
		if bc.bitmap[x] == 0 {
			return int(x * 64)
		}
		if ^bc.bitmap[x] != 0 {
			trailing := countTrailingOnes(bc.bitmap[x])
			return int(x*64) + trailing
		}

	}
	return -1
}

// validate checks that the container size is non-negative
func (bc *bitmapContainer) validate() error {
	if bc.cardinality < arrayDefaultMaxSize {
		return fmt.Errorf("bitmap container size was less than: %d", arrayDefaultMaxSize)
	}

	if maxCapacity < len(bc.bitmap)*64 {
		return fmt.Errorf("bitmap slize size %d exceeded max capacity %d", maxCapacity, len(bc.bitmap)*64)
	}

	if bc.cardinality > maxCapacity {
		return fmt.Errorf("bitmap container size was greater than: %d", maxCapacity)
	}

	if bc.cardinality != int(popcntSlice(bc.bitmap)) {
		return fmt.Errorf("bitmap container size %d did not match underlying slice length: %d", bc.cardinality, int(popcntSlice(bc.bitmap)))
	}

	return nil
}
