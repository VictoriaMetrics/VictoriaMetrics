package logstorage

import (
	"testing"
)

func TestHash128(t *testing.T) {
	f := func(data string, hashExpected u128) {
		t.Helper()
		h := hash128([]byte(data))
		if !h.equal(&hashExpected) {
			t.Fatalf("unexpected hash; got %s; want %s", &h, &hashExpected)
		}
	}
	f("", u128{
		hi: 17241709254077376921,
		lo: 13138662262368978769,
	})

	f("abc", u128{
		hi: 4952883123889572249,
		lo: 3255951525518405514,
	})
}
