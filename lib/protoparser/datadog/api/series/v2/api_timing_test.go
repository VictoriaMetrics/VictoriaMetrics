package datadog

import (
	"fmt"
	"testing"
)

func BenchmarkRequestUnmarshal(b *testing.B) {
	reqBody := []byte(`{
  "series": [
    {
      "interval": 20,
      "metric": "system.load.1",
			"resources": [{
				"name": "test.example.com",
				"type": "host"
			}],
      "points": [
				{
					"timestamp": 1575317847,
					"value": 0.5
				}
      ],
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
		req := new(Request)
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
