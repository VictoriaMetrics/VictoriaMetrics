package logstorage

import (
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// LogRows holds a set of rows needed for Storage.MustAddRows
//
// LogRows must be obtained via GetLogRows()
type LogRows struct {
	// buf holds all the bytes referred by items in LogRows
	buf []byte

	// fieldsBuf holds all the fields referred by items in LogRows
	fieldsBuf []Field

	// streamIDs holds streamIDs for rows added to LogRows
	streamIDs []streamID

	// streamTagsCanonicals holds streamTagsCanonical entries for rows added to LogRows
	streamTagsCanonicals [][]byte

	// timestamps holds stimestamps for rows added to LogRows
	timestamps []int64

	// rows holds fields for rows added to LogRows.
	rows [][]Field

	// sf is a helper for sorting fields in every added row
	sf sortedFields

	// streamFields contains names for stream fields
	streamFields map[string]struct{}

	// ignoreFields contains names for log fields, which must be skipped during data ingestion
	ignoreFields map[string]struct{}
}

type sortedFields []Field

func (sf *sortedFields) Len() int {
	return len(*sf)
}

func (sf *sortedFields) Less(i, j int) bool {
	a := *sf
	return a[i].Name < a[j].Name
}

func (sf *sortedFields) Swap(i, j int) {
	a := *sf
	a[i], a[j] = a[j], a[i]
}

// RowFormatter implementes fmt.Stringer for []Field aka a single log row
type RowFormatter []Field

// String returns user-readable representation for rf
func (rf *RowFormatter) String() string {
	result := MarshalFieldsToJSON(nil, *rf)
	return string(result)
}

// Reset resets lr with all its settings.
//
// Call ResetKeepSettings() for resetting lr without resetting its settings.
func (lr *LogRows) Reset() {
	lr.ResetKeepSettings()

	sfs := lr.streamFields
	for k := range sfs {
		delete(sfs, k)
	}

	ifs := lr.ignoreFields
	for k := range ifs {
		delete(ifs, k)
	}
}

// ResetKeepSettings resets rows stored in lr, while keeping its settings passed to GetLogRows().
func (lr *LogRows) ResetKeepSettings() {
	lr.buf = lr.buf[:0]

	fb := lr.fieldsBuf
	for i := range fb {
		fb[i].Reset()
	}
	lr.fieldsBuf = fb[:0]

	sids := lr.streamIDs
	for i := range sids {
		sids[i].reset()
	}
	lr.streamIDs = sids[:0]

	sns := lr.streamTagsCanonicals
	for i := range sns {
		sns[i] = nil
	}
	lr.streamTagsCanonicals = sns[:0]

	lr.timestamps = lr.timestamps[:0]

	rows := lr.rows
	for i := range rows {
		rows[i] = nil
	}
	lr.rows = rows[:0]

	lr.sf = nil
}

// NeedFlush returns true if lr contains too much data, so it must be flushed to the storage.
func (lr *LogRows) NeedFlush() bool {
	return len(lr.buf) > (maxUncompressedBlockSize/8)*7
}

// MustAdd adds a log entry with the given args to lr.
//
// It is OK to modify the args after returning from the function,
// since lr copies all the args to internal data.
//
// field names longer than MaxFieldNameSize are automatically truncated to MaxFieldNameSize length.
func (lr *LogRows) MustAdd(tenantID TenantID, timestamp int64, fields []Field) {
	// Compose StreamTags from fields according to lr.streamFields
	sfs := lr.streamFields
	st := GetStreamTags()
	for i := range fields {
		f := &fields[i]
		if _, ok := sfs[f.Name]; ok {
			st.Add(f.Name, f.Value)
		}
	}

	// Marshal StreamTags
	bb := bbPool.Get()
	bb.B = st.MarshalCanonical(bb.B)
	PutStreamTags(st)

	// Calculate the id for the StreamTags
	var sid streamID
	sid.tenantID = tenantID
	sid.id = hash128(bb.B)

	// Store the row
	lr.mustAddInternal(sid, timestamp, fields, bb.B)
	bbPool.Put(bb)
}

func (lr *LogRows) mustAddInternal(sid streamID, timestamp int64, fields []Field, streamTagsCanonical []byte) {
	buf := lr.buf
	bufLen := len(buf)
	buf = append(buf, streamTagsCanonical...)

	lr.streamTagsCanonicals = append(lr.streamTagsCanonicals, buf[bufLen:])
	lr.streamIDs = append(lr.streamIDs, sid)
	lr.timestamps = append(lr.timestamps, timestamp)

	// Store all the fields
	ifs := lr.ignoreFields
	fb := lr.fieldsBuf
	fieldsLen := len(fb)
	for i := range fields {
		f := &fields[i]

		if _, ok := ifs[f.Name]; ok {
			// Skip fields from the ifs map
			continue
		}
		if f.Value == "" {
			// Skip fields without values
			continue
		}

		fb = append(fb, Field{})
		dstField := &fb[len(fb)-1]

		bufLen = len(buf)
		fieldName := f.Name
		if len(fieldName) > MaxFieldNameSize {
			fieldName = fieldName[:MaxFieldNameSize]
		}
		if fieldName != "_msg" {
			buf = append(buf, fieldName...)
		}
		dstField.Name = bytesutil.ToUnsafeString(buf[bufLen:])

		bufLen = len(buf)
		buf = append(buf, f.Value...)
		dstField.Value = bytesutil.ToUnsafeString(buf[bufLen:])
	}
	lr.sf = fb[fieldsLen:]
	sort.Sort(&lr.sf)
	lr.rows = append(lr.rows, lr.sf)

	lr.fieldsBuf = fb
	lr.buf = buf
}

// GetRowString returns string representation of the row with the given idx.
func (lr *LogRows) GetRowString(idx int) string {
	tf := TimeFormatter(lr.timestamps[idx])
	streamTags := getStreamTagsString(lr.streamTagsCanonicals[idx])
	var rf RowFormatter
	rf = append(rf[:0], lr.rows[idx]...)
	rf = append(rf, Field{
		Name:  "_time",
		Value: tf.String(),
	})
	rf = append(rf, Field{
		Name:  "_stream",
		Value: streamTags,
	})
	sort.Slice(rf, func(i, j int) bool {
		return rf[i].Name < rf[j].Name
	})
	return rf.String()
}

// GetLogRows returns LogRows from the pool for the given streamFields.
//
// streamFields is a set of field names, which must be associated with the stream.
// ignoreFields is a set of field names, which must be ignored during data ingestion.
//
// Return back it to the pool with PutLogRows() when it is no longer needed.
func GetLogRows(streamFields, ignoreFields []string) *LogRows {
	v := logRowsPool.Get()
	if v == nil {
		v = &LogRows{}
	}
	lr := v.(*LogRows)

	// Initialize streamFields
	sfs := lr.streamFields
	if sfs == nil {
		sfs = make(map[string]struct{}, len(streamFields))
		lr.streamFields = sfs
	}
	for _, f := range streamFields {
		sfs[f] = struct{}{}
	}

	// Initialize ignoreFields
	ifs := lr.ignoreFields
	if ifs == nil {
		ifs = make(map[string]struct{}, len(ignoreFields))
		lr.ignoreFields = ifs
	}
	for _, f := range ignoreFields {
		if f != "" {
			ifs[f] = struct{}{}
		}
	}

	return lr
}

// PutLogRows returns lr to the pool.
func PutLogRows(lr *LogRows) {
	lr.Reset()
	logRowsPool.Put(lr)
}

var logRowsPool sync.Pool

// Len returns the number of items in lr.
func (lr *LogRows) Len() int {
	return len(lr.streamIDs)
}

// Less returns true if (streamID, timestamp) for row i is smaller than the (streamID, timestamp) for row j
func (lr *LogRows) Less(i, j int) bool {
	a := &lr.streamIDs[i]
	b := &lr.streamIDs[j]
	if !a.equal(b) {
		return a.less(b)
	}
	return lr.timestamps[i] < lr.timestamps[j]
}

// Swap swaps rows i and j in lr.
func (lr *LogRows) Swap(i, j int) {
	a := &lr.streamIDs[i]
	b := &lr.streamIDs[j]
	*a, *b = *b, *a

	tsA, tsB := &lr.timestamps[i], &lr.timestamps[j]
	*tsA, *tsB = *tsB, *tsA

	snA, snB := &lr.streamTagsCanonicals[i], &lr.streamTagsCanonicals[j]
	*snA, *snB = *snB, *snA

	fieldsA, fieldsB := &lr.rows[i], &lr.rows[j]
	*fieldsA, *fieldsB = *fieldsB, *fieldsA
}
