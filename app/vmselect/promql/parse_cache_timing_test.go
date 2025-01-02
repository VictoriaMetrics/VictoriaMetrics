package promql

import (
	"testing"
)

func BenchmarkCachePutNoOverFlow(b *testing.B) {
	const items int = (parseCacheMaxLen / 2)
	pc := newParseCache()

	queries := testGenerateQueries(items)
	v := testGetParseCacheValue(queries[0])

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < items; i++ {
				pc.put(queries[i], v)
			}
		}
	})
	if pc.len() != uint64(items) {
		b.Errorf("unexpected value obtained; got %d; want %d", pc.len(), items)
	}
}

func BenchmarkCacheGetNoOverflow(b *testing.B) {
	const items int = parseCacheMaxLen / 2
	pc := newParseCache()

	queries := testGenerateQueries(items)
	v := testGetParseCacheValue(queries[0])

	for i := 0; i < len(queries); i++ {
		pc.put(queries[i], v)
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < items; i++ {
				if v := pc.get(queries[i]); v == nil {
					b.Errorf("unexpected nil value obtained from cache for query: %s ", queries[i])
				}
			}
		}
	})
}

func BenchmarkCachePutGetNoOverflow(b *testing.B) {
	const items int = parseCacheMaxLen / 2
	pc := newParseCache()

	queries := testGenerateQueries(items)
	v := testGetParseCacheValue(queries[0])

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < items; i++ {
				pc.put(queries[i], v)
				if res := pc.get(queries[i]); res == nil {
					b.Errorf("unexpected nil value obtained from cache for query: %s ", queries[i])
				}
			}
		}
	})
	if pc.len() != uint64(items) {
		b.Errorf("unexpected value obtained; got %d; want %d", pc.len(), items)
	}
}

func BenchmarkCachePutOverflow(b *testing.B) {
	const items int = parseCacheMaxLen + (parseCacheMaxLen / 2)
	c := newParseCache()

	queries := testGenerateQueries(items)
	v := testGetParseCacheValue(queries[0])

	for i := 0; i < parseCacheMaxLen; i++ {
		c.put(queries[i], v)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := parseCacheMaxLen; i < items; i++ {
				c.put(queries[i], v)
			}
		}
	})
	maxElemnts := uint64(parseCacheMaxLen + parseBucketCount)
	if c.len() > maxElemnts {
		b.Errorf("cache length is more than expected; got %d, expected %d", c.len(), maxElemnts)
	}
}

func BenchmarkCachePutGetOverflow(b *testing.B) {
	const items int = parseCacheMaxLen + (parseCacheMaxLen / 2)
	c := newParseCache()

	queries := testGenerateQueries(items)
	v := testGetParseCacheValue(queries[0])

	for i := 0; i < parseCacheMaxLen; i++ {
		c.put(queries[i], v)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := parseCacheMaxLen; i < items; i++ {
				c.put(queries[i], v)
				c.get(queries[i])
			}
		}
	})
	maxElemnts := uint64(parseCacheMaxLen + parseBucketCount)
	if c.len() > maxElemnts {
		b.Errorf("cache length is more than expected; got %d, expected %d", c.len(), maxElemnts)
	}
}

var testSimpleQueries = []string{
	`m{a="b"}`,
	`{a="b"}`,
	`m{c="d",a="b"}`,
	`{a="b",c="d"}`,
	`m1{a="foo"}`,
	`m2{a="bar"}`,
	`m1{b="foo"}`,
	`m2{b="bar"}`,
	`m1{a="foo",b="bar"}`,
	`m2{b="bar",c="x"}`,
	`{b="bar"}`,
}

func BenchmarkParsePromQLWithCacheSimple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(testSimpleQueries); j++ {
			_, err := parsePromQLWithCache(testSimpleQueries[j])
			if err != nil {
				b.Errorf("unexpected error: %s", err)
			}
		}
	}
}

func BenchmarkParsePromQLWithCacheSimpleParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < len(testSimpleQueries); i++ {
				_, err := parsePromQLWithCache(testSimpleQueries[i])
				if err != nil {
					b.Errorf("unexpected error: %s", err)
				}
			}
		}
	})
}

var testComplexQueries = []string{
	`sort_desc(label_set(2, "foo", "bar") * ignoring(a) (label_set(time(), "foo", "bar") or label_set(10, "foo", "qwert")))`,
	`sum(a.b{c="d.e",x=~"a.b.+[.a]",y!~"aaa.bb|cc.dd"}) + avg_over_time(1,sum({x=~"aa.bb"}))`,
	`sort((label_set(time() offset 100s, "foo", "bar"), label_set(time()+10, "foo", "baz") offset 50s) offset 400s)`,
	`sort(label_map((
			label_set(time(), "label", "v1"),
			label_set(time()+100, "label", "v2"),
			label_set(time()+200, "label", "v3"),
			label_set(time()+300, "x", "y"),
			label_set(time()+400, "label", "v4"),
		), "label", "v1", "foo", "v2", "bar", "", "qwe", "v4", ""))`,
	`sort(labels_equal((
			label_set(10, "instance", "qwe", "host", "rty"),
			label_set(20, "instance", "qwe", "host", "qwe"),
			label_set(30, "aaa", "bbb", "instance", "foo", "host", "foo"),
		), "instance", "host"))`,
	`with (
			x = (
				label_set(time() > 1500, "foo", "123.456", "__name__", "aaa"),
				label_set(-time(), "foo", "bar", "__name__", "bbb"),
				label_set(-time(), "__name__", "bxs"),
				label_set(-time(), "foo", "45", "bar", "xs"),
			)
		)
		sort(x + label_value(x, "foo"))`,
	`label_replace(
			label_replace(
				label_replace(time(), "__name__", "x${1}y", "foo", ".*"),
				"xxx", "foo${1}bar(${1})", "__name__", "(.+)"),
			"xxx", "AA$1", "xxx", "foox(.+)"
		)`,
	`sort_desc(union(
			label_set(time() > 1400, "__name__", "x", "foo", "bar"),
			label_set(time() < 1700, "__name__", "y", "foo", "baz")) default 123)`,
	`sort(histogram_quantile(0.6,
			label_set(90, "foo", "bar", "le", "10")
			or label_set(100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf")
			or label_set(200, "tag", "xx", "le", "10")
			or label_set(300, "tag", "xx", "le", "30")
		))`,
}

func BenchmarkParsePromQLWithCacheComplex(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(testComplexQueries); j++ {
			_, err := parsePromQLWithCache(testComplexQueries[j])
			if err != nil {
				b.Errorf("unexpected error: %s", err)
			}
		}
	}
}

func BenchmarkParsePromQLWithCacheComplexParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < len(testComplexQueries); i++ {
				_, err := parsePromQLWithCache(testComplexQueries[i])
				if err != nil {
					b.Errorf("unexpected error: %s", err)
				}
			}
		}
	})
}
