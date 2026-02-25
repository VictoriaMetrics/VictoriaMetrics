package logstorage

import (
	"fmt"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/valyala/quicktemplate"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// Field is a single field for the log entry.
type Field struct {
	// Name is the name of the field
	Name string

	// Value is the value of the field
	Value string
}

// Reset resets f for future reuse.
func (f *Field) Reset() {
	f.Name = ""
	f.Value = ""
}

// String returns string representation of f.
func (f *Field) String() string {
	x := f.marshalToJSON(nil)
	return string(x)
}

func (f *Field) marshal(dst []byte, marshalFieldName bool) []byte {
	if marshalFieldName {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.Name))
	}
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.Value))
	return dst
}

// unmarshalInplace unmarshals f from src.
//
// f is valid until src is changed.
func (f *Field) unmarshalInplace(src []byte, unmarshalFieldName bool) ([]byte, error) {
	srcOrig := src

	// Unmarshal field name
	if unmarshalFieldName {
		name, nSize := encoding.UnmarshalBytes(src)
		if nSize <= 0 {
			return srcOrig, fmt.Errorf("cannot unmarshal field name")
		}
		src = src[nSize:]
		f.Name = bytesutil.ToUnsafeString(name)
	}

	// Unmarshal field value
	value, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return srcOrig, fmt.Errorf("cannot unmarshal field value")
	}
	src = src[nSize:]
	f.Value = bytesutil.ToUnsafeString(value)

	return src, nil
}

func (f *Field) marshalToJSON(dst []byte) []byte {
	name := f.Name
	if name == "" {
		name = "_msg"
	}
	dst = quicktemplate.AppendJSONString(dst, name, true)
	dst = append(dst, ':')
	dst = quicktemplate.AppendJSONString(dst, f.Value, true)
	return dst
}

func (f *Field) marshalToLogfmt(dst []byte) []byte {
	name := f.Name
	if name == "" {
		name = "_msg"
	}
	dst = append(dst, name...)
	dst = append(dst, '=')
	if needLogfmtQuoting(f.Value) {
		dst = quicktemplate.AppendJSONString(dst, f.Value, true)
	} else {
		dst = append(dst, f.Value...)
	}
	return dst
}

func getFieldValueByName(fields []Field, name string) string {
	for _, f := range fields {
		if f.Name == name {
			return f.Value
		}
	}
	return ""
}

func needLogfmtQuoting(s string) bool {
	for _, c := range s {
		if isLogfmtSpecialChar(c) {
			return true
		}
	}
	return false
}

func isLogfmtSpecialChar(c rune) bool {
	if c <= 0x20 {
		return true
	}
	switch c {
	case '"', '\\':
		return true
	default:
		return false
	}
}

// RenameField renames the first non-empty field with the name from oldNames list to newName in Fields
func RenameField(fields []Field, oldNames []string, newName string) {
	if len(oldNames) == 0 {
		// Nothing to rename
		return
	}
	for _, n := range oldNames {
		for j := range fields {
			f := &fields[j]
			if f.Name == n && f.Value != "" {
				f.Name = newName
				return
			}
		}
	}
}

// MarshalFieldsToJSON appends JSON-marshaled fields to dst and returns the result.
func MarshalFieldsToJSON(dst []byte, fields []Field) []byte {
	fields = SkipLeadingFieldsWithoutValues(fields)
	dst = append(dst, '{')
	if len(fields) > 0 {
		dst = fields[0].marshalToJSON(dst)
		fields = fields[1:]
		for i := range fields {
			f := &fields[i]
			if f.Value == "" {
				// Skip fields without values
				continue
			}
			dst = append(dst, ',')
			dst = f.marshalToJSON(dst)
		}
	}
	dst = append(dst, '}')
	return dst
}

// MarshalFieldsToLogfmt appends logfmt-marshaled fields to dst and returns the result.
func MarshalFieldsToLogfmt(dst []byte, fields []Field) []byte {
	if len(fields) == 0 {
		return dst
	}
	dst = fields[0].marshalToLogfmt(dst)
	fields = fields[1:]
	for i := range fields {
		dst = append(dst, ' ')
		dst = fields[i].marshalToLogfmt(dst)
	}
	return dst
}

// SkipLeadingFieldsWithoutValues skips leading fields without values.
func SkipLeadingFieldsWithoutValues(fields []Field) []Field {
	i := 0
	for i < len(fields) && fields[i].Value == "" {
		i++
	}
	return fields[i:]
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

	clear(rs.rows)
	rs.rows = rs.rows[:0]
}

func (rs *rows) hasNonEmptyRows() bool {
	rows := rs.rows
	for _, fields := range rows {
		if len(fields) > 0 {
			return true
		}
	}
	return false
}

