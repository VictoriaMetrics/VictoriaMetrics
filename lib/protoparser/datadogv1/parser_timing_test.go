package datadogv1

import (
	"fmt"
	"testing"
)

func BenchmarkRequestUnmarshal(b *testing.B) {
	reqBody := []byte(`{
  "series": [
    {
      "host": "test.example.com",
      "interval": 20,
      "metric": "system.load.1",
      "points": [[
        1575317847,
        0.5
      ]],
      "tags": [
        "environment:test"
      ],
      "type": "rate"
    }
  ]
}`)
	b.SetBytes(int64(len(reqBody)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var req Request
		for pb.Next() {
			if err := req.Unmarshal(reqBody); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			if len(req.Series) != 1 {
				panic(fmt.Errorf("unexpected number of series unmarshaled: got %d; want 4", len(req.Series)))
			}
		}
	})
}
