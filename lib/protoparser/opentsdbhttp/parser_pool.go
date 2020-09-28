package opentsdbhttp

import (
	"github.com/valyala/fastjson"
)

// getJSONParser returns JSON parser.
//
// The parser must be returned to the pool via putJSONParser when no longer needed.
func getJSONParser() *fastjson.Parser {
	return parserPool.Get()
}

// putJSONParser returns p to the pool.
//
// p cannot be used after returning to the pool.
func putJSONParser(p *fastjson.Parser) {
	parserPool.Put(p)
}

var parserPool fastjson.ParserPool
