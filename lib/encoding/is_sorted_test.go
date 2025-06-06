package encoding

import (
	"testing"
)

func TestIsInt64ArraySorted(t *testing.T) {
	f := func(a []int64, expect bool) {
		if IsInt64ArraySorted(a) != expect {
			t.Errorf("%+v should be %v", a, expect)
		}
	}
	f([]int64{1, 2, 3, 4, 6, 7, 8, 9, 9}, true)
	f([]int64{}, true)
	f([]int64{1}, true)
	f([]int64{1, 2}, true)
	f([]int64{1, 3}, true)
	f([]int64{1, 2, 3, 4}, true)
	f([]int64{1, 2, 3, 4, 5}, true)
	f([]int64{1, 2, 3, 4, 6}, true)
	f([]int64{1, 2, 3, 4, 6, 7, 8}, true)
	f([]int64{1, 2, 3, 4, 6, 7, 8, 9}, true)

	f([]int64{1, 2, 3, 4, 6, 7, 8, 9, 9, 8}, false)
	f([]int64{-4, -3, -2, -1, 0, 1, 2, 3, 4, 6, 7, 8, 9, 9}, true)
	f([]int64{-4, -3, -2, -1, 0, 1, 0, 2, 3, 4, 6, 7, 8, 9, 9}, false)
}
