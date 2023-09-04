package opentsdbhttp

import (
	"github.com/valyala/fastjson"
)

// GetJSONParser returns JSON parser.
//
// The parser must be returned to the pool via PutJSONParser when no longer needed.
func GetJSONParser() *fastjson.Parser {
	return parserPool.Get()
}

// PutJSONParser returns p to the pool.
//
// p cannot be used after returning to the pool.
func PutJSONParser(p *fastjson.Parser) {
	parserPool.Put(p)
}

var parserPool fastjson.ParserPool
