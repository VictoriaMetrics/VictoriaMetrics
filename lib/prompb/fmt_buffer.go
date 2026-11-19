package prompb

import (
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type fmtBuffer struct {
	buf []byte
}

func (fb *fmtBuffer) reset() {
	fb.buf = fb.buf[:0]
}

func (fb *fmtBuffer) formatName(prefix, suffix string) string {
	if prefix == "" {
		// There is no prefix, so just return the suffix as is.
		return suffix
	}

	n := len(fb.buf)
	fb.buf = append(fb.buf, prefix...)
	fb.buf = append(fb.buf, suffix...)

	return bytesutil.ToUnsafeString(fb.buf[n:])
}

func (fb *fmtBuffer) formatVmrange(start, end float64) string {
	n := len(fb.buf)
	fb.buf = strconv.AppendFloat(fb.buf, start, 'e', 3, 64)
	fb.buf = append(fb.buf, "..."...)
	fb.buf = strconv.AppendFloat(fb.buf, end, 'e', 3, 64)
	return bytesutil.ToUnsafeString(fb.buf[n:])
}
