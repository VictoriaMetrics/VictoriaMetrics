package logstorage

import (
	"strings"
	"sync"
)

type logfmtParser struct {
	fields []Field
}

func (p *logfmtParser) reset() {
	clear(p.fields)
	p.fields = p.fields[:0]
}

func (p *logfmtParser) addField(name, value string) {
	name = strings.TrimSpace(name)
	if name == "" && value == "" {
		return
	}
	p.fields = append(p.fields, Field{
		Name:  name,
		Value: value,
	})
}

func (p *logfmtParser) parse(s string) {
	p.reset()
	for {
		// Search for field name
		n := strings.IndexAny(s, "= ")
		if n < 0 {
			// empty value
			p.addField(s, "")
			return
		}

		name := s[:n]
		ch := s[n]
		s = s[n+1:]
		if ch == ' ' {
			// empty value
			p.addField(name, "")
			continue
		}
		if len(s) == 0 {
			p.addField(name, "")
			return
		}

		// Search for field value
		value, nOffset := tryUnquoteString(s, "")
		if nOffset >= 0 {
			p.addField(name, value)
			s = s[nOffset:]
			if len(s) == 0 {
				return
			}
			if s[0] != ' ' {
				return
			}
			s = s[1:]
		} else {
			n := strings.IndexByte(s, ' ')
			if n < 0 {
				p.addField(name, s)
				return
			}
			p.addField(name, s[:n])
			s = s[n+1:]
		}
	}
}

func getLogfmtParser() *logfmtParser {
	v := logfmtParserPool.Get()
	if v == nil {
		return &logfmtParser{}
	}
	return v.(*logfmtParser)
}

func putLogfmtParser(p *logfmtParser) {
	p.reset()
	logfmtParserPool.Put(p)
}

var logfmtParserPool sync.Pool
