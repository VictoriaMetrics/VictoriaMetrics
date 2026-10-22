package storage

import (
	"fmt"
	"testing"
)

// BenchmarkBlockUnmarshalDataCached decodes blocks of one query through a TimestampsCache.
//
// Every k blocks the cache is reset, as at the end of a query.
// "hit" blocks share timestamps, as series of one scrape target do. "miss" blocks have distinct timestamps.
// k above tsCacheProbe checks the cost after the cache switches itself off.
func BenchmarkBlockUnmarshalDataCached(b *testing.B) {
	SetTimestampsCacheMaxBytes(1 << 30)
	defer SetTimestampsCacheMaxBytes(0)

	for _, c := range benchCases[1:] {
		for _, shared := range []bool{true, false} {
			for _, k := range []int{60, 512} {
				name := c.name + "/miss"
				if shared {
					name = c.name + "/hit"
				}
				b.Run(fmt.Sprintf("%s/k=%d", name, k), func(b *testing.B) {
					blocks := make([][]byte, k)
					for i := range blocks {
						seed := int64(1)
						if !shared {
							seed = int64(i + 1)
						}
						blocks[i] = benchMarshaledBlock(benchTimestamps(seed, benchBlockRows, c.jitterMs), benchValues(int64(i+1), benchBlockRows, c.counter))
					}

					b.ReportAllocs()
					b.SetBytes(int64(benchBlockRows * 16))
					b.ResetTimer()

					var cache TimestampsCache
					var blk Block
					for i := 0; i < b.N; i++ {
						if i%k == 0 {
							cache.Reset()
						}
						blk.Reset()
						if _, err := UnmarshalBlock(&blk, blocks[i%k]); err != nil {
							b.Fatalf("UnmarshalBlock: %s", err)
						}
						if err := blk.UnmarshalDataCached(&cache); err != nil {
							b.Fatalf("UnmarshalDataCached: %s", err)
						}
					}
				})
			}
		}
	}
}
