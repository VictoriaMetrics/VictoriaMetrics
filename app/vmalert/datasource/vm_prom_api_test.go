package datasource

import (
	"encoding/json"
	"testing"
	"time"
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

func TestGetPrometheusReqTimestamp(t *testing.T) {
	offset := 30 * time.Minute
	testCases := []struct {
		name               string
		s                  *VMStorage
		queryTimeAlignment bool
		originTS, expTS    string
	}{
		{
			"with eval_offset, find previous offset point",
			&VMStorage{
				evaluationOffset:   &offset,
				evaluationInterval: time.Hour,
				lookBack:           1 * time.Minute,
			},
			false,
			"2023-08-28T11:11:00+00:00",
			"2023-08-28T10:30:00+00:00",
		},
		{
			"with eval_offset",
			&VMStorage{
				evaluationOffset:   &offset,
				evaluationInterval: time.Hour,
			},
			true,
			"2023-08-28T11:41:00+00:00",
			"2023-08-28T11:30:00+00:00",
		},
		{
			"with query align",
			&VMStorage{
				evaluationInterval: time.Hour,
			},
			true,
			"2023-08-28T11:11:00+00:00",
			"2023-08-28T11:00:00+00:00",
		},
		{
			"with query align and lookback",
			&VMStorage{
				evaluationInterval: time.Hour,
				lookBack:           1 * time.Minute,
			},
			true,
			"2023-08-28T11:11:00+00:00",
			"2023-08-28T10:59:00+00:00",
		},
		{
			"without query align",
			&VMStorage{
				evaluationInterval: time.Hour,
			},
			false,
			"2023-08-28T11:11:00+00:00",
			"2023-08-28T11:11:00+00:00",
		},
	}
	for _, tc := range testCases {
		oldAlignPara := *queryTimeAlignment
		*queryTimeAlignment = tc.queryTimeAlignment
		originT, _ := time.Parse(time.RFC3339, tc.originTS)
		expT, _ := time.Parse(time.RFC3339, tc.expTS)
		gotTS := tc.s.adjustReqTimestamp(originT)
		if !gotTS.Equal(expT) {
			t.Fatalf("get wrong prometheus request timestamp, expect %s, got %s", expT, gotTS)
		}
		*queryTimeAlignment = oldAlignPara
	}
}
