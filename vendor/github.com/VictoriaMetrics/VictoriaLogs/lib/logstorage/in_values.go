package logstorage

import (
	"slices"
	"strings"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// inValues keeps values for in(...), contains_any(...) and contains_all(...) filters
type inValues struct {
	values []string

	// If q is non-nil, then values must be populated from q before filter execution.
	q *Query

	// qFieldName must be set to field name for obtaining values from if q is non-nil.
	qFieldName string

	tokensHashesAnyOnce   sync.Once
	commonTokensHashesAny []uint64
	tokenSetsHashesAny    [][]uint64

	tokensHashesAllOnce sync.Once
	tokensHashesAll     []uint64

	stringValuesOnce sync.Once
	stringValues     map[string]struct{}

	uint8ValuesOnce sync.Once
	uint8Values     map[string]struct{}

	uint16ValuesOnce sync.Once
	uint16Values     map[string]struct{}

	uint32ValuesOnce sync.Once
	uint32Values     map[string]struct{}

	uint64ValuesOnce sync.Once
	uint64Values     map[string]struct{}

	int64ValuesOnce sync.Once
	int64Values     map[string]struct{}

	float64ValuesOnce sync.Once
	float64Values     map[string]struct{}

	ipv4ValuesOnce sync.Once
	ipv4Values     map[string]struct{}

	timestampISO8601ValuesOnce sync.Once
	timestampISO8601Values     map[string]struct{}
}

func (iv *inValues) String() string {
	if iv.q != nil {
		return iv.q.String()
	}
	values := iv.values
	a := make([]string, len(values))
	for i, value := range values {
		a[i] = quoteTokenIfNeeded(value)
	}
	return strings.Join(a, ",")
}

func (iv *inValues) isEmpty() bool {
	return len(iv.values) == 0
}

func (iv *inValues) hasEmptyValue() bool {
	m := iv.getStringValues()
	_, ok := m[""]
	return ok
}

func (iv *inValues) getNonEmptyValuesLen() int {
	m := iv.getStringValues()
	n := len(m)
	_, ok := m[""]
	if ok {
		n--
	}
	return n
}

func (iv *inValues) isOnlyEmptyValue() bool {
	return len(iv.values) == 1 && iv.values[0] == ""
}

func (iv *inValues) getTokensHashesAll() []uint64 {
	iv.tokensHashesAllOnce.Do(iv.initTokensHashesAll)
	return iv.tokensHashesAll
}

func (iv *inValues) initTokensHashesAll() {
	tokens := tokenizeHashes(nil, iv.values)
	iv.tokensHashesAll = appendHashesHashes(nil, tokens)
}

func (iv *inValues) getTokensHashesAny() ([]uint64, [][]uint64) {
	iv.tokensHashesAnyOnce.Do(iv.initTokensHashesAny)
	return iv.commonTokensHashesAny, iv.tokenSetsHashesAny
}

func (iv *inValues) initTokensHashesAny() {
	commonTokens, tokenSets := getCommonTokensAndTokenSets(iv.values)

	iv.commonTokensHashesAny = appendTokensHashes(nil, commonTokens)

	var hashesBuf []uint64
	tokenSetsHashes := make([][]uint64, len(tokenSets))
	for i, tokens := range tokenSets {
		if hashesBuf == nil || len(hashesBuf) > 60_000/int(unsafe.Sizeof(hashesBuf[0])) {
			hashesBuf = make([]uint64, 0, 64*1024/int(unsafe.Sizeof(hashesBuf[0])))
		}
		hashesBufLen := len(hashesBuf)
		hashesBuf = appendTokensHashes(hashesBuf, tokens)
		tokenSetsHashes[i] = hashesBuf[hashesBufLen:]
	}
	iv.tokenSetsHashesAny = tokenSetsHashes
}

func (iv *inValues) getStringValues() map[string]struct{} {
	iv.stringValuesOnce.Do(iv.initStringValues)
	return iv.stringValues
}

func (iv *inValues) initStringValues() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	iv.stringValues = m
}

func (iv *inValues) getUint8Values() map[string]struct{} {
	iv.uint8ValuesOnce.Do(iv.initUint8Values)
	return iv.uint8Values
}

