package promql

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

type timeseries struct {
	MetricName storage.MetricName
	Values     []float64
	Timestamps []int64

	// Whether the timeseries may be re-used.
	// Timeseries may be re-used only if their members own values
	// they refer to.
	denyReuse bool
}

func (ts *timeseries) Reset() {
	if ts.denyReuse {
		*ts = timeseries{}
		return
	}

	ts.MetricName.Reset()
	ts.Values = ts.Values[:0]
	ts.Timestamps = ts.Timestamps[:0]
}

func (ts *timeseries) String() string {
	return fmt.Sprintf("MetricName=%s, Values=%g, Timestamps=%d", &ts.MetricName, ts.Values, ts.Timestamps)
}

func (ts *timeseries) CopyFromShallowTimestamps(src *timeseries) {
	ts.Reset()
	ts.MetricName.CopyFrom(&src.MetricName)
	ts.Values = append(ts.Values[:0], src.Values...)
	ts.Timestamps = src.Timestamps

	ts.denyReuse = true
}

func (ts *timeseries) CopyFromMetricNames(src *timeseries) {
	ts.Reset()
	ts.MetricName.CopyFrom(&src.MetricName)
	ts.Values = src.Values
	ts.Timestamps = src.Timestamps

	ts.denyReuse = true
}

func (ts *timeseries) CopyShallow(src *timeseries) {
	*ts = *src
	ts.denyReuse = true
}

func getTimeseries() *timeseries {
	if v := timeseriesPool.Get(); v != nil {
		return v.(*timeseries)
	}
	return &timeseries{}
}

func putTimeseries(ts *timeseries) {
	ts.Reset()
	timeseriesPool.Put(ts)
}

var timeseriesPool sync.Pool

func marshalTimeseriesFast(dst []byte, tss []*timeseries, maxSize int, step int64) []byte {
	if len(tss) == 0 {
		// marshal zero timeseries and zero timestamps
		dst = encoding.MarshalUint64(dst, 0)
		dst = encoding.MarshalUint64(dst, 0)
		return dst
	}

	// timestamps are stored only once for all the tss, since they must be identical
	assertIdenticalTimestamps(tss, step)
	timestamps := tss[0].Timestamps

	// Calculate the required size for marshaled tss.
	size := 8 + 8                          // 8 bytes for len(tss) and 8 bytes for len(timestamps)
	size += 8 * len(timestamps)            // encoded timestamps
	size += 8 * len(tss) * len(timestamps) // encoded values
	for _, ts := range tss {
		size += marshaledFastMetricNameSize(&ts.MetricName)
	}
	if size > maxSize {
		// Do not marshal tss, since it would occupy too much space
		return dst
	}

	// Allocate the buffer for the marshaled tss before its' marshaling.
	// This should reduce memory fragmentation and memory usage.
	dstLen := len(dst)
	dst = bytesutil.ResizeWithCopyMayOverallocate(dst, size+dstLen)
	dst = dst[:dstLen]

	// Marshal timestamps and values at first, so they are 8-byte aligned.
	// This prevents from SIGBUS error on arm architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3927
	dst = encoding.MarshalUint64(dst, uint64(len(tss)))
	dst = encoding.MarshalUint64(dst, uint64(len(timestamps)))
	dst = marshalTimestampsFast(dst, timestamps)
	for _, ts := range tss {
		dst = marshalValuesFast(dst, ts.Values)
	}
	for _, ts := range tss {
		dst = marshalMetricNameFast(dst, &ts.MetricName)
	}
	return dst
}

// unmarshalTimeseriesFast unmarshals timeseries from src.
//
// The returned timeseries refer to src, so it is unsafe to modify it while timeseries are in use.
func unmarshalTimeseriesFast(src []byte) ([]*timeseries, error) {
	if len(src) < 16 {
		return nil, fmt.Errorf("cannot unmarshal timeseries from %d bytes; need at least 16 bytes", len(src))
	}
	tssLen := encoding.UnmarshalUint64(src)
	timestampsLen := encoding.UnmarshalUint64(src[8:])
	src = src[16:]

	// Unmarshal timestamps
	tail, timestamps, err := unmarshalTimestampsFast(src, timestampsLen)
	if err != nil {
		return nil, err
	}
	src = tail

	tss := make([]*timeseries, tssLen)
	for i := range tss {
		var ts timeseries
		ts.denyReuse = true
		ts.Timestamps = timestamps
		tss[i] = &ts
	}

	// Unmarshal values
	for _, ts := range tss {
		tail, values, err := unmarshalValuesFast(src, timestampsLen)
		if err != nil {
			return nil, err
		}
		ts.Values = values
		src = tail
	}

	// Unmarshal metric names for the time series
	for _, ts := range tss {
		tail, err := unmarshalMetricNameFast(&ts.MetricName, src)
		if err != nil {
			return nil, err
		}
		src = tail
	}

	if len(src) > 0 {
		return nil, fmt.Errorf("unexpected non-empty tail left after unmarshaling %d timeseries; len(tail)=%d", len(tss), len(src))
	}
	return tss, nil
}