// appendRows appends rows with the given timestamps to rs.
func (rs *rows) appendRows(timestamps []int64, rows [][]Field) {
	rs.timestamps = append(rs.timestamps, timestamps...)

	// Pre-allocate rs.fieldsBuf
	fieldsCount := 0
	for _, fields := range rows {
		fieldsCount += len(fields)
	}
	fieldsBuf := slicesutil.SetLength(rs.fieldsBuf, len(rs.fieldsBuf)+fieldsCount)
	fieldsBuf = fieldsBuf[:len(fieldsBuf)-fieldsCount]

	// Pre-allocate rs.rows
	rs.rows = slicesutil.SetLength(rs.rows, len(rs.rows)+len(rows))
	dstRows := rs.rows[len(rs.rows)-len(rows):]

	for i, fields := range rows {
		fieldsLen := len(fieldsBuf)
		fieldsBuf = append(fieldsBuf, fields...)
		dstRows[i] = fieldsBuf[fieldsLen:]
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

func (rs *rows) skipRowsByDropFilter(dropFilter *partitionSearchOptions, dropFilterFields *prefixfilter.Filter, offset int, stream, streamID string) {
	tmpFields := GetFields()
	defer PutFields(tmpFields)

	tmpFields.Fields = addFieldIfNeeded(tmpFields.Fields, dropFilterFields, "_stream", stream)
	tmpFields.Fields = addFieldIfNeeded(tmpFields.Fields, dropFilterFields, "_stream_id", streamID)
	tmpFieldsBaseLen := len(tmpFields.Fields)

	dstTimestamps := rs.timestamps[:offset]
	dstRows := rs.rows[:offset]

	srcTimestamps := rs.timestamps[offset:]
	srcRows := rs.rows[offset:]

	bb := bbPool.Get()
	for i := range srcTimestamps {
		srcTimestamp := srcTimestamps[i]
		srcFields := srcRows[i]

		if srcTimestamp < dropFilter.minTimestamp || srcTimestamp > dropFilter.maxTimestamp {
			// Fast path - keep row outsize the dropFilter time range
			dstTimestamps = append(dstTimestamps, srcTimestamp)
			dstRows = append(dstRows, srcFields)
			continue
		}

		if dropFilterFields.MatchString("_time") {
			bb.B = marshalTimestampISO8601String(bb.B[:0], srcTimestamp)
			tmpFields.Add("_time", bytesutil.ToUnsafeString(bb.B))
		}

		for _, f := range srcFields {
			tmpFields.Fields = addFieldIfNeeded(tmpFields.Fields, dropFilterFields, f.Name, f.Value)
		}

		if !dropFilter.filter.matchRow(tmpFields.Fields) {
			dstTimestamps = append(dstTimestamps, srcTimestamp)
			dstRows = append(dstRows, srcFields)
		} else if i == 0 {
			// The first row with the minimum timestamp is deleted.
			// Replace it with an empty row with the original timestamp in order to keep valid the assumptions
			// that blocks for the same log stream are sorted by their first (minimum) timestamps.
			// Violating these assumptions leads to data loss during background merge
			// when obtaining the next block to merge via blockStreamReadersHeap.Less.
			//
			// It is safe to use an empty row here, since it is treated as non-existing row
			// during filtering because of VictoraLogs data model - https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model
			//
			// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/825
			dstTimestamps = append(dstTimestamps, srcTimestamp)
			dstRows = append(dstRows, nil)
		}

		clear(tmpFields.Fields[tmpFieldsBaseLen:])
		tmpFields.Fields = tmpFields.Fields[:tmpFieldsBaseLen]
	}
	bbPool.Put(bb)

	rs.timestamps = dstTimestamps

	clear(rs.rows[len(dstRows):])
	rs.rows = dstRows
}

func addFieldIfNeeded(dst []Field, pf *prefixfilter.Filter, name, value string) []Field {
	name = getCanonicalColumnName(name)
	if pf.MatchString(name) {
		dst = append(dst, Field{
			Name:  name,
			Value: value,
		})
	}
	return dst
}

func sortFieldsByName(fields []Field) {
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})
}

// Fields holds a slice of Field items
type Fields struct {
	// Fields is a slice fields
	Fields []Field
}

// Reset resets f.
func (f *Fields) Reset() {
	clear(f.Fields)
	f.Fields = f.Fields[:0]
}

// ClearUpToCapacity clears f.Fields up to its' capacity.
//
// This function is useful in order to make sure f.Fields do not reference underlying byte slices,
// so they could be freed by Go GC.
func (f *Fields) ClearUpToCapacity() {
	clear(f.Fields[:cap(f.Fields)])
	f.Fields = f.Fields[:0]
}

// Add adds (name, value) field to f.
func (f *Fields) Add(name, value string) {
	f.Fields = append(f.Fields, Field{
		Name:  name,
		Value: value,
	})
}

// GetFields returns an empty Fields from the pool.
//
// Pass the returned Fields to PutFields() when it is no longer needed.
func GetFields() *Fields {
	v := fieldsPool.Get()
	if v == nil {
		return &Fields{}
	}
	return v.(*Fields)
}

// PutFields returns f to the pool.
//
// f cannot be used after returning to the pool. Use GetFields() for obtaining an empty Fields from the pool.
func PutFields(f *Fields) {
	f.Reset()
	fieldsPool.Put(f)
}

var fieldsPool sync.Pool
