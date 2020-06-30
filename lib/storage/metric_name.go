package storage

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

const (
	escapeChar       = 0
	tagSeparatorChar = 1
	kvSeparatorChar  = 2
)

// Tag represents a (key, value) tag for metric.
type Tag struct {
	Key   []byte
	Value []byte
}

// Reset resets the tag.
func (tag *Tag) Reset() {
	tag.Key = tag.Key[:0]
	tag.Value = tag.Value[:0]
}

// Equal returns true if tag equals t
func (tag *Tag) Equal(t *Tag) bool {
	return string(tag.Key) == string(t.Key) && string(tag.Value) == string(t.Value)
}

// Marshal appends marshaled tag to dst and returns the result.
func (tag *Tag) Marshal(dst []byte) []byte {
	dst = marshalTagValue(dst, tag.Key)
	dst = marshalTagValue(dst, tag.Value)
	return dst
}

// Unmarshal unmarshals tag from src and returns the remaining data from src.
func (tag *Tag) Unmarshal(src []byte) ([]byte, error) {
	var err error
	src, tag.Key, err = unmarshalTagValue(tag.Key[:0], src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal key: %w", err)
	}
	src, tag.Value, err = unmarshalTagValue(tag.Value[:0], src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal value: %w", err)
	}
	return src, nil
}

func (tag *Tag) copyFrom(src *Tag) {
	tag.Key = append(tag.Key[:0], src.Key...)
	tag.Value = append(tag.Value[:0], src.Value...)
}

func marshalTagValueNoTrailingTagSeparator(dst, src []byte) []byte {
	dst = marshalTagValue(dst, src)
	// Remove trailing tagSeparatorChar
	return dst[:len(dst)-1]
}

func marshalTagValue(dst, src []byte) []byte {
	n1 := bytes.IndexByte(src, escapeChar)
	n2 := bytes.IndexByte(src, tagSeparatorChar)
	n3 := bytes.IndexByte(src, kvSeparatorChar)
	if n1 < 0 && n2 < 0 && n3 < 0 {
		// Fast path.
		dst = append(dst, src...)
		dst = append(dst, tagSeparatorChar)
		return dst
	}

	// Slow path.
	for _, ch := range src {
		switch ch {
		case escapeChar:
			dst = append(dst, escapeChar, '0')
		case tagSeparatorChar:
			dst = append(dst, escapeChar, '1')
		case kvSeparatorChar:
			dst = append(dst, escapeChar, '2')
		default:
			dst = append(dst, ch)
		}
	}

	dst = append(dst, tagSeparatorChar)
	return dst
}

func unmarshalTagValue(dst, src []byte) ([]byte, []byte, error) {
	n := bytes.IndexByte(src, tagSeparatorChar)
	if n < 0 {
		return src, dst, fmt.Errorf("cannot find the end of tag value")
	}
	b := src[:n]
	src = src[n+1:]
	for {
		n := bytes.IndexByte(b, escapeChar)
		if n < 0 {
			dst = append(dst, b...)
			return src, dst, nil
		}
		dst = append(dst, b[:n]...)
		b = b[n+1:]
		if len(b) == 0 {
			return src, dst, fmt.Errorf("missing escaped char")
		}
		switch b[0] {
		case '0':
			dst = append(dst, escapeChar)
		case '1':
			dst = append(dst, tagSeparatorChar)
		case '2':
			dst = append(dst, kvSeparatorChar)
		default:
			return src, dst, fmt.Errorf("unsupported escaped char: %c", b[0])
		}
		b = b[1:]
	}
}

// MetricName reperesents a metric name.
type MetricName struct {
	MetricGroup []byte

	// Tags are optional. They must be sorted by tag Key for canonical view.
	// Use sortTags method.
	Tags []Tag
}

// GetMetricName returns a MetricName from pool.
func GetMetricName() *MetricName {
	v := mnPool.Get()
	if v == nil {
		return &MetricName{}
	}
	return v.(*MetricName)
}

// PutMetricName returns mn to the pool.
func PutMetricName(mn *MetricName) {
	mn.Reset()
	mnPool.Put(mn)
}

