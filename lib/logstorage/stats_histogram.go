package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/metrics"
)

type statsHistogram struct {
	fieldName string
}

func (sh *statsHistogram) String() string {
	return "histogram(" + quoteTokenIfNeeded(sh.fieldName) + ")"
}

func (sh *statsHistogram) updateNeededFields(neededFields fieldsSet) {
	updateNeededFieldsForStatsFunc(neededFields, []string{sh.fieldName})
}

func (sh *statsHistogram) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsHistogramProcessor()
}

type statsHistogramProcessor struct {
	h metrics.Histogram
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
}

func (shp *statsHistogramProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	dst = append(dst, '[')
	shp.h.VisitNonZeroBuckets(func(vmrange string, count uint64) {
		dst = append(dst, `{"vmrange":"`...)
		dst = append(dst, vmrange...)
		dst = append(dst, `","hits":`...)
		dst = marshalUint64String(dst, count)
		dst = append(dst, `},`...)
	})
	dst = dst[:len(dst)-1]
	dst = append(dst, ']')
	return dst
}

func parseStatsHistogram(lex *lexer) (*statsHistogram, error) {
	fields, err := parseStatsFuncFields(lex, "histogram")
	if err != nil {
		return nil, fmt.Errorf("cannot parse field name: %w", err)
	}
	if len(fields) != 1 {
		return nil, fmt.Errorf("unexpected number of fields; got %d; want 1", len(fields))
	}

	sh := &statsHistogram{
		fieldName: fields[0],
	}
	return sh, nil
}
