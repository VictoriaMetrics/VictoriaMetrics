package logstorage

import (
	"fmt"
	"testing"
)

func TestBloomFilter(t *testing.T) {
	f := func(tokens []string) {
		t.Helper()
		dataTokens := bloomFilterMarshalTokens(nil, tokens)
		hashes := tokenizeHashes(nil, tokens)
		dataHashes := bloomFilterMarshalHashes(nil, hashes)
		if string(dataTokens) != string(dataHashes) {
			t.Fatalf("unexpected marshaled bloom filters from hashes\ngot\n%X\nwant\n%X", dataHashes, dataTokens)
		}

		bf := getBloomFilter()
		defer putBloomFilter(bf)
		if err := bf.unmarshal(dataTokens); err != nil {
			t.Fatalf("unexpected error when unmarshaling bloom filter: %s", err)
		}
		tokensHashes := appendTokensHashes(nil, tokens)
		if !bf.containsAll(tokensHashes) {
			t.Fatalf("containsAll must return true for the added tokens")
		}
	}
	f(nil)
	f([]string{"foo"})
	f([]string{"foo", "bar", "baz"})

	// 10k tokens
	tokens := make([]string, 10000)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("token_%d", i)
	}
	f(tokens)
}

func TestBloomFilterUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()
		bf := getBloomFilter()
		defer putBloomFilter(bf)
		if err := bf.unmarshal(data); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}
	f([]byte("a"))
	f([]byte("foo"))
}

func TestBloomFilterUnmarshalGarbage(t *testing.T) {
	data := []byte("01234567")
	var bf bloomFilter
	if err := bf.unmarshal(data); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestBloomFilterFalsePositive(t *testing.T) {
	tokens := make([]string, 20000)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("token_%d", i)
	}
	data := bloomFilterMarshalTokens(nil, tokens)
	bf := getBloomFilter()
	defer putBloomFilter(bf)
	if err := bf.unmarshal(data); err != nil {
		t.Fatalf("unexpected error when unmarshaling bloom filter: %s", err)
	}

	// count the number of false positives on 20K non-existing tokens
	falsePositives := 0
	for i := range tokens {
		token := fmt.Sprintf("non-existing-token_%d", i)
		tokensHashes := appendTokensHashes(nil, []string{token})
		if bf.containsAll(tokensHashes) {
			falsePositives++
		}
	}
	p := float64(falsePositives) / float64(len(tokens))
	maxFalsePositive := 0.0011
	if p > maxFalsePositive {
		t.Fatalf("too high false positive rate; got %.4f; want %.4f max", p, maxFalsePositive)
	}
}

func TestBloomFilterMarshal_TokensVSHashes(t *testing.T) {
	tokens := make([]string, 100)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("token_%d", i)
	}

	dataTokens := bloomFilterMarshalTokens(nil, tokens)

	hashes := tokenizeHashes(nil, tokens)
	dataHashes := bloomFilterMarshalHashes(nil, hashes)

	if string(dataTokens) != string(dataHashes) {
		t.Fatalf("unexpected bloom filter obtained from hashes\ngot\n%X\nwant\n%X", dataHashes, dataTokens)
	}
}

func TestBloomFilterMarshalTokens(t *testing.T) {
	f := func(tokens []string, resultExpected string) {
		t.Helper()

		result := bloomFilterMarshalTokens(nil, tokens)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result\ngot\n%X\nwant\n%X", result, resultExpected)
		}
	}

	f([]string{}, "")
	f([]string{"foo"}, "\x00\x00\x00\x82\x40\x18\x00\x04")
	f([]string{"foo", "bar", "baz"}, "\x00\x00\x81\xA3\x48\x5C\x10\x26")
	f([]string{"foo", "bar", "baz", "foo"}, "\x00\x00\x81\xA3\x48\x5C\x10\x26")
}
