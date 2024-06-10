package datadogv2

import (
	"fmt"
	"testing"
)

func BenchmarkRequestUnmarshalJSON(b *testing.B) {
	reqBody := []byte(`{
  "series": [
    {
      "metric": "system.load.1",
      "type": 0,
      "points": [
        {
          "timestamp": 1636629071,
          "value": 0.7
        }
      ],
      "resources": [
        {
          "name": "dummyhost",
          "type": "host"
        }
      ],
      "tags": ["environment:test"]
    }
  ]
}`)
	b.SetBytes(int64(len(reqBody)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var req Request
		for pb.Next() {
			if err := UnmarshalJSON(&req, reqBody); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			if len(req.Series) != 1 {
				panic(fmt.Errorf("unexpected number of series unmarshaled: got %d; want 4", len(req.Series)))
			}
		}
	})
}
