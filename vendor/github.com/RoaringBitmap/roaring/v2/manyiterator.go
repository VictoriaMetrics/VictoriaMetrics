package roaring

type manyIterable interface {
	nextMany(hs uint32, buf []uint32) int
	nextMany64(hs uint64, buf []uint64) int
}

func (si *shortIterator) nextMany(hs uint32, buf []uint32) int {
	n := 0
	l := si.loc
	s := si.slice
	for n < len(buf) && l < len(s) {
		buf[n] = uint32(s[l]) | hs
		l++
		n++
	}
	si.loc = l
	return n
}

func (si *shortIterator) nextMany64(hs uint64, buf []uint64) int {
	n := 0
	l := si.loc
	s := si.slice
	for n < len(buf) && l < len(s) {
		buf[n] = uint64(s[l]) | hs
		l++
		n++
	}
	si.loc = l
	return n
}