var mnPool sync.Pool

// Reset resets the mn.
func (mn *MetricName) Reset() {
	mn.MetricGroup = mn.MetricGroup[:0]
	mn.Tags = mn.Tags[:0]
}

// CopyFrom copies src to mn.
func (mn *MetricName) CopyFrom(src *MetricName) {
	if cap(mn.MetricGroup) > 0 {
		mn.MetricGroup = append(mn.MetricGroup[:0], src.MetricGroup...)
		mn.Tags = copyTags(mn.Tags[:0], src.Tags)
		return
	}

	// Pre-allocate a single byte slice for MetricGroup + all the tags.
	// This reduces the number of memory allocations for zero mn.
	size := len(src.MetricGroup)
	for i := range src.Tags {
		tag := &src.Tags[i]
		size += len(tag.Key)
		size += len(tag.Value)
	}
	b := make([]byte, 0, size)

	b = append(b, src.MetricGroup...)
	mn.MetricGroup = b[:len(b):len(b)]

	mn.Tags = make([]Tag, len(src.Tags))
	for i := range src.Tags {
		st := &src.Tags[i]
		dt := &mn.Tags[i]
		b = append(b, st.Key...)
		dt.Key = b[len(b)-len(st.Key) : len(b) : len(b)]
		b = append(b, st.Value...)
		dt.Value = b[len(b)-len(st.Value) : len(b) : len(b)]
	}
}

// AddTag adds new tag to mn with the given key and value.
func (mn *MetricName) AddTag(key, value string) {
	tag := mn.addNextTag()
	tag.Key = append(tag.Key[:0], key...)
	tag.Value = append(tag.Value[:0], value...)
}

// AddTagBytes adds new tag to mn with the given key and value.
func (mn *MetricName) AddTagBytes(key, value []byte) {
	tag := mn.addNextTag()
	tag.Key = append(tag.Key[:0], key...)
	tag.Value = append(tag.Value[:0], value...)
}

func (mn *MetricName) addNextTag() *Tag {
	if len(mn.Tags) < cap(mn.Tags) {
		mn.Tags = mn.Tags[:len(mn.Tags)+1]
	} else {
		mn.Tags = append(mn.Tags, Tag{})
	}
	return &mn.Tags[len(mn.Tags)-1]
}

// ResetMetricGroup resets mn.MetricGroup
func (mn *MetricName) ResetMetricGroup() {
	mn.MetricGroup = mn.MetricGroup[:0]
}

var metricGroupTagKey = []byte("__name__")

// RemoveTagsOn removes all the tags not included to onTags.
func (mn *MetricName) RemoveTagsOn(onTags []string) {
	if !hasTag(onTags, metricGroupTagKey) {
		mn.ResetMetricGroup()
	}
	tags := mn.Tags
	mn.Tags = mn.Tags[:0]
	if len(onTags) == 0 {
		return
	}
	for i := range tags {
		tag := &tags[i]
		if hasTag(onTags, tag.Key) {
			mn.AddTagBytes(tag.Key, tag.Value)
		}
	}
}

// RemoveTag removes a tag with the given tagKey
func (mn *MetricName) RemoveTag(tagKey string) {
	if tagKey == "__name__" {
		mn.ResetMetricGroup()
		return
	}
	tags := mn.Tags
	mn.Tags = mn.Tags[:0]
	for i := range tags {
		tag := &tags[i]
		if string(tag.Key) != tagKey {
			mn.AddTagBytes(tag.Key, tag.Value)
		}
	}
}

// RemoveTagsIgnoring removes all the tags included in ignoringTags.
func (mn *MetricName) RemoveTagsIgnoring(ignoringTags []string) {
	if len(ignoringTags) == 0 {
		return
	}
	if hasTag(ignoringTags, metricGroupTagKey) {
		mn.ResetMetricGroup()
	}
	tags := mn.Tags
	mn.Tags = mn.Tags[:0]
	for i := range tags {
		tag := &tags[i]
		if !hasTag(ignoringTags, tag.Key) {
			mn.AddTagBytes(tag.Key, tag.Value)
		}
	}
}

