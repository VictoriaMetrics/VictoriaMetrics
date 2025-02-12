package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/prometheus/prometheus/tsdb"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/mimir"
	remote_read_integration "github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/testdata/servers_integration_test"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
)

const (
	testMimirData  = "testdata/mimir-tsdb"
	tenantID       = "anonymous"
	mimirBlockData = "./testdata/mimir-tsdb/anonymous/01JFJBS3YP1SHZ3PJQ6HK76EC3"
)

// This test simulates close process if user abort it
func TestMimirProcessorRun(t *testing.T) {

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
			BatchSize:   100,
		}

		ctx := context.Background()
		importer, err := vm.NewImporter(ctx, importerCfg)
		if err != nil {
			t.Fatalf("cannot create importer: %s", err)
		}
		defer importer.Close()

		matchName := "__name__"
		matchValue := ".*"
		filter := mimir.Filter{
			TimeMin:    startStr,
			TimeMax:    endStr,
			Label:      matchName,
			LabelValue: matchValue,
		}

		dir, err := os.Getwd()
		if err != nil {
			t.Fatalf("cannot get current working directory: %s", err)
		}

		path := fmt.Sprintf("fs://%s/%s", dir, testMimirData)
		runnner, err := mimir.NewClient(mimir.Config{
			TenantID: tenantID,
			Path:     path,
			Filter:   filter,
		})
		if err != nil {
			t.Fatalf("cannot create mimir client: %s", err)
		}
		p := &prometheusProcessor{
			cl: runnner,
			im: importer,
			cc: 1,
		}

		if err := p.run(ctx); err != nil {
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

	b, err := tsdb.OpenBlock(nil, mimirBlockData, nil, nil)
	if err != nil {
		t.Fatalf("cannot open block: %s", err)
	}
	// timestamp is equal to minTime and maxTime from meta.json
	ss, err := readBlock(b, 1734709200000, 1734709320000)
	if err != nil {
		t.Fatalf("cannot read block: %s", err)
	}

	resultExpected, err := prepareExpectedData(ss)
	if err != nil {
		t.Fatalf("cannot prepare expected data: %s", err)
	}
	// timestamp is equal to minTime and maxTime from meta.json
	f("2024-12-20T15:40:00Z", "2025-01-18T12:42:00Z", 100, resultExpected)
}
