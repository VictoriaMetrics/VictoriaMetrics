package logstorage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

func getArena() *arena {
	v := arenaPool.Get()
	if v == nil {
		return &arena{}
	}
	return v.(*arena)
}

func putArena(a *arena) {
	a.reset()
	arenaPool.Put(a)
}

var arenaPool sync.Pool

type arena struct {
	b []byte
}

func (a *arena) reset() {
	a.b = a.b[:0]
}

func (a *arena) preallocate(n int) {
	a.b = slicesutil.ExtendCapacity(a.b, n)
}

func (a *arena) sizeBytes() int {
	return cap(a.b)
}

func (a *arena) copyBytes(b []byte) []byte {
	if len(b) == 0 {
		return b
	}

	ab := a.b
	abLen := len(ab)
	ab = append(ab, b...)
	a.b = ab
	return ab[abLen:]
}

func (a *arena) copyBytesToString(b []byte) string {
	bCopy := a.copyBytes(b)
	return bytesutil.ToUnsafeString(bCopy)
}

func (a *arena) copyString(s string) string {
	b := bytesutil.ToUnsafeBytes(s)
	return a.copyBytesToString(b)
}

func (a *arena) newBytes(size int) []byte {
	if size <= 0 {
		return nil
	}

	ab := a.b
	abLen := len(ab)
	ab = bytesutil.ResizeWithCopyMayOverallocate(ab, abLen+size)
	result := ab[abLen:]
	a.b = ab
	return result
}
