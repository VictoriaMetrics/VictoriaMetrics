package pb

import (
	"encoding/base64"
	"strconv"

	"github.com/valyala/fastjson"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type fmtBuffer struct {
	buf []byte
}

func (fb *fmtBuffer) reset() {
	fb.buf = fb.buf[:0]
}

func (fb *fmtBuffer) formatInt(v int64) string {
	n := len(fb.buf)
	fb.buf = strconv.AppendInt(fb.buf, v, 10)
	return bytesutil.ToUnsafeString(fb.buf[n:])
}

func (fb *fmtBuffer) formatFloat(v float64) string {
	n := len(fb.buf)
	fb.buf = strconv.AppendFloat(fb.buf, v, 'f', -1, 64)
	return bytesutil.ToUnsafeString(fb.buf[n:])
}

func (fb *fmtBuffer) formatSubFieldName(prefix, suffix string) string {
	if prefix == "" {
		// There is no prefix, so just return the suffix as is.
		return suffix
	}

	n := len(fb.buf)
	fb.buf = append(fb.buf, prefix...)
	fb.buf = append(fb.buf, '.')
	fb.buf = append(fb.buf, suffix...)

	return bytesutil.ToUnsafeString(fb.buf[n:])
}

func (fb *fmtBuffer) formatBase64(src []byte) string {
	n := len(fb.buf)
	fb.buf = base64.StdEncoding.AppendEncode(fb.buf, src)
	return bytesutil.ToUnsafeString(fb.buf[n:])
}

func (fb *fmtBuffer) encodeJSONValue(v *fastjson.Value) string {
	n := len(fb.buf)
	fb.buf = v.MarshalTo(fb.buf)
	return bytesutil.ToUnsafeString(fb.buf[n:])
}

func (fb *fmtBuffer) formatVmrange(start, end float64) string {
	n := len(fb.buf)
	fb.buf = strconv.AppendFloat(fb.buf, start, 'e', 3, 64)
	fb.buf = append(fb.buf, "..."...)
	fb.buf = strconv.AppendFloat(fb.buf, end, 'e', 3, 64)
	return bytesutil.ToUnsafeString(fb.buf[n:])
}