// marshaledFastMetricNameSize returns the size of marshaled mn returned from marshalMetricNameFast.
func marshaledFastMetricNameSize(mn *storage.MetricName) int {
	n := 0
	n += 2 + len(mn.MetricGroup)
	n += 2 // Length of tags.
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		n += 2 + len(tag.Key)
		n += 2 + len(tag.Value)
	}
	return n
}

func marshalValuesFast(dst []byte, values []float64) []byte {
	// Do not marshal len(values), since it is already encoded as len(timestamps) at marshalTimestampsFast.
	valuesBuf := float64ToByteSlice(values)
	dst = append(dst, valuesBuf...)
	return dst
}

// it is unsafe modifying src while the returned values is in use.
func unmarshalValuesFast(src []byte, valuesLen uint64) ([]byte, []float64, error) {
	bufSize := valuesLen * 8
	if uint64(len(src)) < bufSize {
		return src, nil, fmt.Errorf("cannot unmarshal values; got %d ytes; want at least %d bytes", uint64(len(src)), bufSize)
	}
	values := byteSliceToFloat64(src[:bufSize])
	return src[bufSize:], values, nil
}

func marshalTimestampsFast(dst []byte, timestamps []int64) []byte {
	timestampsBuf := int64ToByteSlice(timestamps)
	dst = append(dst, timestampsBuf...)
	return dst
}

// it is unsafe modifying src while the returned timestamps is in use.
func unmarshalTimestampsFast(src []byte, timestampsLen uint64) ([]byte, []int64, error) {
	bufSize := timestampsLen * 8
	if uint64(len(src)) < bufSize {
		return src, nil, fmt.Errorf("cannot unmarshal timestamps; got %d bytes; want at least %d bytes", len(src), bufSize)
	}
	timestamps := byteSliceToInt64(src[:bufSize])
	return src[bufSize:], timestamps, nil
}

// marshalMetricNameFast appends marshaled mn to dst and returns the result.
//
// The result must be unmarshaled with unmarshalMetricNameFast.
func marshalMetricNameFast(dst []byte, mn *storage.MetricName) []byte {
	dst = marshalBytesFast(dst, mn.MetricGroup)
	dst = encoding.MarshalUint16(dst, uint16(len(mn.Tags)))
	// There is no need in tags' sorting - they must be sorted after unmarshaling.
	return marshalMetricTagsFast(dst, mn.Tags)
}

// unmarshalMetricNameFast unmarshals mn from src, so mn members hold references to src.
//
// It is unsafe modifying src while mn is in use.
func unmarshalMetricNameFast(mn *storage.MetricName, src []byte) ([]byte, error) {
	mn.Reset()

	tail, metricGroup, err := unmarshalBytesFast(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal MetricGroup: %w", err)
	}
	src = tail
	mn.MetricGroup = metricGroup[:len(metricGroup):len(metricGroup)]

	if len(src) < 2 {
		return src, fmt.Errorf("not enough bytes for unmarshaling len(tags); need at least 2 bytes; got %d bytes", len(src))
	}
	tagsLen := encoding.UnmarshalUint16(src)
	src = src[2:]
	mn.Tags = slicesutil.SetLength(mn.Tags, int(tagsLen))
	for i := range mn.Tags {
		tail, key, err := unmarshalBytesFast(src)
		if err != nil {
			return tail, fmt.Errorf("cannot unmarshal key for tag[%d]: %w", i, err)
		}
		src = tail

		tail, value, err := unmarshalBytesFast(src)
		if err != nil {
			return tail, fmt.Errorf("cannot unmarshal value for tag[%d]: %w", i, err)
		}
		src = tail

		tag := &mn.Tags[i]
		tag.Key = key[:len(key):len(key)]
		tag.Value = value[:len(value):len(value)]
	}
	return src, nil
}

func marshalMetricTagsFast(dst []byte, tags []storage.Tag) []byte {
	for i := range tags {
		tag := &tags[i]
		dst = marshalBytesFast(dst, tag.Key)
		dst = marshalBytesFast(dst, tag.Value)
	}
	return dst
}

func marshalMetricNameSorted(dst []byte, mn *storage.MetricName) []byte {
	dst = marshalBytesFast(dst, mn.MetricGroup)
	return marshalMetricTagsSorted(dst, mn)
}

func marshalMetricTagsSorted(dst []byte, mn *storage.MetricName) []byte {
	sortMetricTags(mn)
	return marshalMetricTagsFast(dst, mn.Tags)
}

func marshalBytesFast(dst []byte, s []byte) []byte {
	dst = encoding.MarshalUint16(dst, uint16(len(s)))
	dst = append(dst, s...)
	return dst
}

func unmarshalBytesFast(src []byte) ([]byte, []byte, error) {
	if len(src) < 2 {
		return src, nil, fmt.Errorf("cannot decode size form src=%X; it must be at least 2 bytes", src)
	}
	n := encoding.UnmarshalUint16(src)
	src = src[2:]
	if len(src) < int(n) {
		return src, nil, fmt.Errorf("too short src=%X; it must be at least %d bytes", src, n)
	}
	return src[n:], src[:n], nil
}

