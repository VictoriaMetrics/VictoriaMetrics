package logstorage

import (
	"reflect"
	"testing"
)

func TestTokenizeHashes(t *testing.T) {
	f := func(a []string, hashesExpected []uint64) {
		t.Helper()
		hashes := tokenizeHashes(nil, a)
		if !reflect.DeepEqual(hashes, hashesExpected) {
			t.Fatalf("unexpected hashes\ngot\n%X\nwant\n%X", hashes, hashesExpected)
		}
	}

	f(nil, nil)
	f([]string{""}, nil)
	f([]string{"foo"}, []uint64{0x33BF00A859C4BA3F})
	f([]string{"foo foo", "!!foo //"}, []uint64{0x33BF00A859C4BA3F})
	f([]string{"foo bar---.!!([baz]!!! %$# TaSte"}, []uint64{0x33BF00A859C4BA3F, 0x48A37C90AD27A659, 0x42598CF26A247404, 0x34709F40A3286E46})
	f([]string{"foo bar---.!!([baz]!!! %$# baz foo TaSte"}, []uint64{0x33BF00A859C4BA3F, 0x48A37C90AD27A659, 0x42598CF26A247404, 0x34709F40A3286E46})
	f([]string{"теСТ 1234 f12.34", "34 f12 AS"}, []uint64{0xFE846FA145CEABD1, 0xD8316E61D84F6BA4, 0x6D67BA71C4E03D10, 0x5E8D522CA93563ED, 0xED80AED10E029FC8})
}
