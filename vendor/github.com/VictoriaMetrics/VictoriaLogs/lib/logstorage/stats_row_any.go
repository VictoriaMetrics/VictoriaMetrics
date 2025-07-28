package logstorage

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type statsRowAny struct {
	fieldFilters []string
}

func (sa *statsRowAny) String() string {
	return "row_any(" + fieldNamesString(sa.fieldFilters) + ")"
}

func (sa *statsRowAny) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilters(sa.fieldFilters)
}

func (sa *statsRowAny) newStatsProcessor(a *chunkedAllocator) statsProcessor {
	return a.newStatsRowAnyProcessor()
}

type statsRowAnyProcessor struct {
	captured bool

	fields []Field
}

func (sap *statsRowAnyProcessor) updateStatsForAllRows(sf statsFunc, br *blockResult) int {
	sa := sf.(*statsRowAny)
	if sap.captured {
		return 0
	}
	sap.captured = true

	return sap.updateState(sa, br, 0)
}

func (sap *statsRowAnyProcessor) updateStatsForRow(sf statsFunc, br *blockResult, rowIdx int) int {
	sa := sf.(*statsRowAny)
	if sap.captured {
		return 0
	}
	sap.captured = true

	return sap.updateState(sa, br, rowIdx)
}

func (sap *statsRowAnyProcessor) mergeState(_ *chunkedAllocator, _ statsFunc, sfp statsProcessor) {
	src := sfp.(*statsRowAnyProcessor)
	if !sap.captured {
		sap.captured = src.captured
		sap.fields = src.fields
	}
}

func (sap *statsRowAnyProcessor) exportState(dst []byte, _ <-chan struct{}) []byte {
	if !sap.captured {
		dst = append(dst, 0)
		return dst
	}

	dst = append(dst, 1)
	dst = marshalFields(dst, sap.fields)
	return dst
}

func (sap *statsRowAnyProcessor) importState(src []byte, _ <-chan struct{}) (int, error) {
	if len(src) == 0 {
		return 0, fmt.Errorf("missing `captured` flag")
	}
	sap.captured = (src[0] == 1)
	src = src[1:]

	if !sap.captured {
		sap.fields = nil
		return 0, nil
	}

	fields, tail, err := unmarshalFields(nil, src)
	if err != nil {
		return 0, fmt.Errorf("cannot unmarshal fields: %w", err)
	}
	if len(tail) > 0 {
		return 0, fmt.Errorf("unexpected non-empty tail left; len(tail)=%d", len(tail))
	}
	sap.fields = fields

	stateSize := fieldsStateSize(sap.fields)

	return stateSize, nil
}

func marshalFields(dst []byte, fields []Field) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(fields)))
	for _, f := range fields {
		dst = f.marshal(dst, true)
	}
	return dst
}

func unmarshalFields(dst []Field, src []byte) ([]Field, []byte, error) {
	fieldsLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return nil, src, fmt.Errorf("cannot unmarshal fieldsLen")
	}
	if fieldsLen > uint64(len(src)) {
		return nil, src, fmt.Errorf("too big fieldsLen=%d; it mustn't exceed %d", fieldsLen, len(src))
	}
	src = src[n:]

	dstLen := len(dst)
	dst = slicesutil.SetLength(dst, dstLen+int(fieldsLen))
	fields := dst[dstLen:]
	for i := range fields {
		f := &fields[i]
		tail, err := f.unmarshalInplace(src, true)
		if err != nil {
			return nil, tail, fmt.Errorf("cannot unmarshal field: %w", err)
		}
		src = tail

		f.Name = strings.Clone(f.Name)
		f.Value = strings.Clone(f.Value)
	}
	if len(dst) == 0 {
		dst = nil
	}
	return dst, src, nil
}

func fieldsStateSize(fields []Field) int {
	stateSize := int(unsafe.Sizeof(fields[0])) * len(fields)
	for _, f := range fields {
		stateSize += len(f.Name) + len(f.Value)
	}
	return stateSize
}

func (sap *statsRowAnyProcessor) updateState(sa *statsRowAny, br *blockResult, rowIdx int) int {
	stateSizeIncrease := 0
	sap.fields = sap.fields[:0]

	mc := getMatchingColumns(br, sa.fieldFilters)
	for _, c := range mc.cs {
		v := c.getValueAtRow(br, rowIdx)
		sap.fields = append(sap.fields, Field{
			Name:  strings.Clone(c.name),
			Value: strings.Clone(v),
		})
		stateSizeIncrease += len(c.name) + len(v)
	}
	putMatchingColumns(mc)

	return stateSizeIncrease
}

func (sap *statsRowAnyProcessor) finalizeStats(_ statsFunc, dst []byte, _ <-chan struct{}) []byte {
	sortFieldsByName(sap.fields)
	return MarshalFieldsToJSON(dst, sap.fields)
}

func parseStatsRowAny(lex *lexer) (*statsRowAny, error) {
	fieldFilters, err := parseStatsFuncFieldFilters(lex, "row_any")
	if err != nil {
		return nil, err
	}

	sa := &statsRowAny{
		fieldFilters: fieldFilters,
	}
	return sa, nil
}
