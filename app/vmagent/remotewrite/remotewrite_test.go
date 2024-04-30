package remotewrite

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
	"github.com/VictoriaMetrics/metrics"
)

func TestGetLabelsHash_Distribution(t *testing.T) {
	f := func(bucketsCount int) {
		t.Helper()

		// Distribute itemsCount hashes returned by getLabelsHash() across bucketsCount buckets.
		itemsCount := 1_000 * bucketsCount
		m := make([]int, bucketsCount)
		var labels []prompbmarshal.Label
		for i := 0; i < itemsCount; i++ {
			labels = append(labels[:0], prompbmarshal.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("some_name_%d", i),
			})
			for j := 0; j < 10; j++ {
				labels = append(labels, prompbmarshal.Label{
					Name:  fmt.Sprintf("label_%d", j),
					Value: fmt.Sprintf("value_%d_%d", i, j),
				})
			}
			h := getLabelsHash(labels)
			m[h%uint64(bucketsCount)]++
		}

		// Verify that the distribution is even
		expectedItemsPerBucket := itemsCount / bucketsCount
		for _, n := range m {
			if math.Abs(1-float64(n)/float64(expectedItemsPerBucket)) > 0.04 {
				t.Fatalf("unexpected items in the bucket for %d buckets; got %d; want around %d", bucketsCount, n, expectedItemsPerBucket)
			}
		}
	}

	f(2)
	f(3)
	f(4)
	f(5)
	f(10)
}

func TestRemoteWriteContext_TryPush_ImmutableTimeseries(t *testing.T) {
	f := func(streamAggrConfig, relabelConfig string, dedupInterval time.Duration, keepInput, dropInput bool, input string) {
		t.Helper()
		perURLRelabel, err := promrelabel.ParseRelabelConfigsData([]byte(relabelConfig))
		if err != nil {
			t.Fatalf("cannot load relabel configs: %s", err)
		}
		rcs := &relabelConfigs{
			perURL: []*promrelabel.ParsedConfigs{
				perURLRelabel,
			},
		}
		allRelabelConfigs.Store(rcs)

		sf := significantFigures.GetOptionalArg(0)
		rd := roundDigits.GetOptionalArg(0)
		pss := make([]*pendingSeries, 1)
		for i := range pss {
			pss[i] = newPendingSeries(nil, true, sf, rd)
		}

		rwctx := &remoteWriteCtx{
			idx:                    0,
			streamAggrKeepInput:    keepInput,
			streamAggrDropInput:    dropInput,
			pss:                    pss,
			rowsPushedAfterRelabel: metrics.GetOrCreateCounter(`foo`),
			rowsDroppedByRelabel:   metrics.GetOrCreateCounter(`bar`),
		}

		if len(streamAggrConfig) > 0 {
			sas, err := streamaggr.NewAggregatorsFromData([]byte(streamAggrConfig), nil, nil)
			if err != nil {
				t.Fatalf("cannot load streamaggr configs: %s", err)
			}
			rwctx.sas.Store(sas)
		}
		if dedupInterval > 0 {
			rwctx.deduplicator = streamaggr.NewDeduplicator(nil, dedupInterval, nil)
		}

		inputTss := mustParsePromMetrics(input)
		expectedTss := make([]prompbmarshal.TimeSeries, len(inputTss))
		copy(expectedTss, inputTss)
		rwctx.TryPush(inputTss)
		if !reflect.DeepEqual(expectedTss, inputTss) {
			t.Fatalf("unexpected samples;\ngot\n%v\nwant\n%v", expectedTss, inputTss)
		}
	}

	f(`
- interval: 1m
  outputs: [sum_samples]
- interval: 2m
  outputs: [count_series]
`, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, 0, false, false, `
metric{env="dev"} 10
metric{env="bar"} 20
metric{env="dev"} 15
metric{env="bar"} 25
`)
	f(``, ``, time.Hour, false, false, `
metric{env="dev"} 10
metric{env="foo"} 20
metric{env="dev"} 15
metric{env="foo"} 25
`)
	f(``, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, time.Hour, false, false, `
metric{env="dev"} 10
metric{env="bar"} 20
metric{env="dev"} 15
metric{env="bar"} 25
`)
	f(``, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, time.Hour, true, false, `
metric{env="test"} 10
metric{env="dev"} 20
metric{env="foo"} 15
metric{env="dev"} 25
`)
	f(``, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, time.Hour, false, true, `
metric{env="foo"} 10
metric{env="dev"} 20
metric{env="foo"} 15
metric{env="dev"} 25
`)
	f(``, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, time.Hour, true, true, `
metric{env="dev"} 10
metric{env="test"} 20
metric{env="dev"} 15
metric{env="bar"} 25
`)
}

func mustParsePromMetrics(s string) []prompbmarshal.TimeSeries {
	var rows prometheus.Rows
	errLogger := func(s string) {
		panic(fmt.Errorf("unexpected error when parsing Prometheus metrics: %s", s))
	}
	rows.UnmarshalWithErrLogger(s, errLogger)
	var tss []prompbmarshal.TimeSeries
	samples := make([]prompbmarshal.Sample, 0, len(rows.Rows))
	for _, row := range rows.Rows {
		labels := make([]prompbmarshal.Label, 0, len(row.Tags)+1)
		labels = append(labels, prompbmarshal.Label{
			Name:  "__name__",
			Value: row.Metric,
		})
		for _, tag := range row.Tags {
			labels = append(labels, prompbmarshal.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		samples = append(samples, prompbmarshal.Sample{
			Value:     row.Value,
			Timestamp: row.Timestamp,
		})
		ts := prompbmarshal.TimeSeries{
			Labels:  labels,
			Samples: samples[len(samples)-1:],
		}
		tss = append(tss, ts)
	}
	return tss
}
