package logstorage

import (
	"math"
	"slices"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type statsMax struct {
	fields       []string
	containsStar bool
}

func (sm *statsMax) String() string {
	return "max(" + fieldNamesString(sm.fields) + ")"
}

func (sm *statsMax) neededFields() []string {
	return sm.fields
}

func (sm *statsMax) newStatsProcessor() (statsProcessor, int) {
	smp := &statsMaxProcessor{
		sm: sm,
	}
	return smp, int(unsafe.Sizeof(*smp))
}

type statsMaxProcessor struct {
	sm *statsMax

	max    string
	hasMax bool
}

func (smp *statsMaxProcessor) updateStatsForAllRows(br *blockResult) int {
	maxLen := len(smp.max)

	if smp.sm.containsStar {
		// Find the minimum value across all the columns
		for _, c := range br.getColumns() {
			smp.updateStateForColumn(br, c)
		}
	} else {
		// Find the minimum value across the requested columns
		for _, field := range smp.sm.fields {
			c := br.getColumnByName(field)
			smp.updateStateForColumn(br, c)
		}
	}

	return len(smp.max) - maxLen
}

func (smp *statsMaxProcessor) updateStatsForRow(br *blockResult, rowIdx int) int {
	maxLen := len(smp.max)

	if smp.sm.containsStar {
		// Find the minimum value across all the fields for the given row
		for _, c := range br.getColumns() {
			v := c.getValueAtRow(br, rowIdx)
			smp.updateStateString(v)
		}
	} else {
		// Find the minimum value across the requested fields for the given row
		for _, field := range smp.sm.fields {
			c := br.getColumnByName(field)
			v := c.getValueAtRow(br, rowIdx)
			smp.updateStateString(v)
		}
	}

	return maxLen - len(smp.max)
}

func (smp *statsMaxProcessor) mergeState(sfp statsProcessor) {
	src := sfp.(*statsMaxProcessor)
	if src.hasMax {
		smp.updateStateString(src.max)
	}
}

func (smp *statsMaxProcessor) updateStateForColumn(br *blockResult, c *blockResultColumn) {
	if len(br.timestamps) == 0 {
		return
	}

	if c.isTime {
		// Special case for time column
		timestamps := br.timestamps
		maxTimestamp := timestamps[len(timestamps)-1]
		for _, timestamp := range timestamps[:len(timestamps)-1] {
			if timestamp > maxTimestamp {
				maxTimestamp = timestamp
			}
		}

		bb := bbPool.Get()
		bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], maxTimestamp)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)

		return
	}
	if c.isConst {
		// Special case for const column
		v := c.encodedValues[0]
		smp.updateStateString(v)
		return
	}

	switch c.valueType {
	case valueTypeString:
		for _, v := range c.encodedValues {
			smp.updateStateString(v)
		}
	case valueTypeDict:
		for _, v := range c.dictValues {
			smp.updateStateString(v)
		}
	case valueTypeUint8:
		maxN := unmarshalUint8(c.encodedValues[0])
		for _, v := range c.encodedValues[1:] {
			n := unmarshalUint8(v)
			if n > maxN {
				maxN = n
			}
		}
		bb := bbPool.Get()
		bb.B = marshalUint8String(bb.B[:0], maxN)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeUint16:
		maxN := unmarshalUint16(c.encodedValues[0])
		for _, v := range c.encodedValues[1:] {
			n := unmarshalUint16(v)
			if n > maxN {
				maxN = n
			}
		}
		bb := bbPool.Get()
		bb.B = marshalUint16String(bb.B[:0], maxN)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeUint32:
		maxN := unmarshalUint32(c.encodedValues[0])
		for _, v := range c.encodedValues[1:] {
			n := unmarshalUint32(v)
			if n > maxN {
				maxN = n
			}
		}
		bb := bbPool.Get()
		bb.B = marshalUint32String(bb.B[:0], maxN)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeUint64:
		maxN := unmarshalUint64(c.encodedValues[0])
		for _, v := range c.encodedValues[1:] {
			n := unmarshalUint64(v)
			if n > maxN {
				maxN = n
			}
		}
		bb := bbPool.Get()
		bb.B = marshalUint64String(bb.B[:0], maxN)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeFloat64:
		maxF := unmarshalFloat64(c.encodedValues[0])
		for _, v := range c.encodedValues[1:] {
			f := unmarshalFloat64(v)
			if math.IsNaN(maxF) || f > maxF {
				maxF = f
			}
		}
		bb := bbPool.Get()
		bb.B = marshalFloat64String(bb.B[:0], maxF)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeIPv4:
		maxIP := unmarshalIPv4(c.encodedValues[0])
		for _, v := range c.encodedValues[1:] {
			ip := unmarshalIPv4(v)
			if ip > maxIP {
				maxIP = ip
			}
		}
		bb := bbPool.Get()
		bb.B = marshalIPv4String(bb.B[:0], maxIP)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	case valueTypeTimestampISO8601:
		maxTimestamp := unmarshalTimestampISO8601(c.encodedValues[0])
		for _, v := range c.encodedValues[1:] {
			timestamp := unmarshalTimestampISO8601(v)
			if timestamp > maxTimestamp {
				maxTimestamp = timestamp
			}
		}
		bb := bbPool.Get()
		bb.B = marshalTimestampISO8601String(bb.B[:0], maxTimestamp)
		smp.updateStateBytes(bb.B)
		bbPool.Put(bb)
	default:
		logger.Panicf("BUG: unknown valueType=%d", c.valueType)
	}
}

func (smp *statsMaxProcessor) updateStateBytes(b []byte) {
	v := bytesutil.ToUnsafeString(b)
	smp.updateStateString(v)
}

func (smp *statsMaxProcessor) updateStateString(v string) {
	if smp.hasMax && !lessString(smp.max, v) {
		return
	}
	smp.max = strings.Clone(v)
	smp.hasMax = true
}

func (smp *statsMaxProcessor) finalizeStats() string {
	if !smp.hasMax {
		return "NaN"
	}
	return smp.max
}

func parseStatsMax(lex *lexer) (*statsMax, error) {
	fields, err := parseFieldNamesForStatsFunc(lex, "max")
	if err != nil {
		return nil, err
	}
	sm := &statsMax{
		fields:       fields,
		containsStar: slices.Contains(fields, "*"),
	}
	return sm, nil
}
