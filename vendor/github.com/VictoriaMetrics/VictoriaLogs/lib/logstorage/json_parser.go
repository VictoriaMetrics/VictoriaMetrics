package logstorage

import (
	"slices"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/valyala/fastjson"
)

// JSONParser parses a single JSON log message into Fields.
//
// See https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model
//
// Use GetJSONParser() for obtaining the parser.
type JSONParser struct {
	// Fields contains the parsed JSON line after Parse() call
	//
	// The Fields are valid until the next call to ParseLogMessage()
	// or until the parser is returned to the pool with PutJSONParser() call.
	Fields []Field

	// p is used for fast JSON parsing
	p fastjson.Parser

	// buf is used for holding the backing data for Fields
	buf []byte

	// prefixBuf is used for holding the current key prefix
	// when it is composed from multiple keys.
	prefixBuf []byte

	preserveKeys    []string
	maxFieldNameLen int
}

func (p *JSONParser) reset() {
	clear(p.Fields)
	p.Fields = p.Fields[:0]

	p.buf = p.buf[:0]

	p.prefixBuf = p.prefixBuf[:0]
	p.preserveKeys = nil
	p.maxFieldNameLen = 0
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
// JSON values for keys from the preserveKeys list are preserved without flattening.
//
// The p.Fields remains valid until the next call to ParseLogMessage() or PutJSONParser().
func (p *JSONParser) ParseLogMessage(msg []byte, preserveKeys []string) error {
	return p.parseLogMessage(msg, preserveKeys, maxFieldNameSize)
}

// parseLogMessage parses the given JSON log message msg into p.Fields.
//
// Items in nested objects are flattened with `k1.k2. ... .kN` key until the key matches one of the preserveKeys
// or its length exceeds maxFieldNameLen.
//
// The p.Fields remains valid until the next call to ParseLogMessage() or PutJSONParser().
func (p *JSONParser) parseLogMessage(msg []byte, preserveKeys []string, maxFieldNameLen int) error {
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
	p.maxFieldNameLen = maxFieldNameLen
	p.preserveKeys = preserveKeys
	p.appendLogFields(o)
	return nil
}

func (p *JSONParser) appendLogFields(o *fastjson.Object) {
	if p.isTooLongKey(o) || p.shouldPreserveKeyPrefix() {
		p.appendPreservedLogField(o)
		return
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

			prefixLen := len(p.prefixBuf)
			p.prefixBuf = append(p.prefixBuf, k...)
			p.prefixBuf = append(p.prefixBuf, '.')
			p.appendLogFields(o)
			p.prefixBuf = p.prefixBuf[:prefixLen]
		case fastjson.TypeArray, fastjson.TypeNumber, fastjson.TypeTrue, fastjson.TypeFalse:
			// Convert JSON arrays, numbers, true and false values to their string representation
			bufLen := len(p.buf)
			p.buf = v.MarshalTo(p.buf)
			value := p.buf[bufLen:]
			p.appendLogField(k, value)
		case fastjson.TypeString:
			// Decode JSON strings
			bufLen := len(p.buf)
			p.buf = append(p.buf, v.GetStringBytes()...)
			value := p.buf[bufLen:]
			p.appendLogField(k, value)
		default:
			logger.Panicf("BUG: unexpected JSON type: %s", t)
		}
	})
}

func (p *JSONParser) isTooLongKey(o *fastjson.Object) bool {
	maxKeyLen := 0
	o.Visit(func(k []byte, _ *fastjson.Value) {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	})
	return len(p.prefixBuf)+maxKeyLen > p.maxFieldNameLen
}

func (p *JSONParser) shouldPreserveKeyPrefix() bool {
	if len(p.prefixBuf) == 0 {
		return false
	}

	key := bytesutil.ToUnsafeString(p.prefixBuf)

	// Drop trailing dot
	key = key[:len(key)-1]

	return slices.Contains(p.preserveKeys, key)
}

func (p *JSONParser) appendPreservedLogField(o *fastjson.Object) {
	prefixLen := len(p.prefixBuf)
	if prefixLen > 0 {
		// Drop trailing dot
		p.prefixBuf = p.prefixBuf[:prefixLen-1]
	}

	bufLen := len(p.buf)
	p.buf = o.MarshalTo(p.buf)
	value := p.buf[bufLen:]
	p.appendLogField(nil, value)
	p.prefixBuf = p.prefixBuf[:prefixLen]
}

func (p *JSONParser) appendLogField(k, value []byte) {
	bufLen := len(p.buf)
	p.buf = append(p.buf, p.prefixBuf...)
	p.buf = append(p.buf, k...)
	name := p.buf[bufLen:]

	nameStr := bytesutil.ToUnsafeString(name)
	if nameStr == "" {
		nameStr = "_msg"
	}
	valueStr := bytesutil.ToUnsafeString(value)

	p.Fields = append(p.Fields, Field{
		Name:  nameStr,
		Value: valueStr,
	})
}