// GetTagValue returns tag value for the given tagKey.
func (mn *MetricName) GetTagValue(tagKey string) []byte {
	if tagKey == "__name__" {
		return mn.MetricGroup
	}
	tags := mn.Tags
	for i := range tags {
		tag := &tags[i]
		if string(tag.Key) == tagKey {
			return tag.Value
		}
	}
	return nil
}

// SetTags sets tags from src with keys matching addTags.
func (mn *MetricName) SetTags(addTags []string, src *MetricName) {
	for _, tagName := range addTags {
		if tagName == string(metricGroupTagKey) {
			mn.MetricGroup = append(mn.MetricGroup[:0], src.MetricGroup...)
			continue
		}
		var srcTag *Tag
		for i := range src.Tags {
			t := &src.Tags[i]
			if string(t.Key) == tagName {
				srcTag = t
				break
			}
		}
		if srcTag == nil {
			mn.RemoveTag(tagName)
			continue
		}
		found := false
		for i := range mn.Tags {
			t := &mn.Tags[i]
			if string(t.Key) == tagName {
				t.Value = append(t.Value[:0], srcTag.Value...)
				found = true
				break
			}
		}
		if !found {
			mn.AddTagBytes(srcTag.Key, srcTag.Value)
		}
	}
}

func hasTag(tags []string, key []byte) bool {
	for _, t := range tags {
		if t == string(key) {
			return true
		}
	}
	return false
}

// String returns user-readable representation of the metric name.
//
// Use this function only for debug logging.
func (mn *MetricName) String() string {
	mn.sortTags()
	var tags []string
	for i := range mn.Tags {
		t := &mn.Tags[i]
		tags = append(tags, fmt.Sprintf("%q=%q", t.Key, t.Value))
	}
	tagsStr := strings.Join(tags, ", ")
	return fmt.Sprintf("MetricGroup=%q, tags=[%s]", mn.MetricGroup, tagsStr)
}

// Marshal appends marshaled mn to dst and returns the result.
//
// Tags must be sorted before calling this function.
func (mn *MetricName) Marshal(dst []byte) []byte {
	// Calculate the required size and pre-allocate space in dst
	dstLen := len(dst)
	requiredSize := len(mn.MetricGroup) + 1
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		requiredSize += len(tag.Key) + len(tag.Value) + 2
	}
	dst = bytesutil.Resize(dst, requiredSize)
	dst = dst[:dstLen]

	// Marshal MetricGroup
	dst = marshalTagValue(dst, mn.MetricGroup)

	// Marshal tags.
	dst = marshalTags(dst, mn.Tags)
	return dst
}

// Unmarshal unmarshals mn from src.
func (mn *MetricName) Unmarshal(src []byte) error {
	// Unmarshal MetricGroup.
	var err error
	src, mn.MetricGroup, err = unmarshalTagValue(mn.MetricGroup[:0], src)
	if err != nil {
		return fmt.Errorf("cannot unmarshal MetricGroup: %w", err)
	}

	mn.Tags = mn.Tags[:0]
	for len(src) > 0 {
		tag := mn.addNextTag()
		var err error
		src, err = tag.Unmarshal(src)
		if err != nil {
			return fmt.Errorf("cannot unmarshal tag: %w", err)
		}
	}

	// There is no need in verifying for identical tag keys,
	// since they must be handled in MetricName.Marshal inside marshalTags.

	return nil
}

// The maximum length of label name.
//
// Longer names are truncated.
const maxLabelNameLen = 256

// The maximum length of label value.
//
// Longer values are truncated.
const maxLabelValueLen = 16 * 1024

// The maximum number of labels per each timeseries.
var maxLabelsPerTimeseries = 30

// SetMaxLabelsPerTimeseries sets the limit on the number of labels
// per each time series.
//
// Superfouos labels are dropped.
func SetMaxLabelsPerTimeseries(maxLabels int) {
	if maxLabels <= 0 {
		logger.Panicf("BUG: maxLabels must be positive; got %d", maxLabels)
	}
	maxLabelsPerTimeseries = maxLabels
}

