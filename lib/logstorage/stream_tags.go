package logstorage

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// GetStreamTags returns a StreamTags from pool.
func GetStreamTags() *StreamTags {
	v := streamTagsPool.Get()
	if v == nil {
		return &StreamTags{}
	}
	return v.(*StreamTags)
}

// PutStreamTags returns st to the pool.
func PutStreamTags(st *StreamTags) {
	st.Reset()
	streamTagsPool.Put(st)
}

var streamTagsPool sync.Pool

// StreamTags contains stream tags.
type StreamTags struct {
	// buf holds all the data backed by tags
	buf []byte

	// tags contains added tags.
	tags []streamTag
}

// Reset resets st for reuse
func (st *StreamTags) Reset() {
	st.buf = st.buf[:0]

	tags := st.tags
	clear(tags)
	st.tags = tags[:0]
}

// String returns string representation of st.
func (st *StreamTags) String() string {
	b := st.marshalString(nil)
	return string(b)
}

func (st *StreamTags) marshalString(dst []byte) []byte {
	dst = append(dst, '{')

	tags := st.tags
	if len(tags) > 0 {
		dst = tags[0].marshalString(dst)
		tags = tags[1:]
		for i := range tags {
			dst = append(dst, ',')
			dst = tags[i].marshalString(dst)
		}
	}

	dst = append(dst, '}')

	return dst
}

// Add adds (name:value) tag to st.
func (st *StreamTags) Add(name, value string) {
	if len(value) == 0 {
		return
	}

	if len(name) == 0 {
		name = "_msg"
	}

	buf := st.buf

	bufLen := len(buf)
	buf = append(buf, name...)
	bName := buf[bufLen:]

	bufLen = len(buf)
	buf = append(buf, value...)
	bValue := buf[bufLen:]

	st.buf = buf

	st.tags = append(st.tags, streamTag{
		Name:  bName,
		Value: bValue,
	})
}

// MarshalCanonical marshal st in a canonical way
func (st *StreamTags) MarshalCanonical(dst []byte) []byte {
	sort.Sort(st)

	tags := st.tags
	dst = encoding.MarshalVarUint64(dst, uint64(len(tags)))
	for i := range tags {
		tag := &tags[i]
		dst = encoding.MarshalBytes(dst, tag.Name)
		dst = encoding.MarshalBytes(dst, tag.Value)
	}
	return dst
}

// UnmarshalCanonical unmarshals st from src marshaled with MarshalCanonical.
func (st *StreamTags) UnmarshalCanonical(src []byte) ([]byte, error) {
	st.Reset()

	srcOrig := src

	n, nSize := encoding.UnmarshalVarUint64(src)
	if nSize <= 0 {
		return srcOrig, fmt.Errorf("cannot unmarshal tags len")
	}
	src = src[nSize:]
	for i := uint64(0); i < n; i++ {
		name, nSize := encoding.UnmarshalBytes(src)
		if nSize <= 0 {
			return srcOrig, fmt.Errorf("cannot unmarshal tag name")
		}
		src = src[nSize:]

		value, nSize := encoding.UnmarshalBytes(src)
		if nSize <= 0 {
			return srcOrig, fmt.Errorf("cannot unmarshal tag value")
		}
		src = src[nSize:]

		sName := bytesutil.ToUnsafeString(name)
		sValue := bytesutil.ToUnsafeString(value)
		st.Add(sName, sValue)
	}

	return src, nil
}

func getStreamTagsString(streamTagsCanonical string) string {
	st := GetStreamTags()
	mustUnmarshalStreamTags(st, streamTagsCanonical)
	s := st.String()
	PutStreamTags(st)

	return s
}

func mustUnmarshalStreamTags(dst *StreamTags, streamTagsCanonical string) {
	src := bytesutil.ToUnsafeBytes(streamTagsCanonical)
	tail, err := dst.UnmarshalCanonical(src)
	if err != nil {
		logger.Panicf("FATAL: cannot unmarshal StreamTags: %s", err)
	}
	if len(tail) > 0 {
		logger.Panicf("FATAL: unexpected tail left after unmarshaling StreamTags; len(tail)=%d; tail=%q", len(tail), tail)
	}
}

// Len returns the number of tags in st.
func (st *StreamTags) Len() int {
	return len(st.tags)
}

// Less returns true if tag i is smaller than the tag j.
func (st *StreamTags) Less(i, j int) bool {
	tags := st.tags
	return tags[i].less(&tags[j])
}

// Swap swaps i and j tags
func (st *StreamTags) Swap(i, j int) {
	tags := st.tags
	tags[i], tags[j] = tags[j], tags[i]
}

// streamTag represents a (name:value) tag for stream.
type streamTag struct {
	Name  []byte
	Value []byte
}

func (tag *streamTag) marshalString(dst []byte) []byte {
	dst = append(dst, tag.Name...)
	dst = append(dst, '=')
	dst = strconv.AppendQuote(dst, bytesutil.ToUnsafeString(tag.Value))
	return dst
}

// reset resets the tag.
func (tag *streamTag) reset() {
	tag.Name = tag.Name[:0]
	tag.Value = tag.Value[:0]
}

func (tag *streamTag) equal(t *streamTag) bool {
	return string(tag.Name) == string(t.Name) && string(tag.Value) == string(t.Value)
}

func (tag *streamTag) less(t *streamTag) bool {
	if string(tag.Name) != string(t.Name) {
		return string(tag.Name) < string(t.Name)
	}
	return string(tag.Value) < string(t.Value)
}

func (tag *streamTag) indexdbMarshal(dst []byte) []byte {
	dst = marshalTagValue(dst, tag.Name)
	dst = marshalTagValue(dst, tag.Value)
	return dst
}

func (tag *streamTag) indexdbUnmarshal(src []byte) ([]byte, error) {
	var err error
	src, tag.Name, err = unmarshalTagValue(tag.Name[:0], src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal key: %w", err)
	}
	src, tag.Value, err = unmarshalTagValue(tag.Value[:0], src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal value: %w", err)
	}
	return src, nil
}

const (
	escapeChar       = 0
	tagSeparatorChar = 1
	kvSeparatorChar  = 2
)

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
