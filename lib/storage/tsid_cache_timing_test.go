package storage

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// metricNameSizes are the approximate marshaled metric name sizes at which the
// tsid cache key modes are compared.
var metricNameSizes = []int{100, 300, 800, 2048}

// buildRowsWithNameSize returns MetricRows for the given number of distinct
// series, with MetricNameRaw of approximately nameSize bytes.
func buildRowsWithNameSize(series, nameSize int) []MetricRow {
	ts := time.Now().UnixMilli()
	mrs := make([]MetricRow, 0, series)
	var mnRaw []byte
	for i := 0; i < series; i++ {
		labels := []prompb.Label{
			{Name: "__name__", Value: fmt.Sprintf("request_duration_seconds_%06d", i)},
			{Name: "job", Value: "api"},
			{Name: "instance", Value: fmt.Sprintf("host-%04d:9090", i%1000)},
		}
		// Pad with label values until MetricNameRaw reaches nameSize.
		for n := 0; ; n++ {
			mnRaw = MarshalMetricNameRaw(mnRaw[:0], labels)
			if len(mnRaw) >= nameSize {
				break
			}
			pad := min(nameSize-len(mnRaw), 64)
			labels = append(labels, prompb.Label{
				Name:  fmt.Sprintf("label_%02d", n),
				Value: strings.Repeat("v", pad),
			})
		}
		mrs = append(mrs, MetricRow{
			MetricNameRaw: append([]byte(nil), mnRaw...),
			Timestamp:     ts,
			Value:         float64(i),
		})
	}
	return mrs
}

// BenchmarkTSIDCacheKey measures the fingerprint construction alone.
func BenchmarkTSIDCacheKey(b *testing.B) {
	var sink uint64
	for _, nameSize := range metricNameSizes {
		metricNameRaw := buildRowsWithNameSize(1, nameSize)[0].MetricNameRaw
		b.Run(fmt.Sprintf("nameSize=%d", len(metricNameRaw)), func(b *testing.B) {
			var kb [16]byte
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := marshalTSIDCacheKey(&kb, metricNameRaw)
				sink += uint64(key[0])
			}
		})
	}
	if sink == 0 {
		b.Fatalf("unexpected zero sink")
	}
}

// BenchmarkTSIDCacheGetPut measures tsid cache lookups and stores in both key
// modes across metric name sizes.
func BenchmarkTSIDCacheGetPut(b *testing.B) {
	const series = 20000
	for _, mode := range []string{tsidCacheKeyModeMetricName, tsidCacheKeyModeFingerprint} {
		for _, nameSize := range metricNameSizes {
			mrs := buildRowsWithNameSize(series, nameSize)
			b.Run(fmt.Sprintf("%s/nameSize=%d", mode, len(mrs[0].MetricNameRaw)), func(b *testing.B) {
				restore := setTSIDCacheKeyModeForBench(mode)
				defer restore()

				s := MustOpenStorage(b.TempDir(), OpenOptions{})
				defer s.MustClose()
				s.AddRows(mrs, 64)
				s.DebugFlush()

				b.Run("get", func(b *testing.B) {
					var lTSID legacyTSID
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						mr := &mrs[i%len(mrs)]
						if !s.getTSIDByMetricNameFromCache(&lTSID, mr.MetricNameRaw) {
							b.Fatalf("unexpected cache miss")
						}
					}
				})
				b.Run("put", func(b *testing.B) {
					lTSID := legacyTSID{TSID: TSID{MetricID: 42}}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						mr := &mrs[i%len(mrs)]
						s.putTSIDByMetricNameToCache(&lTSID, mr.MetricNameRaw)
					}
				})
			})
		}
	}
}

// setTSIDCacheKeyModeForBench sets the package-global key mode and returns a
// function restoring the previous one.
func setTSIDCacheKeyModeForBench(mode string) func() {
	prevFlag := tsidCacheFingerprintKey
	SetTSIDCacheKeyMode(mode)
	return func() {
		tsidCacheFingerprintKey = prevFlag
	}
}
