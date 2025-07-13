package logstorage

import (
	"fmt"
	"maps"
	"sort"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsHistogram struct {
	fieldName string
}

func (sh *statsHistogram) String() string {
	return "histogram(" + quoteTokenIfNeeded(sh.fieldName) + ")"
}

func (sh *statsHistogram) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(sh.fieldName)
}

func (sh *statsHistogram) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsHistogramProcessor()
}

type statsHistogramProcessor struct {
	h metrics.Histogram

	// bucketsMap is initialized only in loadState().
	//
	// It contains additional state for h.
	bucketsMap map[string]uint64
}

func (shp *statsHistogramProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sh := sf.(*statsHistogram)

	c := br.getColumnByName(sh.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		f, ok := tryParseNumber(v)
		if ok {
			for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
				shp.h.Update(f)
			}
		}
		return 0
	}

	switch c.valueType {
	case valueTypeUint8:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint8(v)
			shp.h.Update(float64(n))
		}
	case valueTypeUint16:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint16(v)
			shp.h.Update(float64(n))
		}
	case valueTypeUint32:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint32(v)
			shp.h.Update(float64(n))
		}
	case valueTypeUint64:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint64(v)
			shp.h.Update(float64(n))
		}
	case valueTypeInt64:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalInt64(v)
			shp.h.Update(float64(n))
		}
	case valueTypeFloat64:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			f := unmarshalFloat64(v)
			shp.h.Update(f)
		}
	case valueTypeIPv4:
		// skip ipv4 values, since they cannot be represented as numbers
	case valueTypeTimestampISO8601:
		// skip iso8601 values, since they cannot be represented as numbers
	default:
		values := c.getValues(br)
		for _, v := range values {
			f, ok := tryParseNumber(v)
			if ok {
				shp.h.Update(f)
			}
		}
	}

	return 0
}

func (shp *statsHistogramProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sh := sf.(*statsHistogram)

	c := br.getColumnByName(sh.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		f, ok := tryParseNumber(v)
		if ok {
			shp.h.Update(f)
		}
		return 0
	}

	switch c.valueType {
	case valueTypeUint8:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint8(v)
		shp.h.Update(float64(n))
	case valueTypeUint16:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint16(v)
		shp.h.Update(float64(n))
	case valueTypeUint32:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint32(v)
		shp.h.Update(float64(n))
	case valueTypeUint64:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalUint64(v)
		shp.h.Update(float64(n))
	case valueTypeInt64:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		n := unmarshalInt64(v)
		shp.h.Update(float64(n))
	case valueTypeFloat64:
		values := c.getValuesEncoded(br)
		v := values[rowIdx]
		f := unmarshalFloat64(v)
		shp.h.Update(f)
	case valueTypeIPv4:
		// skip ipv4 values, since they cannot be represented as numbers
	case valueTypeTimestampISO8601:
		// skip iso8601 values, since they cannot be represented as numbers
	default:
		v := c.getValueAtRow(br, rowIdx)
		f, ok := tryParseNumber(v)
		if ok {
			shp.h.Update(f)
		}
	}

	return 0
}

func (shp *statsHistogramProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsHistogramProcessor)
	shp.h.Merge(&src.h)

	for vmrange, count := range src.bucketsMap {
		if shp.bucketsMap == nil {
			shp.bucketsMap = make(map[string]uint64)
		}
		shp.bucketsMap[vmrange] += count
	}
}

func (shp *statsHistogramProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	m := shp.getCompleteBucketsMap()

	dst = encoding.MarshalVarUint64(dst, uint64(len(m)))
	for vmrange, count := range m {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(vmrange))
		dst = encoding.MarshalVarUint64(dst, count)
	}

	return dst
}

func (shp *statsHistogramProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	shp.h.Reset()

	bucketsLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return 0, fmt.Errorf("cannot unmarshal bucketsLen")
	}
	src = src[n:]

	stateSizeIncrease := 0
	m := make(map[string]uint64, bucketsLen)
	for i := uint64(0); i < bucketsLen; i++ {
		v, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return 0, fmt.Errorf("cannot unmarshal vmrange")
		}
		vmrange := string(v)
		src = src[n:]

		count, n := encoding.UnmarshalVarUint64(src)
		if n <= 0 {
			return 0, fmt.Errorf("cannot unmarshal bucket count")
		}
		src = src[n:]

		m[vmrange] = count

		stateSizeIncrease += int(unsafe.Sizeof(vmrange)) + len(vmrange) + int(unsafe.Sizeof(count))
	}
	if len(src) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail left after decoding histogram; len(tail)=%d", len(src))
	}

	if len(m) == 0 {
		m = nil
	}
	shp.bucketsMap = m

	return stateSizeIncrease, nil
}

func (shp *statsHistogramProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	m := shp.getCompleteBucketsMap()

	vmranges := make([]string, 0, len(m))
	for vmrange := range m {
		vmranges = append(vmranges, vmrange)
	}
	sort.Slice(vmranges, func(i, j int) bool {
		return stringsutil.LessNatural(vmranges[i], vmranges[j])
	})

	dst = append(dst, '[')
	for _, vmrange := range vmranges {
		dst = append(dst, `{"vmrange":"`...)
		dst = append(dst, vmrange...)
		dst = append(dst, `","hits":`...)
		dst = marshalUint64String(dst, m[vmrange])
		dst = append(dst, `},`...)
	}
	dst = dst[:len(dst)-1]
	dst = append(dst, ']')
	return dst
}

func (shp *statsHistogramProcessor) getCompleteBucketsMap() map[string]uint64 {
	m := maps.Clone(shp.bucketsMap)
	if m == nil {
		m = make(map[string]uint64)
	}
	shp.h.VisitNonZeroBuckets(func(vmrange string, count uint64) {
		m[vmrange] += count
	})
	return m
}

func parseStatsHistogram(lex *lexer) (*statsHistogram, error) {
	fields, err := parseStatsFuncFields(lex, "histogram")
	if err != nil {
		return nil, fmt.Errorf("cannot parse field name: %w", err)
	}
	if len(fields) != 1 {
		return nil, fmt.Errorf("'histogram' accepts only a single field; got %d fields", len(fields))
	}

	sh := &statsHistogram{
		fieldName: fields[0],
	}
	return sh, nil
}
