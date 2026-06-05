package logstorage

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
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
	// tags contains added tags.
	tags []Field
}

// Reset resets st for reuse
func (st *StreamTags) Reset() {
	// Clear references to strings, which can belong to external buffers.
	// This guarantees that these buffers can be cleared by GC.
	clear(st.tags)

	st.tags = st.tags[:0]
}

// String returns string representation of st.
func (st *StreamTags) String() string {
	b := st.marshalString(nil)
	return string(b)
}

func (st *StreamTags) verifyCanonicalFieldValues(fields []Field) error {
	// Verify that the unmarshaled stream tags match the corresponding fields' values.
	// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/38

	prevTagName := ""
	for _, tag := range st.tags {
		tagName := tag.Name

		if err := CheckStreamFieldName(tagName); err != nil {
			return fmt.Errorf("invalid stream tag name: %s; streamTags: %s", tagName, st)
		}

		if tagName <= prevTagName {
			return fmt.Errorf("stream tag names must be sorted; got %q after %q; streamTags: %s", tagName, prevTagName, st)
		}

		tagValue := tag.Value
		found := false
		for _, f := range fields {
			if f.Name != tagName {
				continue
			}
			if f.Value != tagValue {
				line := MarshalFieldsToJSON(nil, fields)
				return fmt.Errorf("unexpected value for the stream tag %q; got %q; want %q; streamTags: %s; fields: %s", tagName, f.Value, tagValue, st, line)
			}
			found = true
		}
		if !found {
			line := MarshalFieldsToJSON(nil, fields)
			return fmt.Errorf("cannot find value for the stream tag %q in fields; want %q; streamTags: %s; fields: %s", tagName, tagValue, st, line)
		}
	}
	return nil
}

func (st *StreamTags) marshalString(dst []byte) []byte {
	dst = append(dst, '{')

	tags := st.tags
	if len(tags) > 0 {
		dst = tags[0].marshalToStreamTag(dst)
		tags = tags[1:]
		for i := range tags {
			dst = append(dst, ',')
			dst = tags[i].marshalToStreamTag(dst)
		}
	}

	dst = append(dst, '}')

	return dst
}

// unmarshalStringInplace unmarshals st from string representation stored at s received via marshalString().
//
// st points to s, so s mustn't be changed while st is in use.
func (st *StreamTags) unmarshalStringInplace(s string) error {
	st.Reset()

	var err error
	st.tags, err = parseStreamFields(st.tags[:0], s)
	return err
}

// Add adds (name:value) tag to st.
//
// name and value mustn't be changed while st is in use.
func (st *StreamTags) Add(name, value string) {
	if len(value) == 0 {
		return
	}

	if len(name) == 0 {
		name = "_msg"
	}

	st.tags = append(st.tags, Field{
		Name:  name,
		Value: value,
	})
}

// MarshalCanonical marshal st in a canonical way
func (st *StreamTags) MarshalCanonical(dst []byte) []byte {
	sort.Sort(st)

	tags := st.tags
	dst = encoding.MarshalVarUint64(dst, uint64(len(tags)))
	for i := range tags {
		tag := &tags[i]
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(tag.Name))
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(tag.Value))
	}
	return dst
}

// UnmarshalCanonicalInplace unmarshals st from src marshaled with MarshalCanonical.
//
// st points to src, so src mustn't be changed while st is in use.
func (st *StreamTags) UnmarshalCanonicalInplace(src []byte) ([]byte, error) {
	st.Reset()

	srcOrig := src

	n, nSize := encoding.UnmarshalVarUint64(src)
	if nSize <= 0 {
		return srcOrig, fmt.Errorf("cannot unmarshal tags len")
	}
	src = src[nSize:]
	for range n {
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

	if !sort.IsSorted(st) {
		return srcOrig, fmt.Errorf("stream tags must be sorted in alphabetical order; got unsorted: %s", st)
	}

	return src, nil
}

func getStreamTagsString(streamTagsCanonical string) string {
	st := GetStreamTags()
	mustUnmarshalStreamTagsInplace(st, streamTagsCanonical)
	s := st.String()
	PutStreamTags(st)

	return s
}

func mustUnmarshalStreamTagsInplace(dst *StreamTags, streamTagsCanonical string) {
	src := bytesutil.ToUnsafeBytes(streamTagsCanonical)
	tail, err := dst.UnmarshalCanonicalInplace(src)
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

const (
	escapeChar       = 0
	tagSeparatorChar = 1
	kvSeparatorChar  = 2
)

func marshalTagValue(dst []byte, src string) []byte {
	n1 := strings.IndexByte(src, escapeChar)
	n2 := strings.IndexByte(src, tagSeparatorChar)
	n3 := strings.IndexByte(src, kvSeparatorChar)
	if n1 < 0 && n2 < 0 && n3 < 0 {
		// Fast path.
		dst = append(dst, src...)
		dst = append(dst, tagSeparatorChar)
		return dst
	}

	// Slow path.
	for i := range len(src) {
		ch := src[i]
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

// CheckStreamFieldNames returns non-nil error if names contain prohibited chars, which cannot be used in stream field names.
func CheckStreamFieldNames(names []string) error {
	for _, name := range names {
		if err := CheckStreamFieldName(name); err != nil {
			return err
		}
	}
	return nil
}

// CheckStreamFieldName returns non-nil error if the name contain prohibited chars, which cannot be used in stream field names.
func CheckStreamFieldName(name string) error {
	// Do not use strings.ContainsAny because it is slower than two calls to strings.IndexByte()
	// TODO: replace this to strings.ContainsAny() when it will be optimized in Go standard library.
	// See BenchmarkCheckStreamFieldNames.

	if strings.IndexByte(name, '=') >= 0 {
		// The '=' cannot be located in stream field name, since it prevents from the proper parsing
		// when such a name is put inside _stream value.
		// For example:
		// - 'foo=bar' name cannot be parsed reliably in _stream={foo=bar="baz"}
		return fmt.Errorf("the %q cannot contain '=' char", name)
	}
	if strings.IndexByte(name, '}') >= 0 {
		// The '}' cannot be located in stream field name, since it prevents from the proper parsing
		// when such a name is put inside _stream value.
		// For example:
		// - 'foo}bar' name cannot be parsed reliably in _stream={foo}bar="baz"}
		return fmt.Errorf("the %q cannot contain '}' char", name)
	}

	return nil
}