func float64ToByteSlice(a []float64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(a))), len(a)*8)
}

func int64ToByteSlice(a []int64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(a))), len(a)*8)
}

func byteSliceToInt64(b []byte) []int64 {
	// Make sure that the returned slice is properly aligned to 8 bytes.
	// This prevents from SIGBUS error on arm architectures, which deny unaligned access.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3927
	if uintptr(unsafe.Pointer(unsafe.SliceData(b)))%8 != 0 {
		logger.Panicf("BUG: the input byte slice b must be aligned to 8 bytes")
	}
	return unsafe.Slice((*int64)(unsafe.Pointer(unsafe.SliceData(b))), len(b)/8)
}

func byteSliceToFloat64(b []byte) []float64 {
	// Make sure that the returned slice is properly aligned to 8 bytes.
	// This prevents from SIGBUS error on arm architectures, which deny unaligned access.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3927
	if uintptr(unsafe.Pointer(unsafe.SliceData(b)))%8 != 0 {
		logger.Panicf("BUG: the input byte slice b must be aligned to 8 bytes")
	}
	return unsafe.Slice((*float64)(unsafe.Pointer(unsafe.SliceData(b))), len(b)/8)
}

func stringMetricName(mn *storage.MetricName) string {
	var dst []byte
	dst = append(dst, mn.MetricGroup...)
	sortMetricTags(mn)
	dst = appendStringMetricTags(dst, mn.Tags)
	return string(dst)
}

func stringMetricTags(mn *storage.MetricName) string {
	var dst []byte
	sortMetricTags(mn)
	dst = appendStringMetricTags(dst, mn.Tags)
	return string(dst)
}

func appendStringMetricTags(dst []byte, tags []storage.Tag) []byte {
	dst = append(dst, '{')
	for i := range tags {
		tag := &tags[i]
		dst = append(dst, tag.Key...)
		dst = append(dst, '=')
		value := bytesutil.ToUnsafeString(tag.Value)
		dst = strconv.AppendQuote(dst, value)
		if i+1 < len(tags) {
			dst = append(dst, ", "...)
		}
	}
	dst = append(dst, '}')
	return dst
}

func assertIdenticalTimestamps(tss []*timeseries, step int64) {
	if len(tss) == 0 {
		return
	}
	tsGolden := tss[0]
	if len(tsGolden.Values) != len(tsGolden.Timestamps) {
		logger.Panicf("BUG: len(tsGolden.Values) must match len(tsGolden.Timestamps); got %d vs %d", len(tsGolden.Values), len(tsGolden.Timestamps))
	}
	if len(tsGolden.Timestamps) > 0 {
		prevTimestamp := tsGolden.Timestamps[0]
		for _, timestamp := range tsGolden.Timestamps[1:] {
			if timestamp-prevTimestamp != step {
				logger.Panicf("BUG: invalid step between timestamps; got %d; want %d; tsGolden.Timestamps=%d", timestamp-prevTimestamp, step, tsGolden.Timestamps)
			}
			prevTimestamp = timestamp
		}
	}
	for _, ts := range tss {
		if len(ts.Values) != len(tsGolden.Values) {
			logger.Panicf("BUG: unexpected len(ts.Values); got %d; want %d; ts.Values=%g", len(ts.Values), len(tsGolden.Values), ts.Values)
		}
		if len(ts.Timestamps) != len(tsGolden.Timestamps) {
			logger.Panicf("BUG: unexpected len(ts.Timestamps); got %d; want %d; ts.Timestamps=%d", len(ts.Timestamps), len(tsGolden.Timestamps), ts.Timestamps)
		}
		if len(ts.Timestamps) == 0 {
			continue
		}
		if &ts.Timestamps[0] == &tsGolden.Timestamps[0] {
			// Fast path - shared timestamps.
			continue
		}
		for i := range ts.Timestamps {
			if ts.Timestamps[i] != tsGolden.Timestamps[i] {
				logger.Panicf("BUG: timestamps mismatch at position %d; got %d; want %d; ts.Timestamps=%d, tsGolden.Timestamps=%d",
					i, ts.Timestamps[i], tsGolden.Timestamps[i], ts.Timestamps, tsGolden.Timestamps)
			}
		}
	}
}

func sortMetricTags(mn *storage.MetricName) {
	mts := (*metricTagsSorter)(mn)
	if !sort.IsSorted(mts) {
		sort.Sort(mts)
	}
}

type metricTagsSorter storage.MetricName

func (mts *metricTagsSorter) Len() int {
	return len(mts.Tags)
}

func (mts *metricTagsSorter) Less(i, j int) bool {
	a := mts.Tags
	return string(a[i].Key) < string(a[j].Key)
}

func (mts *metricTagsSorter) Swap(i, j int) {
	a := mts.Tags
	a[i], a[j] = a[j], a[i]
}
