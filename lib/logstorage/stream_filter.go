package logstorage

import (
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
)

// StreamFilter is a filter for streams, e.g. `_stream:{...}`
type StreamFilter struct {
	orFilters []*andStreamFilter
}

func (sf *StreamFilter) isEmpty() bool {
	for _, af := range sf.orFilters {
		if len(af.tagFilters) > 0 {
			return false
		}
	}
	return true
}

func (sf *StreamFilter) marshalForCacheKey(dst []byte) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(sf.orFilters)))
	for _, af := range sf.orFilters {
		dst = encoding.MarshalVarUint64(dst, uint64(len(af.tagFilters)))
		for _, f := range af.tagFilters {
			dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.tagName))
			dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.op))
			dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.value))
		}
	}
	return dst
}

func (sf *StreamFilter) String() string {
	a := make([]string, len(sf.orFilters))
	for i := range a {
		a[i] = sf.orFilters[i].String()
	}
	return "{" + strings.Join(a, " or ") + "}"
}

type andStreamFilter struct {
	tagFilters []*streamTagFilter
}

func (af *andStreamFilter) String() string {
	a := make([]string, len(af.tagFilters))
	for i := range a {
		a[i] = af.tagFilters[i].String()
	}
	return strings.Join(a, ",")
}

// streamTagFilter is a filter for `tagName op value`
type streamTagFilter struct {
	// tagName is the name for the tag to filter
	tagName string
	// op is operation such as `=`, `!=`, `=~` or `!~`
	op string

	// value is the value
	value string

	regexpOnce sync.Once
	regexp     *regexutil.PromRegex
}

func (tf *streamTagFilter) getRegexp() *regexutil.PromRegex {
	tf.regexpOnce.Do(tf.initRegexp)
	return tf.regexp
}

func (tf *streamTagFilter) initRegexp() {
	re, err := regexutil.NewPromRegex(tf.value)
	if err != nil {
		logger.Panicf("BUG: cannot parse regexp %q: %s", tf.value, err)
	}
	tf.regexp = re
}

func (tf *streamTagFilter) String() string {
	return quoteTokenIfNeeded(tf.tagName) + tf.op + strconv.Quote(tf.value)
}
