package roaring

type shortIterable interface {
	hasNext() bool
	next() uint16
}

type shortPeekable interface {
	shortIterable
	peekNext() uint16
	advanceIfNeeded(minval uint16)
}

type shortIterator struct {
	slice []uint16
	loc   int
}

func (si *shortIterator) hasNext() bool {
	return si.loc < len(si.slice)
}

func (si *shortIterator) next() uint16 {
	a := si.slice[si.loc]
	si.loc++
	return a
}

func (si *shortIterator) peekNext() uint16 {
	return si.slice[si.loc]
}

func (si *shortIterator) advanceIfNeeded(minval uint16) {
	if si.hasNext() && si.peekNext() < minval {
		si.loc = advanceUntil(si.slice, si.loc, len(si.slice), minval)
	}
}

type reverseIterator struct {
	slice []uint16
	loc   int
}

func (si *reverseIterator) hasNext() bool {
	return si.loc >= 0
}

func (si *reverseIterator) next() uint16 {
	a := si.slice[si.loc]
	si.loc--
	return a
}

type arrayContainerUnsetIterator struct {
	content []uint16
	// pos is the index of the next set bit that is >= nextVal.
	// When nextVal reaches content[pos], pos is incremented.
	pos     int
	nextVal int
}

func (acui *arrayContainerUnsetIterator) next() uint16 {
	val := acui.nextVal
	acui.nextVal++
	for acui.pos < len(acui.content) && uint16(acui.nextVal) >= acui.content[acui.pos] {
		acui.nextVal++
		acui.pos++
	}
	return uint16(val)
}

func (acui *arrayContainerUnsetIterator) hasNext() bool {
	return acui.nextVal < 65536
}

func (acui *arrayContainerUnsetIterator) peekNext() uint16 {
	return uint16(acui.nextVal)
}

func (acui *arrayContainerUnsetIterator) advanceIfNeeded(minval uint16) {
	if !acui.hasNext() || acui.peekNext() >= minval {
		return
	}
	acui.nextVal = int(minval)
	acui.pos = binarySearch(acui.content, minval)
	if acui.pos < 0 {
		acui.pos = -acui.pos - 1
	}
	for acui.pos < len(acui.content) && uint16(acui.nextVal) >= acui.content[acui.pos] {
		acui.nextVal++
		acui.pos++
	}
}

func newArrayContainerUnsetIterator(content []uint16) *arrayContainerUnsetIterator {
	acui := &arrayContainerUnsetIterator{content: content, pos: 0, nextVal: 0}
	for acui.pos < len(acui.content) && uint16(acui.nextVal) >= acui.content[acui.pos] {
		acui.nextVal++
		acui.pos++
	}
	return acui
}
