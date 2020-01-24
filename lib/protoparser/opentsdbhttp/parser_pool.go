package opentsdbhttp

import (
	"github.com/valyala/fastjson"
)

// GetParser returns JSON parser.
//
// The parser must be returned to the pool via PutParser when no longer needed.
func GetParser() *fastjson.Parser {
	return parserPool.Get()
}

// PutParser returns p to the pool.
//
// p cannot be used after returning to the pool.
func PutParser(p *fastjson.Parser) {
	parserPool.Put(p)
}

var parserPool fastjson.ParserPool
