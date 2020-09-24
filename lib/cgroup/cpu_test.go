package cgroup

import (
	"testing"
)

func TestCountCPUs(t *testing.T) {
	f := func(s string, nExpected int) {
		t.Helper()
		n := countCPUs(s)
		if n != nExpected {
			t.Fatalf("unexpected result from countCPUs(%q); got %d; want %d", s, n, nExpected)
		}
	}
	f("", -1)
	f("1", 1)
	f("234", 1)
	f("1,2", 2)
	f("0-1", 2)
	f("0-0", 1)
	f("1-2,3,5-9,200-210", 19)
	f("0-3", 4)
	f("0-6", 7)
}
