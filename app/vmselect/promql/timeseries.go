package promql

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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
		logger.Panicf("BUG: tss cannot be empty")
	}

	// Calculate the required size for marshaled tss.
	size := 0
	for _, ts := range tss {
		size += ts.marshaledFastSizeNoTimestamps()
	}
	// timestamps are stored only once for all the tss, since they are identical.
	assertIdenticalTimestamps(tss, step)
	size += 8 * len(tss[0].Timestamps)

	if size > maxSize {
		// Do not marshal tss, since it would occupy too much space
		return dst
	}

	// Allocate the buffer for the marshaled tss before its' marshaling.
	// This should reduce memory fragmentation and memory usage.
	dst = bytesutil.Resize(dst, size)
	dst = marshalFastTimestamps(dst[:0], tss[0].Timestamps)
	for _, ts := range tss {
		dst = ts.marshalFastNoTimestamps(dst)
	}
	return dst
}

// unmarshalTimeseriesFast unmarshals timeseries from src.
//
// The returned timeseries refer to src, so it is unsafe to modify it
// until timeseries are in use.
func unmarshalTimeseriesFast(src []byte) ([]*timeseries, error) {
	tail, timestamps, err := unmarshalFastTimestamps(src)
	if err != nil {
		return nil, err
	}
	src = tail

	var tss []*timeseries
	for len(src) > 0 {
		var ts timeseries
		ts.denyReuse = false
		ts.Timestamps = timestamps

		tail, err := ts.unmarshalFastNoTimestamps(src)
		if err != nil {
			return nil, err
		}
		src = tail

		tss = append(tss, &ts)
	}
	return tss, nil
}

// marshaledFastSizeNoTimestamps returns the size of marshaled ts
// returned from marshalFastNoTimestamps.
func (ts *timeseries) marshaledFastSizeNoTimestamps() int {
	mn := &ts.MetricName
	n := 2 + len(mn.MetricGroup)
	n += 2 // Length of tags.
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		n += 2 + len(tag.Key)
		n += 2 + len(tag.Value)
	}
	n += 8 * len(ts.Values)
	return n
}

// marshalFastNoTimestamps appends marshaled ts to dst and returns the result.
//
// It doesn't marshal timestamps.
//
// The result must be unmarshaled with unmarshalFastNoTimestamps.
func (ts *timeseries) marshalFastNoTimestamps(dst []byte) []byte {
	mn := &ts.MetricName
	dst = marshalBytesFast(dst, mn.MetricGroup)
	dst = encoding.MarshalUint16(dst, uint16(len(mn.Tags)))
	// There is no need in tags' sorting - they must be sorted after unmarshaling.
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		dst = marshalBytesFast(dst, tag.Key)
		dst = marshalBytesFast(dst, tag.Value)
	}

	// Do not marshal len(ts.Values), since it is already encoded as len(ts.Timestamps)
	// during marshalFastTimestamps.
	var valuesBuf []byte
	if len(ts.Values) > 0 {
		valuesBuf = float64ToByteSlice(ts.Values)
	}
	dst = append(dst, valuesBuf...)
	return dst
}

func marshalFastTimestamps(dst []byte, timestamps []int64) []byte {
	dst = encoding.MarshalUint32(dst, uint32(len(timestamps)))
	var timestampsBuf []byte
	if len(timestamps) > 0 {
		timestampsBuf = int64ToByteSlice(timestamps)
	}
	dst = append(dst, timestampsBuf...)
	return dst
}

// it is unsafe modifying src while the returned timestamps is in use.
func unmarshalFastTimestamps(src []byte) ([]byte, []int64, error) {
	if len(src) < 4 {
		return src, nil, fmt.Errorf("cannot decode len(timestamps); got %d bytes; want at least %d bytes", len(src), 4)
	}
	timestampsCount := int(encoding.UnmarshalUint32(src))
	src = src[4:]
	if timestampsCount == 0 {
		return src, nil, nil
	}

	bufSize := timestampsCount * 8
	if len(src) < bufSize {
		return src, nil, fmt.Errorf("cannot unmarshal timestamps; got %d bytes; want at least %d bytes", len(src), bufSize)
	}
	timestamps := byteSliceToInt64(src[:bufSize])
	src = src[bufSize:]

	return src, timestamps, nil
}

// unmarshalFastNoTimestamps unmarshals ts from src, so ts members reference src.
//
// It is expected that ts.Timestamps is already unmarshaled.
//
// It is unsafe to modify src while ts is in use.
func (ts *timeseries) unmarshalFastNoTimestamps(src []byte) ([]byte, error) {
	// ts members point to src, so they cannot be re-used.
	ts.denyReuse = true

	tail, err := unmarshalMetricNameFast(&ts.MetricName, src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal MetricName: %w", err)
	}
	src = tail

	valuesCount := len(ts.Timestamps)
	if valuesCount == 0 {
		return src, nil
	}
	bufSize := valuesCount * 8
	if len(src) < bufSize {
		return src, fmt.Errorf("cannot unmarshal values; got %d bytes; want at least %d bytes", len(src), bufSize)
	}
	ts.Values = byteSliceToFloat64(src[:bufSize])

	return src[bufSize:], nil
}

func float64ToByteSlice(a []float64) (b []byte) {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh.Data = uintptr(unsafe.Pointer(&a[0]))
	sh.Len = len(a) * int(unsafe.Sizeof(a[0]))
	sh.Cap = sh.Len
	return
}

func int64ToByteSlice(a []int64) (b []byte) {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh.Data = uintptr(unsafe.Pointer(&a[0]))
	sh.Len = len(a) * int(unsafe.Sizeof(a[0]))
	sh.Cap = sh.Len
	return
}

func byteSliceToInt64(b []byte) (a []int64) {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&a))
	sh.Data = uintptr(unsafe.Pointer(&b[0]))
	sh.Len = len(b) / int(unsafe.Sizeof(a[0]))
	sh.Cap = sh.Len
	return
}

func byteSliceToFloat64(b []byte) (a []float64) {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&a))
	sh.Data = uintptr(unsafe.Pointer(&b[0]))
	sh.Len = len(b) / int(unsafe.Sizeof(a[0]))
	sh.Cap = sh.Len
	return
}

// unmarshalMetricNameFast unmarshals mn from src, so mn members
// hold references to src.
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
	if n := int(tagsLen) - cap(mn.Tags); n > 0 {
		mn.Tags = append(mn.Tags[:cap(mn.Tags)], make([]storage.Tag, n)...)
	}
	mn.Tags = mn.Tags[:tagsLen]
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
	sortMetricTags(mn.Tags)
	dst = marshalMetricTagsFast(dst, mn.Tags)
	return dst
}

func marshalMetricTagsSorted(dst []byte, mn *storage.MetricName) []byte {
	sortMetricTags(mn.Tags)
	return marshalMetricTagsFast(dst, mn.Tags)
}

func sortMetricTags(tags []storage.Tag) {
	less := func(i, j int) bool {
		return string(tags[i].Key) < string(tags[j].Key)
	}
	if sort.SliceIsSorted(tags, less) {
		return
	}
	sort.Slice(tags, less)
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

func stringMetricName(mn *storage.MetricName) string {
	var dst []byte
	dst = append(dst, mn.MetricGroup...)
	sortMetricTags(mn.Tags)
	dst = appendStringMetricTags(dst, mn.Tags)
	return string(dst)
}

func stringMetricTags(mn *storage.MetricName) string {
	var dst []byte
	sortMetricTags(mn.Tags)
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
