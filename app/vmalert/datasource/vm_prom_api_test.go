package datasource

import (
	"encoding/json"
	"testing"
)

func BenchmarkMetrics(b *testing.B) {
	payload := []byte(`[{"metric":{"__name__":"vm_rows"},"value":[1583786142,"13763"]},{"metric":{"__name__":"vm_requests", "foo":"bar", "baz": "qux"},"value":[1583786140,"2000"]}]`)

	var pi promInstant
	if err := json.Unmarshal(payload, &pi.Result); err != nil {
		b.Fatalf(err.Error())
	}
	b.Run("Instant", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = pi.metrics()
		}
	})
}
