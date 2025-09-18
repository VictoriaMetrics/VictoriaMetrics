package remotewrite

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consistenthash"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
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
		var labels []prompb.Label
		for i := 0; i < itemsCount; i++ {
			labels = append(labels[:0], prompb.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("some_name_%d", i),
			})
			for j := 0; j < 10; j++ {
				labels = append(labels, prompb.Label{
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
	f := func(streamAggrConfig, relabelConfig string, enableWindows bool, dedupInterval time.Duration, keepInput, dropInput bool, input string) {
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

		pss := make([]*pendingSeries, 1)
		isVMProto := &atomic.Bool{}
		isVMProto.Store(true)
		pss[0] = newPendingSeries(nil, isVMProto, 0, 100)
		rwctx := &remoteWriteCtx{
			idx:                    0,
			streamAggrKeepInput:    keepInput,
			streamAggrDropInput:    dropInput,
			pss:                    pss,
			rowsPushedAfterRelabel: metrics.GetOrCreateCounter(`foo`),
			rowsDroppedByRelabel:   metrics.GetOrCreateCounter(`bar`),
		}
		if dedupInterval > 0 {
			rwctx.deduplicator = streamaggr.NewDeduplicator(nil, enableWindows, dedupInterval, nil, "dedup-global")
		}

		if streamAggrConfig != "" {
			pushNoop := func(_ []prompb.TimeSeries) {}
			opts := streamaggr.Options{
				EnableWindows: enableWindows,
			}
			sas, err := streamaggr.LoadFromData([]byte(streamAggrConfig), pushNoop, &opts, "global")
			if err != nil {
				t.Fatalf("cannot load streamaggr configs: %s", err)
			}
			defer sas.MustStop()
			rwctx.sas.Store(sas)
		}

		offsetMsecs := time.Now().UnixMilli()
		inputTss := prometheus.MustParsePromMetrics(input, offsetMsecs)
		expectedTss := make([]prompb.TimeSeries, len(inputTss))

		// copy inputTss to make sure it is not mutated during TryPush call
		copy(expectedTss, inputTss)
		if !rwctx.TryPushTimeSeries(inputTss, false) {
			t.Fatalf("cannot push samples to rwctx")
		}

		if !reflect.DeepEqual(expectedTss, inputTss) {
			t.Fatalf("unexpected samples;\ngot\n%v\nwant\n%v", inputTss, expectedTss)
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
`, false, 0, false, false, `
metric{env="dev"} 10
metric{env="bar"} 20
metric{env="dev"} 15
metric{env="bar"} 25
`)
	f(``, ``, true, time.Hour, false, false, `
metric{env="dev"} 10
metric{env="foo"} 20
metric{env="dev"} 15
metric{env="foo"} 25
`)
	f(``, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, true, time.Hour, false, false, `
metric{env="dev"} 10
metric{env="bar"} 20
metric{env="dev"} 15
metric{env="bar"} 25
`)
	f(``, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, true, time.Hour, true, false, `
metric{env="test"} 10
metric{env="dev"} 20
metric{env="foo"} 15
metric{env="dev"} 25
`)
	f(``, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, true, time.Hour, false, true, `
metric{env="foo"} 10
metric{env="dev"} 20
metric{env="foo"} 15
metric{env="dev"} 25
`)
	f(``, `
- action: keep
  source_labels: [env]
  regex: "dev"
`, true, time.Hour, true, true, `
metric{env="dev"} 10
metric{env="test"} 20
metric{env="dev"} 15
metric{env="bar"} 25
`)
}

func TestShardAmountRemoteWriteCtx(t *testing.T) {
	// 1. distribute 100000 series to n nodes.
	// 2. remove the last node from healthy list.
	// 3. distribute the same 10000 series to (n-1) node again.
	// 4. check active time series change rate:
	// change rate must < (3/total nodes). e.g. +30% if 10 you have 10 nodes.

	f := func(remoteWriteCount int, healthyIdx []int, replicas int) {
		t.Helper()
		defer func() {
			rwctxsGlobal = nil
			rwctxsGlobalIdx = nil
			rwctxConsistentHashGlobal = nil
		}()

		rwctxsGlobal = make([]*remoteWriteCtx, remoteWriteCount)
		rwctxsGlobalIdx = make([]int, remoteWriteCount)
		rwctxs := make([]*remoteWriteCtx, 0, len(healthyIdx))

		for i := range remoteWriteCount {
			rwCtx := &remoteWriteCtx{
				idx: i,
			}
			rwctxsGlobalIdx[i] = i

			if i >= len(healthyIdx) {
				rwctxsGlobal[i] = rwCtx
				continue
			}
			hIdx := healthyIdx[i]
			if hIdx != i {
				rwctxs = append(rwctxs, &remoteWriteCtx{
					idx: hIdx,
				})
			} else {
				rwctxs = append(rwctxs, rwCtx)
			}
			rwctxsGlobal[i] = rwCtx
		}

		seriesCount := 100000
		// build 1000000 series
		tssBlock := make([]prompb.TimeSeries, 0, seriesCount)
		for i := 0; i < seriesCount; i++ {
			tssBlock = append(tssBlock, prompb.TimeSeries{
				Labels: []prompb.Label{
					{
						Name:  "label",
						Value: strconv.Itoa(i),
					},
				},
				Samples: []prompb.Sample{
					{
						Timestamp: 0,
						Value:     0,
					},
				},
			})
		}

		// build consistent hash for x remote write context
		// build active time series set
		nodes := make([]string, 0, remoteWriteCount)
		activeTimeSeriesByNodes := make([]map[string]struct{}, remoteWriteCount)
		for i := 0; i < remoteWriteCount; i++ {
			nodes = append(nodes, fmt.Sprintf("node%d", i))
			activeTimeSeriesByNodes[i] = make(map[string]struct{})
		}
		rwctxConsistentHashGlobal = consistenthash.NewConsistentHash(nodes, 0)

		// create shards
		x := getTSSShards(len(rwctxs))
		shards := x.shards

		// execute
		shardAmountRemoteWriteCtx(tssBlock, shards, rwctxs, replicas)

		for i, nodeIdx := range healthyIdx {
			for _, ts := range shards[i] {
				// add it to node[nodeIdx]'s active time series
				activeTimeSeriesByNodes[nodeIdx][prompb.LabelsToString(ts.Labels)] = struct{}{}
			}
		}

		totalActiveTimeSeries := 0
		for _, activeTimeSeries := range activeTimeSeriesByNodes {
			totalActiveTimeSeries += len(activeTimeSeries)
		}
		avgActiveTimeSeries1 := totalActiveTimeSeries / remoteWriteCount
		putTSSShards(x)

		// removed last node
		rwctxs = rwctxs[:len(rwctxs)-1]
		healthyIdx = healthyIdx[:len(healthyIdx)-1]

		x = getTSSShards(len(rwctxs))
		shards = x.shards

		// execute
		shardAmountRemoteWriteCtx(tssBlock, shards, rwctxs, replicas)
		for i, nodeIdx := range healthyIdx {
			for _, ts := range shards[i] {
				// add it to node[nodeIdx]'s active time series
				activeTimeSeriesByNodes[nodeIdx][prompb.LabelsToString(ts.Labels)] = struct{}{}
			}
		}

		totalActiveTimeSeries = 0
		for _, activeTimeSeries := range activeTimeSeriesByNodes {
			totalActiveTimeSeries += len(activeTimeSeries)
		}
		avgActiveTimeSeries2 := totalActiveTimeSeries / remoteWriteCount

		changed := math.Abs(float64(avgActiveTimeSeries2-avgActiveTimeSeries1) / float64(avgActiveTimeSeries1))
		threshold := 3 / float64(remoteWriteCount)

		if changed >= threshold {
			t.Fatalf("average active time series before: %d, after: %d, changed: %.2f. threshold: %.2f", avgActiveTimeSeries1, avgActiveTimeSeries2, changed, threshold)
		}

	}

	f(5, []int{0, 1, 2, 3, 4}, 1)

	f(5, []int{0, 1, 2, 3, 4}, 2)

	f(10, []int{0, 1, 2, 3, 4, 5, 6, 7, 9}, 1)

	f(10, []int{0, 1, 2, 3, 4, 5, 6, 7, 9}, 3)
}

func TestCalculateHealthyRwctxIdx(t *testing.T) {
	f := func(total int, healthyIdx []int, unhealthyIdx []int) {
		t.Helper()

		healthyMap := make(map[int]bool)
		for _, idx := range healthyIdx {
			healthyMap[idx] = true
		}
		rwctxsGlobal = make([]*remoteWriteCtx, total)
		rwctxsGlobalIdx = make([]int, total)
		rwctxs := make([]*remoteWriteCtx, 0, len(healthyIdx))
		for i := range rwctxsGlobal {
			rwctx := &remoteWriteCtx{idx: i}
			rwctxsGlobal[i] = rwctx
			if healthyMap[i] {
				rwctxs = append(rwctxs, rwctx)
			}
			rwctxsGlobalIdx[i] = i
		}

		gotHealthyIdx, gotUnhealthyIdx := calculateHealthyRwctxIdx(rwctxs)
		if !reflect.DeepEqual(healthyIdx, gotHealthyIdx) {
			t.Errorf("calculateHealthyRwctxIdx want healthyIdx = %v, got %v", healthyIdx, gotHealthyIdx)
		}
		if !reflect.DeepEqual(unhealthyIdx, gotUnhealthyIdx) {
			t.Errorf("calculateHealthyRwctxIdx want unhealthyIdx = %v, got %v", unhealthyIdx, gotUnhealthyIdx)
		}
	}

	f(5, []int{0, 1, 2, 3, 4}, nil)
	f(5, []int{0, 1, 2, 4}, []int{3})
	f(5, []int{2, 4}, []int{0, 1, 3})
	f(5, []int{0, 2, 4}, []int{1, 3})
	f(5, []int{}, []int{0, 1, 2, 3, 4})
	f(5, []int{4}, []int{0, 1, 2, 3})
	f(1, []int{0}, nil)
	f(1, []int{}, []int{0})
}
