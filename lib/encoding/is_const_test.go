package encoding

import (
	"math/rand"
	"testing"
)

func TestIsConst(t *testing.T) {
	f := func(a []int64, okExpected bool) {
		t.Helper()
		ok := IsConst(a)
		if ok != okExpected {
			t.Fatalf("unexpected isConst for a=%d; got %v; want %v", a, ok, okExpected)
		}
	}
	f([]int64{}, false)
	f([]int64{1}, true)
	f([]int64{1, 2}, false)
	f([]int64{1, 1}, true)
	f([]int64{1, 1, 1}, true)
	f([]int64{1, 1, 2}, false)
	//
	arr1 := getData(1024)
	f(arr1, true)
	arr1[len(arr1)-1] = -1
	f(arr1, false)
	arr2 := getData(1024 + 3)
	f(arr2, true)
	arr2[len(arr2)-3] = -2
	f(arr2, false)
}

func getData(cnt int) []int64 {
	arr := make([]int64, cnt)
	seed := rand.Int63n(0x7f7f7f7f7f7f7f7f)
	for i := 0; i < cnt; i++ {
		arr[i] = seed
	}
	return arr
}
