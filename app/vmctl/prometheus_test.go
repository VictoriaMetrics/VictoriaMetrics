package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	remote_read_integration "github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/testdata/servers_integration_test"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
)

const (
	testSnapshot = "./testdata/snapshots/20250118T124506Z-59d1b952d7eaf547"
	blockData    = "./testdata/snapshots/20250118T124506Z-59d1b952d7eaf547/01JHWQ445Y2P1TDYB05AEKD6MC"
)

// This test simulates close process if user abort it
func TestPrometheusProcessorRun(t *testing.T) {

	f := func(startStr, endStr string, numOfSeries int, resultExpected []vm.TimeSeries) {
		t.Helper()

		dst := remote_read_integration.NewRemoteWriteServer(t)

		defer func() {
			dst.Close()
		}()

		dst.Series(resultExpected)
		dst.ExpectedSeries(resultExpected)

		if err := fillStorage(resultExpected); err != nil {
			t.Fatalf("cannot fill storage: %s", err)
		}

		isSilent = true
		defer func() { isSilent = false }()

		bf, err := backoff.New(1, 1.8, time.Second*2)
		if err != nil {
			t.Fatalf("cannot create backoff: %s", err)
		}

		importerCfg := vm.Config{
			Addr:        dst.URL(),
			Transport:   nil,
			Concurrency: 1,
			Backoff:     bf,
		}

		ctx := context.Background()
		importer, err := vm.NewImporter(ctx, importerCfg)
		if err != nil {
			t.Fatalf("cannot create importer: %s", err)
		}
		defer importer.Close()

		matchName := "__name__"
		matchValue := ".*"
		filter := prometheus.Filter{
			TimeMin:    startStr,
			TimeMax:    endStr,
			Label:      matchName,
			LabelValue: matchValue,
		}

		runnner, err := prometheus.NewClient(prometheus.Config{
			Snapshot: testSnapshot,
			Filter:   filter,
		})
		if err != nil {
			t.Fatalf("cannot create prometheus client: %s", err)
		}
		p := &prometheusProcessor{
			cl: runnner,
			im: importer,
			cc: 1,
		}

		if err := p.run(); err != nil {
			t.Fatalf("run() error: %s", err)
		}

		collectedTs := dst.GetCollectedTimeSeries()
		t.Logf("collected timeseries: %d; expected timeseries: %d", len(collectedTs), len(resultExpected))
		if len(collectedTs) != len(resultExpected) {
			t.Fatalf("unexpected number of collected time series; got %d; want %d", len(collectedTs), numOfSeries)
		}

		deleted, err := deleteSeries(matchName, matchValue)
		if err != nil {
			t.Fatalf("cannot delete series: %s", err)
		}
		if deleted != numOfSeries {
			t.Fatalf("unexpected number of deleted series; got %d; want %d", deleted, numOfSeries)
		}
	}

	processFlags()
	vmstorage.Init(promql.ResetRollupResultCacheIfNeeded)
	defer func() {
		vmstorage.Stop()
		if err := os.RemoveAll(storagePath); err != nil {
			log.Fatalf("cannot remove %q: %s", storagePath, err)
		}
	}()

	barpool.Disable(true)
	defer func() {
		barpool.Disable(false)
	}()

	b, err := tsdb.OpenBlock(nil, blockData, nil, nil)
	if err != nil {
		t.Fatalf("cannot open block: %s", err)
	}
	// timestamp is equal to minTime and maxTime from meta.json
	ss, err := readBlock(b, 1737204082361, 1737204302539)
	if err != nil {
		t.Fatalf("cannot read block: %s", err)
	}

	resultExpected, err := prepareExpectedData(ss)
	if err != nil {
		t.Fatalf("cannot prepare expected data: %s", err)
	}

	f("2025-01-18T12:40:00Z", "2025-01-18T12:46:00Z", 2792, resultExpected)
}

func readBlock(b tsdb.BlockReader, timeMin int64, timeMax int64) (storage.SeriesSet, error) {
	minTime, maxTime := b.Meta().MinTime, b.Meta().MaxTime

	if timeMin != 0 {
		minTime = timeMin
	}
	if timeMax != 0 {
		maxTime = timeMax
	}

	q, err := tsdb.NewBlockQuerier(b, minTime, maxTime)
	if err != nil {
		return nil, err
	}
	matchName := "__name__"
	matchValue := ".*"
	ctx := context.Background()
	ss := q.Select(ctx, false, nil, labels.MustNewMatcher(labels.MatchRegexp, matchName, matchValue))
	return ss, nil
}

func prepareExpectedData(ss storage.SeriesSet) ([]vm.TimeSeries, error) {
	var expectedSeriesSet []vm.TimeSeries
	var it chunkenc.Iterator
	for ss.Next() {
		var name string
		var labelPairs []vm.LabelPair
		series := ss.At()

		for _, label := range series.Labels() {
			if label.Name == "__name__" {
				name = label.Value
				continue
			}
			labelPairs = append(labelPairs, vm.LabelPair{
				Name:  label.Name,
				Value: label.Value,
			})
		}
		if name == "" {
			return nil, fmt.Errorf("failed to find `__name__` label in labelset for block")
		}

		var timestamps []int64
		var values []float64
		it = series.Iterator(it)
		for {
			typ := it.Next()
			if typ == chunkenc.ValNone {
				break
			}
			if typ != chunkenc.ValFloat {
				// Skip unsupported values
				continue
			}
			t, v := it.At()
			timestamps = append(timestamps, t)
			values = append(values, v)
		}
		if err := it.Err(); err != nil {
			return nil, err
		}
		ts := vm.TimeSeries{
			Name:       name,
			LabelPairs: labelPairs,
			Timestamps: timestamps,
			Values:     values,
		}
		expectedSeriesSet = append(expectedSeriesSet, ts)
	}
	return expectedSeriesSet, nil
}
