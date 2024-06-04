package logstorage

import (
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// Field is a single field for the log entry.
type Field struct {
	// Name is the name of the field
	Name string

	// Value is the value of the field
	Value string
}

// Reset resets f for future re-use.
func (f *Field) Reset() {
	f.Name = ""
	f.Value = ""
}

// String returns string representation of f.
func (f *Field) String() string {
	x := f.marshalToJSON(nil)
	return string(x)
}

func (f *Field) marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.Name))
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.Value))
	return dst
}

func (f *Field) unmarshal(a *arena, src []byte) ([]byte, error) {
	srcOrig := src

	// Unmarshal field name
	b, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return srcOrig, fmt.Errorf("cannot unmarshal field name")
	}
	src = src[nSize:]
	f.Name = a.copyBytesToString(b)

	// Unmarshal field value
	b, nSize = encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return srcOrig, fmt.Errorf("cannot unmarshal field value")
	}
	src = src[nSize:]
	f.Value = a.copyBytesToString(b)

	return src, nil
}

func (f *Field) marshalToJSON(dst []byte) []byte {
	dst = strconv.AppendQuote(dst, f.Name)
	dst = append(dst, ':')
	dst = strconv.AppendQuote(dst, f.Value)
	return dst
}

// MarshalFieldsToJSON appends JSON-marshaled fields to dt and returns the result.
func MarshalFieldsToJSON(dst []byte, fields []Field) []byte {
	dst = append(dst, '{')
	if len(fields) > 0 {
		dst = fields[0].marshalToJSON(dst)
		fields = fields[1:]
		for i := range fields {
			dst = append(dst, ',')
			dst = fields[i].marshalToJSON(dst)
		}
	}
	dst = append(dst, '}')
	return dst
}

func appendFields(a *arena, dst, src []Field) []Field {
	for _, f := range src {
		dst = append(dst, Field{
			Name:  a.copyString(f.Name),
			Value: a.copyString(f.Value),
		})
	}
	return dst
}

// rows is an aux structure used during rows merge
type rows struct {
	fieldsBuf []Field

	timestamps []int64

	rows [][]Field
}

// reset resets rs
func (rs *rows) reset() {
	fb := rs.fieldsBuf
	for i := range fb {
		fb[i].Reset()
	}
	rs.fieldsBuf = fb[:0]

	rs.timestamps = rs.timestamps[:0]

	rows := rs.rows
	for i := range rows {
		rows[i] = nil
	}
	rs.rows = rows[:0]
}

// appendRows appends rows with the given timestamps to rs.
func (rs *rows) appendRows(timestamps []int64, rows [][]Field) {
	rs.timestamps = append(rs.timestamps, timestamps...)

	fieldsBuf := rs.fieldsBuf
	for _, fields := range rows {
		fieldsLen := len(fieldsBuf)
		fieldsBuf = append(fieldsBuf, fields...)
		rs.rows = append(rs.rows, fieldsBuf[fieldsLen:])
	}
	rs.fieldsBuf = fieldsBuf
}

// mergeRows merges the args and appends them to rs.
func (rs *rows) mergeRows(timestampsA, timestampsB []int64, fieldsA, fieldsB [][]Field) {
	for len(timestampsA) > 0 && len(timestampsB) > 0 {
		i := 0
		minTimestamp := timestampsB[0]
		for i < len(timestampsA) && timestampsA[i] <= minTimestamp {
			i++
		}
		rs.appendRows(timestampsA[:i], fieldsA[:i])
		fieldsA = fieldsA[i:]
		timestampsA = timestampsA[i:]

		fieldsA, fieldsB = fieldsB, fieldsA
		timestampsA, timestampsB = timestampsB, timestampsA
	}
	if len(timestampsA) == 0 {
		rs.appendRows(timestampsB, fieldsB)
	} else {
		rs.appendRows(timestampsA, fieldsA)
	}
}
