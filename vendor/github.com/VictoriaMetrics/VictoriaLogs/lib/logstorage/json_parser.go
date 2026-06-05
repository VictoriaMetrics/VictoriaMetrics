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
// Use ParseLogMessage() for parsing the JSON log message.
//
// Use GetJSONParser() for obtaining the parser.
type JSONParser struct {
	commonJSON

	// p is used for fast JSON parsing
	p fastjson.Parser
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
// The given fieldPrefix is added to all the parsed field names.
//
// The p.Fields remains valid until the next call to ParseLogMessage() or PutJSONParser().
func (p *JSONParser) ParseLogMessage(msg []byte, preserveKeys []string, fieldPrefix string) error {
	return p.parseLogMessage(msg, preserveKeys, fieldPrefix, maxFieldNameSize)
}

// parseLogMessage parses the given JSON log message msg into p.Fields.
//
// Items in nested objects are flattened with `k1.k2. ... .kN` key until the key matches one of the preserveKeys
// or its length exceeds maxFieldNameLen.
//
// The p.Fields remains valid until the next call to ParseLogMessage() or PutJSONParser().
func (p *JSONParser) parseLogMessage(msg []byte, preserveKeys []string, fieldPrefix string, maxFieldNameLen int) error {
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
	p.init(preserveKeys, fieldPrefix, maxFieldNameLen)
	p.appendLogFields(o)
	return nil
}

type commonJSON struct {
	// Fields contains the parsed JSON line after appendLogFields() call.
	Fields []Field

	// buf is used for holding the backing data for Fields
	buf []byte

	// prefixBuf is used for holding the current key prefix when it is composed from multiple keys.
	prefixBuf []byte

	preserveKeys    []string
	fieldPrefix     string
	maxFieldNameLen int
}

func (c *commonJSON) reset() {
	c.resetKeepSettings()
	c.preserveKeys = nil
	c.fieldPrefix = ""
	c.maxFieldNameLen = 0
}

func (c *commonJSON) init(preserveKeys []string, fieldPrefix string, maxFieldNameLen int) {
	c.preserveKeys = preserveKeys
	c.fieldPrefix = fieldPrefix
	c.maxFieldNameLen = maxFieldNameLen
}

func (c *commonJSON) resetKeepSettings() {
	clear(c.Fields)
	c.Fields = c.Fields[:0]

	c.buf = c.buf[:0]

	c.prefixBuf = c.prefixBuf[:0]
}

func (c *commonJSON) appendLogFields(o *fastjson.Object) {
	if c.isTooLongKey(o) || c.shouldPreserveKeyPrefix() {
		c.appendPreservedLogField(o)
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

			prefixLen := len(c.prefixBuf)
			c.prefixBuf = append(c.prefixBuf, k...)
			c.prefixBuf = append(c.prefixBuf, '.')
			c.appendLogFields(o)
			c.prefixBuf = c.prefixBuf[:prefixLen]
		case fastjson.TypeArray, fastjson.TypeNumber, fastjson.TypeTrue, fastjson.TypeFalse:
			// Convert JSON arrays, numbers, true and false values to their string representation
			bufLen := len(c.buf)
			c.buf = v.MarshalTo(c.buf)
			value := c.buf[bufLen:]
			c.appendLogField(k, value)
		case fastjson.TypeString:
			// Decode JSON strings
			bufLen := len(c.buf)
			c.buf = append(c.buf, v.GetStringBytes()...)
			value := c.buf[bufLen:]
			c.appendLogField(k, value)
		default:
			logger.Panicf("BUG: unexpected JSON type: %s", t)
		}
	})
}

func (c *commonJSON) isTooLongKey(o *fastjson.Object) bool {
	maxKeyLen := 0
	o.Visit(func(k []byte, _ *fastjson.Value) {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	})
	return len(c.prefixBuf)+maxKeyLen > c.maxFieldNameLen
}

func (c *commonJSON) shouldPreserveKeyPrefix() bool {
	if len(c.prefixBuf) == 0 {
		return false
	}

	key := bytesutil.ToUnsafeString(c.prefixBuf)

	// Drop trailing dot
	key = key[:len(key)-1]

	return slices.Contains(c.preserveKeys, key)
}

func (c *commonJSON) appendPreservedLogField(o *fastjson.Object) {
	prefixLen := len(c.prefixBuf)
	if prefixLen > 0 {
		// Drop trailing dot
		c.prefixBuf = c.prefixBuf[:prefixLen-1]
	}

	bufLen := len(c.buf)
	c.buf = o.MarshalTo(c.buf)
	value := c.buf[bufLen:]
	c.appendLogField(nil, value)
	c.prefixBuf = c.prefixBuf[:prefixLen]
}

func (c *commonJSON) appendLogField(k, value []byte) {
	bufLen := len(c.buf)
	c.buf = append(c.buf, c.fieldPrefix...)
	c.buf = append(c.buf, c.prefixBuf...)
	c.buf = append(c.buf, k...)
	name := c.buf[bufLen:]

	nameStr := bytesutil.ToUnsafeString(name)
	if nameStr == "" {
		nameStr = "_msg"
	}
	valueStr := bytesutil.ToUnsafeString(value)

	c.Fields = append(c.Fields, Field{
		Name:  nameStr,
		Value: valueStr,
	})
}
