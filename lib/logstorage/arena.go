package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type arena struct {
	b []byte
}

func (a *arena) reset() {
	a.b = a.b[:0]
}

func (a *arena) copyBytes(b []byte) []byte {
	ab := a.b
	abLen := len(ab)
	ab = append(ab, b...)
	result := ab[abLen:]
	a.b = ab
	return result
}

func (a *arena) newBytes(size int) []byte {
	ab := a.b
	abLen := len(ab)
	ab = bytesutil.ResizeWithCopyMayOverallocate(ab, abLen+size)
	result := ab[abLen:]
	a.b = ab
	return result
}
