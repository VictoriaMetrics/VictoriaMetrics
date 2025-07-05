package logstorage

import (
	"sync"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// tokenizeHashes extracts word tokens from a, hashes them, appends hashes to dst and returns the result.
//
// The returned hashes must be passed to bloomFilterMarshalHashes in order to build bloom filters.
// The returned hashes must be passed to appendHashesHashes before being passed to bloomFilter.containsAll.
func tokenizeHashes(dst []uint64, a []string) []uint64 {
	t := getHashTokenizer()
	for i, s := range a {
		if i > 0 && s == a[i-1] {
			// This string has been already tokenized
			continue
		}
		dst = t.tokenizeString(dst, s)
	}
	putHashTokenizer(t)

	return dst
}

const hashTokenizerBucketsCount = 1024

type hashTokenizer struct {
	buckets [hashTokenizerBucketsCount]hashTokenizerBucket
	bm      bitmap
}

type hashTokenizerBucket struct {
	v        uint64
	overflow []uint64
}

func (b *hashTokenizerBucket) reset() {
	// do not spend CPU time on clearing v and b.overflow items,
	// since they'll be overwritten with new items.
	b.overflow = b.overflow[:0]
}

func newHashTokenizer() *hashTokenizer {
	var t hashTokenizer
	t.bm.init(len(t.buckets))
	return &t
}

func (t *hashTokenizer) reset() {
	if t.bm.onesCount() <= len(t.buckets)/4 {
		t.bm.forEachSetBit(func(idx int) bool {
			t.buckets[idx].reset()
			return false
		})
	} else {
		buckets := t.buckets[:]
		for i := range buckets {
			buckets[i].reset()
		}
		t.bm.init(len(t.buckets))
	}
}

func (t *hashTokenizer) tokenizeStringUnicode(dst []uint64, s string) []uint64 {
	for len(s) > 0 {
		// Search for the next token.
		n := len(s)
		for offset, r := range s {
			if isTokenRune(r) {
				n = offset
				break
			}
		}
		s = s[n:]
		// Search for the end of the token.
		n = len(s)
		for offset, r := range s {
			if !isTokenRune(r) {
				n = offset
				break
			}
		}
		if n == 0 {
			break
		}

		// Register the token
		token := s[:n]
		s = s[n:]
		if h, ok := t.addToken(token); ok {
			dst = append(dst, h)
		}
	}
	return dst
}

func (t *hashTokenizer) addToken(token string) (uint64, bool) {
	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(token))
	idx := int(h % uint64(len(t.buckets)))

	b := &t.buckets[idx]
	if !t.bm.isSetBit(idx) {
		b.v = h
		t.bm.setBit(idx)
		return h, true
	}

	if b.v == h {
		return h, false
	}
	for _, v := range b.overflow {
		if v == h {
			return h, false
		}
	}
	b.overflow = append(b.overflow, h)
	return h, true
}

func getHashTokenizer() *hashTokenizer {
	v := hashTokenizerPool.Get()
	if v == nil {
		return newHashTokenizer()
	}
	return v.(*hashTokenizer)
}

func putHashTokenizer(t *hashTokenizer) {
	t.reset()
	hashTokenizerPool.Put(t)
}

var hashTokenizerPool sync.Pool

func initAsciiTable() (table [256]byte) {
	for i := '0'; i <= '9'; i++ {
		table[i] = 1
	}
	for i := 'a'; i <= 'z'; i++ {
		table[i] = 1
	}
	for i := 'A'; i <= 'Z'; i++ {
		table[i] = 1
	}
	table['_'] = 1
	return
}

func initUnicodeTable() (table [256]byte) {
	for i := '0'; i <= '9'; i++ {
		table[i] = 1
	}
	for i := 'a'; i <= 'z'; i++ {
		table[i] = 1
	}
	for i := 'A'; i <= 'Z'; i++ {
		table[i] = 1
	}
	table['_'] = 1
	for i := 128; i <= 255; i++ {
		table[i] = 1
	}
	return
}

var lookupTables [2][256]byte = func() [2][256]byte {
	return [2][256]byte{
		initAsciiTable(),
		initUnicodeTable(),
	}
}()

func (t *hashTokenizer) tokenizeString(dst []uint64, s string) []uint64 {
	i := 0
	ptr := unsafe.Pointer(unsafe.StringData(s))
	var curUnicodeFlag byte
	for i < len(s) {
		curUnicodeFlag = 0
		// Search for the next token.
		start := len(s)
		for i < len(s) {
			c := *(*byte)(unsafe.Add(ptr, uintptr(i)))
			unicodeFlag := c & 0x80
			unicodeFlag = (unicodeFlag | (-unicodeFlag)) >> 7
			curUnicodeFlag |= unicodeFlag
			flag := lookupTables[unicodeFlag][c]
			if flag == 0 {
				i++
				continue
			}
			start = i
			i++
			break
		}
		// Search for the end of the token.
		end := len(s)
		for i < len(s) {
			c := *(*byte)(unsafe.Add(ptr, uintptr(i)))
			flag := lookupTables[curUnicodeFlag][c]
			if flag != 0 {
				i++
				continue
			}
			end = i
			i++
			break
		}
		if end <= start {
			break
		}

		// Register the token.
		token := unsafe.String((*byte)(unsafe.Add(ptr, start)), end-start)
		if curUnicodeFlag == 1 {
			dst = t.tokenizeStringUnicode(dst, token)
			continue
		}
		if h, ok := t.addToken(token); ok {
			dst = append(dst, h)
		}
	}
	return dst
}
