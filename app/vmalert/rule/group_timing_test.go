package rule

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkGetStaleSeries(b *testing.B) {
	ts := time.Now()
	n := 100
	payload := make([]prompbmarshal.TimeSeries, 0, n)
	for i := 0; i < n; i++ {
		s := fmt.Sprintf("%d", i)
		labels := toPromLabels(b,
			"__name__", "foo", ""+
				"instance", s,
			"job", s,
			"state", s,
		)
		payload = append(payload, newTimeSeriesPB([]float64{1}, []int64{ts.Unix()}, labels))
	}

	e := &executor{
		previouslySentSeriesToRW: make(map[uint64]map[string][]prompbmarshal.Label),
	}
	ar := &AlertingRule{RuleID: 1}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		e.getStaleSeries(ar, payload, ts)
	}
}