// MarshalMetricNameRaw marshals labels to dst and returns the result.
//
// The result must be unmarshaled with MetricName.unmarshalRaw
func MarshalMetricNameRaw(dst []byte, labels []prompb.Label) []byte {
	// Calculate the required space for dst.
	dstLen := len(dst)
	dstSize := dstLen
	for i := range labels {
		if i >= maxLabelsPerTimeseries {
			atomic.AddUint64(&MetricsWithDroppedLabels, 1)
			break
		}
		label := &labels[i]
		if len(label.Name) > maxLabelNameLen {
			atomic.AddUint64(&TooLongLabelNames, 1)
			label.Name = label.Name[:maxLabelNameLen]
		}
		if len(label.Value) > maxLabelValueLen {
			atomic.AddUint64(&TooLongLabelValues, 1)
			label.Value = label.Value[:maxLabelValueLen]
		}
		if len(label.Value) == 0 {
			// Skip labels without values, since they have no sense in prometheus.
			continue
		}
		if string(label.Name) == "__name__" {
			label.Name = label.Name[:0]
		}
		dstSize += len(label.Name)
		dstSize += len(label.Value)
		dstSize += 4
	}
	dst = bytesutil.Resize(dst, dstSize)[:dstLen]

	// Marshal labels to dst.
	for i := range labels {
		if i >= maxLabelsPerTimeseries {
			break
		}
		label := &labels[i]
		if len(label.Value) == 0 {
			// Skip labels without values, since they have no sense in prometheus.
			continue
		}
		dst = marshalBytesFast(dst, label.Name)
		dst = marshalBytesFast(dst, label.Value)
	}
	return dst
}

var (
	// MetricsWithDroppedLabels is the number of metrics with at least a single dropped label
	MetricsWithDroppedLabels uint64

	// TooLongLabelNames is the number of too long label names
	TooLongLabelNames uint64

	// TooLongLabelValues is the number of too long label values
	TooLongLabelValues uint64
)

// marshalRaw marshals mn to dst and returns the result.
//
// The results may be unmarshaled with MetricName.unmarshalRaw.
//
// This function is for testing purposes. MarshalMetricNameRaw must be used
// in prod instead.
func (mn *MetricName) marshalRaw(dst []byte) []byte {
	dst = marshalBytesFast(dst, nil)
	dst = marshalBytesFast(dst, mn.MetricGroup)

	mn.sortTags()
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		dst = marshalBytesFast(dst, tag.Key)
		dst = marshalBytesFast(dst, tag.Value)
	}
	return dst
}

// unmarshalRaw unmarshals mn encoded with MarshalMetricNameRaw.
func (mn *MetricName) unmarshalRaw(src []byte) error {
	mn.Reset()
	for len(src) > 0 {
		tail, key, err := unmarshalBytesFast(src)
		if err != nil {
			return fmt.Errorf("cannot decode key: %w", err)
		}
		src = tail

		tail, value, err := unmarshalBytesFast(src)
		if err != nil {
			return fmt.Errorf("cannot decode value: %w", err)
		}
		src = tail

		if len(key) == 0 {
			mn.MetricGroup = append(mn.MetricGroup[:0], value...)
		} else {
			mn.AddTagBytes(key, value)
		}
	}
	return nil
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

// sortTags sorts tags in mn.
//
// Tags sorting is quite slow, so try avoiding it by caching mn
// with sorted tags.
func (mn *MetricName) sortTags() {
	if len(mn.Tags) == 0 {
		return
	}

	cts := getCanonicalTags()
	if n := len(mn.Tags) - cap(cts.tags); n > 0 {
		cts.tags = append(cts.tags[:cap(cts.tags)], make([]canonicalTag, n)...)
	}
	dst := cts.tags[:len(mn.Tags)]
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		ct := &dst[i]
		ct.key = normalizeTagKey(tag.Key)
		ct.tag.copyFrom(tag)
	}
	cts.tags = dst

	// Use sort.Sort instead of sort.Slice, since sort.Slice allocates a lot.
	sort.Sort(&cts.tags)

	for i := range cts.tags {
		mn.Tags[i].copyFrom(&cts.tags[i].tag)
	}

	putCanonicalTags(cts)
}

