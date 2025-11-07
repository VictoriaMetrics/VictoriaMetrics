package logstorage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
)

// StreamFilter is a filter for streams, e.g. `_stream:{...}`
type StreamFilter struct {
	orFilters []*andStreamFilter
}

func (sf *StreamFilter) matchStreamName(s string) bool {
	sn := getStreamName()
	defer putStreamName(sn)

	if !sn.parse(s) {
		return false
	}

	for _, of := range sf.orFilters {
		matchAndFilters := true
		for _, tf := range of.tagFilters {
			if !sn.match(tf) {
				matchAndFilters = false
				break
			}
		}
		if matchAndFilters {
			return true
		}
	}
	return false
}

func (sf *StreamFilter) isEmpty() bool {
	for _, af := range sf.orFilters {
		if len(af.tagFilters) > 0 {
			return false
		}
	}
	return true
}

func (sf *StreamFilter) marshalForCacheKey(dst []byte) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(sf.orFilters)))
	for _, af := range sf.orFilters {
		dst = encoding.MarshalVarUint64(dst, uint64(len(af.tagFilters)))
		for _, f := range af.tagFilters {
			dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.tagName))
			dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.op))
			dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(f.value))
		}
	}
	return dst
}

func (sf *StreamFilter) String() string {
	a := make([]string, len(sf.orFilters))
	for i := range a {
		a[i] = sf.orFilters[i].String()
	}
	return "{" + strings.Join(a, " or ") + "}"
}

type andStreamFilter struct {
	tagFilters []*streamTagFilter
}

func (af *andStreamFilter) String() string {
	a := make([]string, len(af.tagFilters))
	for i := range a {
		a[i] = af.tagFilters[i].String()
	}
	return strings.Join(a, ",")
}

// streamTagFilter is a filter for `tagName op value`
type streamTagFilter struct {
	// tagName is the name for the tag to filter
	tagName string

	// op is operation such as `=`, `!=`, `=~`, `!~` or `:`
	op string

	// value is the value
	value string

	// regexp is initialized for `=~` and `!~` op.
	regexp *regexutil.PromRegex
}

func (tf *streamTagFilter) String() string {
	return quoteTokenIfNeeded(tf.tagName) + tf.op + strconv.Quote(tf.value)
}

func parseStreamFilter(lex *lexer) (*StreamFilter, error) {
	if !lex.isKeyword("{") {
		return nil, fmt.Errorf("unexpected token %q instead of '{' in _stream filter", lex.token)
	}
	lex.nextToken()
	var filters []*andStreamFilter
	for {
		f, err := parseAndStreamFilter(lex)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
		switch {
		case lex.isKeyword("}"):
			lex.nextToken()
			sf := &StreamFilter{
				orFilters: filters,
			}
			return sf, nil
		case lex.isKeyword("or"):
			lex.nextToken()
			if lex.isKeyword("}") {
				return nil, fmt.Errorf("unexpected '}' after 'or' in _stream filter")
			}
		default:
			return nil, fmt.Errorf("unexpected token in _stream filter: %q; want '}' or 'or'", lex.token)
		}
	}
}

func parseAndStreamFilter(lex *lexer) (*andStreamFilter, error) {
	var filters []*streamTagFilter
	for {
		if lex.isKeyword("}") {
			asf := &andStreamFilter{
				tagFilters: filters,
			}
			return asf, nil
		}
		f, err := parseStreamTagFilter(lex)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
		switch {
		case lex.isKeyword("or", "}"):
			asf := &andStreamFilter{
				tagFilters: filters,
			}
			return asf, nil
		case lex.isKeyword(","):
			lex.nextToken()
		default:
			return nil, fmt.Errorf("unexpected token %q in _stream filter after %q; want 'or', 'and', '}' or ','", lex.token, f)
		}
	}
}

func parseStreamTagFilter(lex *lexer) (*streamTagFilter, error) {
	// parse tagName
	tagName, err := lex.nextCompoundToken()
	if err != nil {
		return nil, fmt.Errorf("cannot parse stream tag name inside {...}: %w", err)
	}
	if !lex.isKeyword("=", "!=", "=~", "!~", "in", "not_in") {
		return nil, fmt.Errorf("unsupported operation %q inside {...} for %q field; supported operations: =, !=, =~, !~, in, not_in", lex.token, tagName)
	}

	// parse op
	op := lex.token
	lex.nextToken()

	// parse tag value
	value := ""
	if op == "in" || op == "not_in" {
		args, isWildcard, err := parseArgsInParensPossibleWildcard(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s() args inside {...}: %w", op, err)
		}
		if op == "in" {
			op = "=~"
		} else {
			op = "!~"
		}
		if isWildcard {
			value = ".*"
		} else {
			argsEscaped := make([]string, len(args))
			for i := range args {
				argsEscaped[i] = regexp.QuoteMeta(args[i])
			}
			value = strings.Join(argsEscaped, "|")
		}
	} else {
		v, err := lex.nextCompoundToken()
		if err != nil {
			return nil, fmt.Errorf("cannot parse value for tag %q inside {...}: %w", tagName, err)
		}
		value = v
	}

	stf := &streamTagFilter{
		tagName: tagName,
		op:      op,
		value:   value,
	}
	if op == "=~" || op == "!~" {
		re, err := regexutil.NewPromRegex(value)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp %q for %q inside {...}: %w", value, tagName, err)
		}
		stf.regexp = re
	}
	return stf, nil
}

func getStreamName() *streamName {
	v := streamNamePool.Get()
	if v == nil {
		return &streamName{}
	}
	return v.(*streamName)
}

func putStreamName(sn *streamName) {
	sn.reset()
	streamNamePool.Put(sn)
}

var streamNamePool sync.Pool

type streamName struct {
	tags []Field
}

func (sn *streamName) reset() {
	clear(sn.tags)
	sn.tags = sn.tags[:0]
}

func (sn *streamName) parse(s string) bool {
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return false
	}
	s = s[1 : len(s)-1]
	if len(s) == 0 {
		return true
	}

	for {
		// Parse tag name
		n := strings.IndexByte(s, '=')
		if n < 0 {
			// cannot find tag name
			return false
		}
		name := s[:n]
		s = s[n+1:]

		// Parse tag value
		if len(s) == 0 || s[0] != '"' {
			return false
		}
		qPrefix, err := strconv.QuotedPrefix(s)
		if err != nil {
			return false
		}
		s = s[len(qPrefix):]
		value, err := strconv.Unquote(qPrefix)
		if err != nil {
			return false
		}

		sn.tags = append(sn.tags, Field{
			Name:  name,
			Value: value,
		})

		if len(s) == 0 {
			return true
		}
		if s[0] != ',' {
			return false
		}
		s = s[1:]
	}
}

func (sn *streamName) match(tf *streamTagFilter) bool {
	v := sn.getTagValueByTagName(tf.tagName)
	switch tf.op {
	case "=":
		return v == tf.value
	case "!=":
		return v != tf.value
	case "=~":
		return tf.regexp.MatchString(v)
	case "!~":
		return !tf.regexp.MatchString(v)
	default:
		logger.Panicf("BUG: unexpected tagFilter operation: %q", tf.op)
		return false
	}
}

func (sn *streamName) getTagValueByTagName(name string) string {
	for _, t := range sn.tags {
		if t.Name == name {
			return t.Value
		}
	}
	return ""
}