func (iv *inValues) initUint8Values() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*1)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<8) {
			continue
		}
		bufLen := len(buf)
		buf = append(buf, byte(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	iv.uint8Values = m
}

func (iv *inValues) getUint16Values() map[string]struct{} {
	iv.uint16ValuesOnce.Do(iv.initUint16Values)
	return iv.uint16Values
}

func (iv *inValues) initUint16Values() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*2)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<16) {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint16(buf, uint16(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	iv.uint16Values = m
}

func (iv *inValues) getUint32Values() map[string]struct{} {
	iv.uint32ValuesOnce.Do(iv.initUint32Values)
	return iv.uint32Values
}

func (iv *inValues) initUint32Values() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*4)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok || n >= (1<<32) {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint32(buf, uint32(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	iv.uint32Values = m
}

func (iv *inValues) getUint64Values() map[string]struct{} {
	iv.uint64ValuesOnce.Do(iv.initUint64Values)
	return iv.uint64Values
}

func (iv *inValues) getInt64Values() map[string]struct{} {
	iv.int64ValuesOnce.Do(iv.initInt64Values)
	return iv.int64Values
}

func (iv *inValues) initUint64Values() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		n, ok := tryParseUint64(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint64(buf, n)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	iv.uint64Values = m
}

func (iv *inValues) initInt64Values() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		n, ok := tryParseInt64(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalInt64(buf, n)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	iv.int64Values = m
}

func (iv *inValues) getFloat64Values() map[string]struct{} {
	iv.float64ValuesOnce.Do(iv.initFloat64Values)
	return iv.float64Values
}

func (iv *inValues) initFloat64Values() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		f, ok := tryParseFloat64Exact(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = marshalFloat64(buf, f)
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	iv.float64Values = m
}

func (iv *inValues) getIPv4Values() map[string]struct{} {
	iv.ipv4ValuesOnce.Do(iv.initIPv4Values)
	return iv.ipv4Values
}

func (iv *inValues) initIPv4Values() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*4)
	for _, v := range values {
		n, ok := tryParseIPv4(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint32(buf, uint32(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	iv.ipv4Values = m
}

func (iv *inValues) getTimestampISO8601Values() map[string]struct{} {
	iv.timestampISO8601ValuesOnce.Do(iv.initTimestampISO8601Values)
	return iv.timestampISO8601Values
}

func (iv *inValues) initTimestampISO8601Values() {
	values := iv.values
	m := make(map[string]struct{}, len(values))
	buf := make([]byte, 0, len(values)*8)
	for _, v := range values {
		n, ok := tryParseTimestampISO8601(v)
		if !ok {
			continue
		}
		bufLen := len(buf)
		buf = encoding.MarshalUint64(buf, uint64(n))
		s := bytesutil.ToUnsafeString(buf[bufLen:])
		m[s] = struct{}{}
	}
	iv.timestampISO8601Values = m
}

func getCommonTokensAndTokenSets(values []string) ([]string, [][]string) {
	var tokensBuf []string
	tokenSets := make([][]string, len(values))
	for i, v := range values {
		if tokensBuf == nil || len(tokensBuf) > 60_000/int(unsafe.Sizeof(tokensBuf[0])) {
			tokensBuf = make([]string, 0, 64*1024/int(unsafe.Sizeof(tokensBuf[0])))
		}
		tokensBufLen := len(tokensBuf)
		tokensBuf = tokenizeStrings(tokensBuf, []string{v})
		tokenSets[i] = tokensBuf[tokensBufLen:]
	}

	commonTokens := getCommonTokens(tokenSets)
	if len(commonTokens) == 0 {
		return nil, tokenSets
	}

	// remove commonTokens from tokenSets
	for i, tokens := range tokenSets {
		dstTokens := tokens[:0]
		for _, token := range tokens {
			if !slices.Contains(commonTokens, token) {
				dstTokens = append(dstTokens, token)
			}
		}
		tokenSets[i] = dstTokens
	}

	return commonTokens, tokenSets
}

// getCommonTokens returns common tokens seen at every set of tokens inside tokenSets.
//
// The returned common tokens preserve the original order seen in tokenSets.
func getCommonTokens(tokenSets [][]string) []string {
	if len(tokenSets) == 0 {
		return nil
	}

	commonTokens := append([]string{}, tokenSets[0]...)

	for _, tokens := range tokenSets[1:] {
		if len(commonTokens) == 0 {
			return nil
		}
		dst := commonTokens[:0]
		for _, token := range commonTokens {
			if slices.Contains(tokens, token) {
				dst = append(dst, token)
			}
		}
		commonTokens = dst
	}
	return commonTokens
}
