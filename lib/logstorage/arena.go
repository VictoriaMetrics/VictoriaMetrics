package logstorage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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

func (a *arena) sizeBytes() int {
	return len(a.b)
}

func (a *arena) copyBytes(b []byte) []byte {
	if len(b) == 0 {
		return b
	}

	ab := a.b
	abLen := len(ab)
	ab = append(ab, b...)
	result := ab[abLen:]
	a.b = ab
	return result
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
