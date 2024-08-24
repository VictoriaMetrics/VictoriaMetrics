package datasource

import (
	"os"
	"testing"
)

func BenchmarkPromInstantUnmarshal(b *testing.B) {
	data, err := os.ReadFile("testdata/instant_response.json")
	if err != nil {
		b.Fatalf("error while reading file: %s", err)
	}

	// BenchmarkParsePrometheusResponse/Instant_std+fastjson-10                    1760            668959 ns/op          280147 B/op       5781 allocs/op
	b.Run("Instant std+fastjson", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var pi promInstant
			err = pi.Unmarshal(data)
			if err != nil {
				b.Fatalf("unexpected parse err: %s", err)
			}
		}
	})
}
