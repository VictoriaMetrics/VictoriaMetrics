package fastnum

import (
	"fmt"
	"testing"
)

func TestIsInt64Zeros(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000, 1e4, 1e5, 8*1024 + 1} {
		t.Run(fmt.Sprintf("%d_items", n), func(t *testing.T) {
			a := make([]int64, n)
			if !IsInt64Zeros(a) {
				t.Fatalf("IsInt64Zeros must return true")
			}
			if len(a) > 0 {
				a[len(a)-1] = 1
				if IsInt64Zeros(a) {
					t.Fatalf("IsInt64Zeros must return false")
				}
			}
		})
	}
}

func TestIsInt64Ones(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000, 1e4, 1e5, 8*1024 + 1} {
		t.Run(fmt.Sprintf("%d_items", n), func(t *testing.T) {
			a := make([]int64, n)
			for i := 0; i < n; i++ {
				a[i] = 1
			}
			if !IsInt64Ones(a) {
				t.Fatalf("IsInt64Ones must return true")
			}
			if len(a) > 0 {
				a[len(a)-1] = 0
				if IsInt64Ones(a) {
					t.Fatalf("IsInt64Ones must return false")
				}
			}
		})
	}
}

func TestIsFloat64Zeros(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000, 1e4, 1e5, 8*1024 + 1} {
		t.Run(fmt.Sprintf("%d_items", n), func(t *testing.T) {
			a := make([]float64, n)
			if !IsFloat64Zeros(a) {
				t.Fatalf("IsInt64Zeros must return true")
			}
			if len(a) > 0 {
				a[len(a)-1] = 1
				if IsFloat64Zeros(a) {
					t.Fatalf("IsInt64Zeros must return false")
				}
			}
		})
	}
}

func TestIsFloat64Ones(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000, 1e4, 1e5, 8*1024 + 1} {
		t.Run(fmt.Sprintf("%d_items", n), func(t *testing.T) {
			a := make([]float64, n)
			for i := 0; i < n; i++ {
				a[i] = 1
			}
			if !IsFloat64Ones(a) {
				t.Fatalf("IsInt64Ones must return true")
			}
			if len(a) > 0 {
				a[len(a)-1] = 0
				if IsFloat64Ones(a) {
					t.Fatalf("IsInt64Ones must return false")
				}
			}
		})
	}
}

func TestAppendInt64Zeros(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000, 1e4, 1e5, 8*1024 + 1} {
		t.Run(fmt.Sprintf("%d_items", n), func(t *testing.T) {
			a := AppendInt64Zeros(nil, n)
			if len(a) != n {
				t.Fatalf("unexpected len(a); got %d; want %d", len(a), n)
			}
			if !IsInt64Zeros(a) {
				t.Fatalf("IsInt64Zeros must return true")
			}

			prefix := []int64{1, 2, 3}
			a = AppendInt64Zeros(prefix, n)
			if len(a) != len(prefix)+n {
				t.Fatalf("unexpected len(a) with prefix; got %d; want %d", len(a), len(prefix)+n)
			}
			for i := 0; i < len(prefix); i++ {
				if a[i] != prefix[i] {
					t.Fatalf("unexpected prefix[%d]; got %d; want %d", i, a[i], prefix[i])
				}
			}
			if !IsInt64Zeros(a[len(prefix):]) {
				t.Fatalf("IsInt64Zeros for prefixed a must return true")
			}
		})
	}
}

func TestAppendInt64Ones(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000, 1e4, 1e5, 8*1024 + 1} {
		t.Run(fmt.Sprintf("%d_items", n), func(t *testing.T) {
			a := AppendInt64Ones(nil, n)
			if len(a) != n {
				t.Fatalf("unexpected len(a); got %d; want %d", len(a), n)
			}
			if !IsInt64Ones(a) {
				t.Fatalf("IsInt64Ones must return true")
			}

			prefix := []int64{1, 2, 3}
			a = AppendInt64Ones(prefix, n)
			if len(a) != len(prefix)+n {
				t.Fatalf("unexpected len(a) with prefix; got %d; want %d", len(a), len(prefix)+n)
			}
			for i := 0; i < len(prefix); i++ {
				if a[i] != prefix[i] {
					t.Fatalf("unexpected prefix[%d]; got %d; want %d", i, a[i], prefix[i])
				}
			}
			if !IsInt64Ones(a[len(prefix):]) {
				t.Fatalf("IsInt64Ones for prefixed a must return true")
			}
		})
	}
}

func TestAppendFloat64Zeros(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000, 1e4, 1e5, 8*1024 + 1} {
		t.Run(fmt.Sprintf("%d_items", n), func(t *testing.T) {
			a := AppendFloat64Zeros(nil, n)
			if len(a) != n {
				t.Fatalf("unexpected len(a); got %d; want %d", len(a), n)
			}
			if !IsFloat64Zeros(a) {
				t.Fatalf("IsFloat64Zeros must return true")
			}

			prefix := []float64{1, 2, 3}
			a = AppendFloat64Zeros(prefix, n)
			if len(a) != len(prefix)+n {
				t.Fatalf("unexpected len(a) with prefix; got %d; want %d", len(a), len(prefix)+n)
			}
			for i := 0; i < len(prefix); i++ {
				if a[i] != prefix[i] {
					t.Fatalf("unexpected prefix[%d]; got %f; want %f", i, a[i], prefix[i])
				}
			}
			if !IsFloat64Zeros(a[len(prefix):]) {
				t.Fatalf("IsFloat64Zeros for prefixed a must return true")
			}
		})
	}
}

func TestAppendFloat64Ones(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100, 1000, 1e4, 1e5, 8*1024 + 1} {
		t.Run(fmt.Sprintf("%d_items", n), func(t *testing.T) {
			a := AppendFloat64Ones(nil, n)
			if len(a) != n {
				t.Fatalf("unexpected len(a); got %d; want %d", len(a), n)
			}
			if !IsFloat64Ones(a) {
				t.Fatalf("IsFloat64Ones must return true")
			}

			prefix := []float64{1, 2, 3}
			a = AppendFloat64Ones(prefix, n)
			if len(a) != len(prefix)+n {
				t.Fatalf("unexpected len(a) with prefix; got %d; want %d", len(a), len(prefix)+n)
			}
			for i := 0; i < len(prefix); i++ {
				if a[i] != prefix[i] {
					t.Fatalf("unexpected prefix[%d]; got %f; want %f", i, a[i], prefix[i])
				}
			}
			if !IsFloat64Ones(a[len(prefix):]) {
				t.Fatalf("IsFloat64Ones for prefixed a must return true")
			}
		})
	}
}
