package common

import (
	"encoding/json"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/valyala/fastjson"
	"sync"
)

type ParserCtx struct {
	p         fastjson.Parser
	buf       []byte
	prefixBuf []byte
	fields    []logstorage.Field
}

func (pctx *ParserCtx) Reset() {
	pctx.buf = pctx.buf[:0]
	pctx.prefixBuf = pctx.prefixBuf[:0]

	fields := pctx.fields
	for i := range fields {
		lf := &fields[i]
		lf.Name = ""
		lf.Value = ""
	}
	pctx.fields = fields[:0]
}

func GetParserCtx() *ParserCtx {
	v := parserCtxPool.Get()
	if v == nil {
		return &ParserCtx{}
	}
	return v.(*ParserCtx)
}

func PutParserCtx(pctx *ParserCtx) {
	pctx.Reset()
	parserCtxPool.Put(pctx)
}

func (pctx *ParserCtx) Fields() []logstorage.Field {
	return pctx.fields[:]
}

func (pctx *ParserCtx) ParseLogMessage(msg json.RawMessage) error {
	s := bytesutil.ToUnsafeString(msg)
	v, err := pctx.p.Parse(s)
	if err != nil {
		return fmt.Errorf("cannot parse json: %w", err)
	}
	if t := v.Type(); t != fastjson.TypeObject {
		return fmt.Errorf("expecting json dictionary; got %s", t)
	}
	pctx.Reset()
	pctx.fields, pctx.buf, pctx.prefixBuf = appendLogFields(pctx.fields, pctx.buf, pctx.prefixBuf, v)
	return nil
}

func (pctx *ParserCtx) RenameField(name string, newName string) {
	if name == "" {
		return
	}
	for i := range pctx.fields {
		f := &pctx.fields[i]
		if f.Name == name {
			f.Name = newName
			return
		}
	}
}

func appendLogFields(dst []logstorage.Field, dstBuf, prefixBuf []byte, v *fastjson.Value) ([]logstorage.Field, []byte, []byte) {
	o := v.GetObject()
	o.Visit(func(k []byte, v *fastjson.Value) {
		t := v.Type()
		switch t {
		case fastjson.TypeNull:
			// Skip nulls
		case fastjson.TypeObject:
			// Flatten nested JSON objects.
			// For example, {"foo":{"bar":"baz"}} is converted to {"foo.bar":"baz"}
			prefixLen := len(prefixBuf)
			prefixBuf = append(prefixBuf, k...)
			prefixBuf = append(prefixBuf, '.')
			dst, dstBuf, prefixBuf = appendLogFields(dst, dstBuf, prefixBuf, v)
			prefixBuf = prefixBuf[:prefixLen]
		case fastjson.TypeArray, fastjson.TypeNumber, fastjson.TypeTrue, fastjson.TypeFalse:
			// Convert JSON arrays, numbers, true and false values to their string representation
			dstBufLen := len(dstBuf)
			dstBuf = v.MarshalTo(dstBuf)
			value := dstBuf[dstBufLen:]
			dst, dstBuf = appendLogField(dst, dstBuf, prefixBuf, k, value)
		case fastjson.TypeString:
			// Decode JSON strings
			dstBufLen := len(dstBuf)
			dstBuf = append(dstBuf, v.GetStringBytes()...)
			value := dstBuf[dstBufLen:]
			dst, dstBuf = appendLogField(dst, dstBuf, prefixBuf, k, value)
		default:
			logger.Panicf("BUG: unexpected JSON type: %s", t)
		}
	})
	return dst, dstBuf, prefixBuf
}

func appendLogField(dst []logstorage.Field, dstBuf, prefixBuf, k, value []byte) ([]logstorage.Field, []byte) {
	dstBufLen := len(dstBuf)
	dstBuf = append(dstBuf, prefixBuf...)
	dstBuf = append(dstBuf, k...)
	name := dstBuf[dstBufLen:]

	dst = append(dst, logstorage.Field{
		Name:  bytesutil.ToUnsafeString(name),
		Value: bytesutil.ToUnsafeString(value),
	})
	return dst, dstBuf
}

var parserCtxPool sync.Pool
