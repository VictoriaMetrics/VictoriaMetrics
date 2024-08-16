package stringsutil

import (
	"testing"
)

func BenchmarkLessNatural(b *testing.B) {
	b.Run("distinct_string_prefixes", func(b *testing.B) {
		benchmarkLessNatural(b, []string{
			"aaa", "bbb", "ccc", "ddd", "eee", "fff", "g", "hh", "kkk", "ooo", "ppppp", "wwww", "zzz", "qqq",
		})
	})
	b.Run("distinct_numeric_values", func(b *testing.B) {
		benchmarkLessNatural(b, []string{
			"111", "222", "333", "44", "5555", "666", "7", "88", "999", "000", "123", "452", "34", "234",
		})
	})
	b.Run("common_string_prefixes_distinct_string_suffixes", func(b *testing.B) {
		benchmarkLessNatural(b, []string{
			"XXXaaa", "XXXbbb", "XXXccc", "XXXddd", "XXXeee", "XXXfff", "XXXg", "XXXhh", "XXXkkk", "XXXooo", "XXXppppp", "XXXwwww", "XXXzzz", "XXXqqq",
		})
	})
	b.Run("common_string_prefixes_distinct_numeric_suffixes", func(b *testing.B) {
		benchmarkLessNatural(b, []string{
			"XXX111", "XXX222", "XXX33", "XXX4", "XXX555", "XXX666", "XXX7", "XXX88", "XXX999", "XXX000", "XXX12345", "XXX3211", "XXX873", "XXX98",
		})
	})
	b.Run("common_string_values", func(b *testing.B) {
		benchmarkLessNatural(b, []string{
			"XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX", "XXXXX",
		})
	})
	b.Run("common_numeric_prefixes", func(b *testing.B) {
		benchmarkLessNatural(b, []string{
			"111aaa", "111bbb", "111ccc", "111ddd", "111eee", "111fff", "111g", "111hh", "111kkk", "111ooo", "111ppppp", "111wwww", "111zzz", "111qqq",
		})
	})
	b.Run("common_numeric_values", func(b *testing.B) {
		benchmarkLessNatural(b, []string{
			"11111", "11111", "11111", "11111", "11111", "11111", "11111", "11111", "11111", "11111", "11111", "11111", "11111", "11111",
		})
	})
}

func benchmarkLessNatural(b *testing.B, a []string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(a) - 1))
	b.RunParallel(func(pb *testing.PB) {
		n := uint64(0)
		for pb.Next() {
			for i := 1; i < len(a); i++ {
				if LessNatural(a[i-1], a[i]) {
					n++
				}
			}
		}
		GlobalSink.Add(n)
	})
}
