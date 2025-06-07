package logstorage

import (
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

func updateNeededFieldsForPipePack(pf *prefixfilter.Filter, resultField string, fieldFilters []string) {
	if pf.MatchString(resultField) {
		pf.AddDenyFilter(resultField)
		if len(fieldFilters) > 0 {
			pf.AddAllowFilters(fieldFilters)
		} else {
			pf.AddAllowFilter("*")
		}
	}
}

func newPipePackProcessor(ppNext pipeProcessor, resultField string, fields []string, marshalFields func(dst []byte, fields []Field) []byte) pipeProcessor {
	return &pipePackProcessor{
		ppNext:        ppNext,
		resultField:   resultField,
		fields:        fields,
		marshalFields: marshalFields,
	}
}

type pipePackProcessor struct {
	ppNext        pipeProcessor
	resultField   string
	fields        []string
	marshalFields func(dst []byte, fields []Field) []byte

	shards atomicutil.Slice[pipePackProcessorShard]
}

type pipePackProcessorShard struct {
	rc resultColumn

	buf    []byte
	fields []Field

	cs []*blockResultColumn
}

func (ppp *pipePackProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := ppp.shards.Get(workerID)

	shard.rc.name = ppp.resultField

	csAll := br.getColumns()
	cs := shard.cs[:0]
	if len(ppp.fields) == 0 {
		cs = append(cs, csAll...)
	} else {
		for _, c := range csAll {
			for _, f := range ppp.fields {
				if c.name == f || strings.HasSuffix(f, "*") && strings.HasPrefix(c.name, f[:len(f)-1]) {
					cs = append(cs, c)
				}
			}
		}
	}
	shard.cs = cs

	buf := shard.buf[:0]
	fields := shard.fields
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		fields = fields[:0]
		for _, c := range cs {
			v := c.getValueAtRow(br, rowIdx)
			fields = append(fields, Field{
				Name:  c.name,
				Value: v,
			})
		}

		bufLen := len(buf)
		buf = ppp.marshalFields(buf, fields)
		v := bytesutil.ToUnsafeString(buf[bufLen:])
		shard.rc.addValue(v)
	}
	shard.fields = fields

	br.addResultColumn(shard.rc)
	ppp.ppNext.writeBlock(workerID, br)

	shard.rc.reset()
}

func (ppp *pipePackProcessor) flush() error {
	return nil
}
