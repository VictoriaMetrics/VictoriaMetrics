package logstorage

import (
	"fmt"
	"testing"
)

func TestBloomFilter(t *testing.T) {
	f := func(tokens []string) {
		t.Helper()
		data := bloomFilterMarshal(nil, tokens)
		bf := getBloomFilter()
		defer putBloomFilter(bf)
		if err := bf.unmarshal(data); err != nil {
			t.Fatalf("unexpected error when unmarshaling bloom filter: %s", err)
		}
		for _, token := range tokens {
			if !bf.containsAny([]string{token}) {
				t.Fatalf("bloomFilterContains must return true for the added token %q", token)
			}
		}
		if !bf.containsAll(tokens) {
			t.Fatalf("bloomFilterContains must return true for the added tokens")
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
	data := bloomFilterMarshal(nil, tokens)
	bf := getBloomFilter()
	defer putBloomFilter(bf)
	if err := bf.unmarshal(data); err != nil {
		t.Fatalf("unexpected error when unmarshaling bloom filter: %s", err)
	}

	// count the number of false positives on 20K non-existing tokens
	falsePositives := 0
	for i := range tokens {
		token := fmt.Sprintf("non-existing-token_%d", i)
		if bf.containsAny([]string{token}) {
			falsePositives++
		}
	}
	p := float64(falsePositives) / float64(len(tokens))
	maxFalsePositive := 0.0011
	if p > maxFalsePositive {
		t.Fatalf("too high false positive rate; got %.4f; want %.4f max", p, maxFalsePositive)
	}
}