func getCanonicalTags() *canonicalTags {
	v := canonicalTagsPool.Get()
	if v == nil {
		return &canonicalTags{}
	}
	return v.(*canonicalTags)
}

func putCanonicalTags(cts *canonicalTags) {
	cts.tags = cts.tags[:0]
	canonicalTagsPool.Put(cts)
}

var canonicalTagsPool sync.Pool

type canonicalTags struct {
	tags canonicalTagsSort
}

type canonicalTag struct {
	key []byte
	tag Tag
}

type canonicalTagsSort []canonicalTag

func (ts *canonicalTagsSort) Len() int { return len(*ts) }
func (ts *canonicalTagsSort) Less(i, j int) bool {
	x := *ts
	return string(x[i].key) < string(x[j].key)
}
func (ts *canonicalTagsSort) Swap(i, j int) {
	x := *ts
	x[i], x[j] = x[j], x[i]
}

func marshalTags(dst []byte, tags []Tag) []byte {
	var prevKey []byte
	for i := range tags {
		t := &tags[i]
		if string(prevKey) == string(t.Key) {
			// Skip duplicate keys, since they aren't allowed in Prometheus data model.
			continue
		}
		prevKey = t.Key
		dst = t.Marshal(dst)
	}
	return dst
}

func copyTags(dst, src []Tag) []Tag {
	dstLen := len(dst)
	if n := dstLen + len(src) - cap(dst); n > 0 {
		dst = append(dst[:cap(dst)], make([]Tag, n)...)
	}
	dst = dst[:dstLen+len(src)]
	for i := range src {
		dst[dstLen+i].copyFrom(&src[i])
	}
	return dst
}

var commonTagKeys = func() map[string][]byte {
	lcm := map[string][]byte{
		// job-like tags must go first in MetricName.Tags.
		// This should improve data locality.
		// They start with \x00\x00.
		// Do not change values!
		//
		// TODO: add more job-like tags.
		"namespace":   []byte("\x00\x00\x00"),
		"ns":          []byte("\x00\x00\x01"),
		"datacenter":  []byte("\x00\x00\x08"),
		"dc":          []byte("\x00\x00\x09"),
		"environment": []byte("\x00\x00\x0c"),
		"env":         []byte("\x00\x00\x0d"),
		"cluster":     []byte("\x00\x00\x10"),
		"service":     []byte("\x00\x00\x18"),
		"job":         []byte("\x00\x00\x20"),
		"model":       []byte("\x00\x00\x28"),
		"type":        []byte("\x00\x00\x30"),
		"sensor_type": []byte("\x00\x00\x38"),
		"SensorType":  []byte("\x00\x00\x38"),
		"db":          []byte("\x00\x00\x40"),

		// instance-like tags must go second in MetricName.Tags.
		// This should improve data locality.
		// They start with \x00\x01.
		// Do not change values!
		//
		// TODO: add more instance-like tags.
		"instance":    []byte("\x00\x01\x00"),
		"host":        []byte("\x00\x01\x08"),
		"server":      []byte("\x00\x01\x10"),
		"pod":         []byte("\x00\x01\x18"),
		"node":        []byte("\x00\x01\x20"),
		"device":      []byte("\x00\x01\x28"),
		"tenant":      []byte("\x00\x01\x30"),
		"client":      []byte("\x00\x01\x38"),
		"name":        []byte("\x00\x01\x40"),
		"measurement": []byte("\x00\x01\x48"),
	}

	// Generate Upper-case variants of lc
	m := make(map[string][]byte, len(lcm)*2)
	for k, v := range lcm {
		s := strings.ToUpper(k[:1]) + k[1:]
		m[k] = v
		m[s] = v
	}
	return m
}()

func normalizeTagKey(key []byte) []byte {
	tagKey := commonTagKeys[string(key)]
	if tagKey == nil {
		return key
	}
	return tagKey
}
