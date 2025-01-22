package logstorage

import (
	"testing"
)

func BenchmarkGetColumnNameIDs(b *testing.B) {
	a := []string{"", "foo", "bar.baz", "asdf dsf dfs"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := getColumnNameIDs(a)
		if len(m) != len(a) {
			b.Errorf("unexpected map length; got %d; want %d", len(m), len(a))
		}
	}
}
