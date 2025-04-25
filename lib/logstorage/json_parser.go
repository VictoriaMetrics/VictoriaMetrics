package logstorage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/valyala/fastjson"
)

// JSONParser parses a single JSON log message into Fields.
//
// See https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model
//
// Use GetParser() for obtaining the parser.
type JSONParser struct {
	// Fields contains the parsed JSON line after Parse() call
	//
	// The Fields are valid until the next call to ParseLogMessage()
	// or until the parser is returned to the pool with PutParser() call.
	Fields []Field

	// p is used for fast JSON parsing
	p fastjson.Parser

	// buf is used for holding the backing data for Fields
	buf []byte

	// prefixBuf is used for holding the current key prefix
	// when it is composed from multiple keys.
	prefixBuf []byte
}

func (p *JSONParser) reset() {
	clear(p.Fields)
	p.Fields = p.Fields[:0]

	p.buf = p.buf[:0]
}

// GetJSONParser returns JSONParser ready to parse JSON lines.
//
// Return the parser to the pool when it is no longer needed by calling PutJSONParser().
func GetJSONParser() *JSONParser {
	v := parserPool.Get()
	if v == nil {
		return &JSONParser{}
	}
	return v.(*JSONParser)
}

// PutJSONParser returns the parser to the pool.
//
// The parser cannot be used after returning to the pool.
func PutJSONParser(p *JSONParser) {
	p.reset()
	parserPool.Put(p)
}

var parserPool sync.Pool

// ParseLogMessage parses the given JSON log message msg into p.Fields.
//
// The p.Fields remains valid until the next call to ParseLogMessage() or PutJSONParser().
func (p *JSONParser) ParseLogMessage(msg []byte) error {
	return p.parseLogMessage(msg, maxFieldNameSize)
}

// ParseLogMessage parses the given JSON log message msg into p.Fields.
//
// Items in nested objects are flattenned with `k1.k2. ... .kN` key until its' length exceeds maxFieldNameLen.
//
// The p.Fields remains valid until the next call to ParseLogMessage() or PutJSONParser().
func (p *JSONParser) parseLogMessage(msg []byte, maxFieldNameLen int) error {
	p.reset()

	msgStr := bytesutil.ToUnsafeString(msg)
	v, err := p.p.Parse(msgStr)
	if err != nil {
		return err
	}
	o, err := v.Object()
	if err != nil {
		return err
	}
	p.Fields, p.buf, p.prefixBuf = appendLogFields(p.Fields, p.buf, p.prefixBuf, o, maxFieldNameLen)
	return nil
}

func appendLogFields(dst []Field, dstBuf, prefixBuf []byte, o *fastjson.Object, maxFieldNameLen int) ([]Field, []byte, []byte) {
	maxKeyLen := 0
	o.Visit(func(k []byte, _ *fastjson.Value) {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	})

	prefixLen := len(prefixBuf)
	if prefixLen+maxKeyLen > maxFieldNameLen {
		// Too long composite key. Convert o to string representation

		if len(prefixBuf) > 0 && prefixBuf[len(prefixBuf)-1] == '.' {
			// Drop trailing dot if needed
			prefixBuf = prefixBuf[:len(prefixBuf)-1]
		}

		dstBufLen := len(dstBuf)
		dstBuf = o.MarshalTo(dstBuf)
		value := dstBuf[dstBufLen:]
		dst, dstBuf = appendLogField(dst, dstBuf, prefixBuf, nil, value)
		return dst, dstBuf, prefixBuf[:prefixLen]
	}

	// Flatten JSON object o.
	// For example, {"foo":{"bar":"baz"}} is converted to {"foo.bar":"baz"}
	o.Visit(func(k []byte, v *fastjson.Value) {
		t := v.Type()
		switch t {
		case fastjson.TypeNull:
			// Skip nulls
		case fastjson.TypeObject:
			// Flatten nested JSON objects.
			o, err := v.Object()
			if err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}

			prefixBuf = append(prefixBuf, k...)
			prefixBuf = append(prefixBuf, '.')
			dst, dstBuf, prefixBuf = appendLogFields(dst, dstBuf, prefixBuf, o, maxFieldNameLen)
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

func appendLogField(dst []Field, dstBuf, prefixBuf, k, value []byte) ([]Field, []byte) {
	dstBufLen := len(dstBuf)
	dstBuf = append(dstBuf, prefixBuf...)
	dstBuf = append(dstBuf, k...)
	name := dstBuf[dstBufLen:]

	nameStr := bytesutil.ToUnsafeString(name)
	if nameStr == "" {
		nameStr = "_msg"
	}
	valueStr := bytesutil.ToUnsafeString(value)

	dst = append(dst, Field{
		Name:  nameStr,
		Value: valueStr,
	})
	return dst, dstBuf
}
