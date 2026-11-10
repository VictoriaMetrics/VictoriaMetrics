package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vminsertapi"
)

var (
	numMetrics  = flag.Int("datagen.metrics", 1000, "number of metrics emitted by a host")
	numHosts    = flag.Int("datagen.hosts", 100, "number of hosts that emit metics")
	churnRate   = flag.Float64("datagen.churn", 1, "the percentage of hosts that change their entire metric set every day")
	concurrency = flag.Int("datagen.concurrency", cgroup.AvailableCPUs(), "number of concurrent ingestion workers")
)

func GenerateData(api vminsertapi.API, retention time.Duration) {
	begin := time.Now()

	if *churnRate < 0 || *churnRate > 1 {
		logger.Fatalf("-datagen.churnRate must be in [0, 1] interval")
	}

	const msecPerDay = 24 * 3600 * 1000
	numDays := retention.Milliseconds() / msecPerDay
	rowsTotal := uint64(*numMetrics * *numHosts * int(numDays))
	var rowsGenerated atomic.Uint64

	logger.Infof("datagen: loading %d rows (%d metrics x %d hosts x %d days), concurrency=%d", rowsTotal, *numMetrics, *numHosts, numDays, *concurrency)

	doneCh := make(chan struct{})

	go func() {
		tt := time.NewTicker(time.Second)
		defer tt.Stop()
		for {
			select {
			case <-doneCh:
				return
			case <-tt.C:
				progress := rowsGenerated.Load()
				percent := progress * 100 / rowsTotal
				logger.Infof("datagen: loaded %d out of %d (%d%%)", progress, rowsTotal, percent)
			}
		}
	}()

	mrsCh := make(chan []storage.MetricRow, *concurrency)

	var wg sync.WaitGroup
	for range *concurrency {
		wg.Add(1)
		wg.Go(func() {
			defer wg.Done()
			for mrs := range mrsCh {
				if err := api.WriteRows(mrs); err != nil {
					logger.Panicf("could not write rows: %s", err)
				}
				rowsGenerated.Add(uint64(len(mrs)))
			}
		})
	}

	shardLabelValue, err := os.Hostname()
	if err != nil {
		logger.Fatalf("could not get host name: %s", err)
	}
	today := (time.Now().UnixMilli() / msecPerDay) * msecPerDay
	maxRotationHost := int(float64(*numHosts) * *churnRate)
	for d := range numDays {
		timestamp := today - int64(d)*msecPerDay
		dateStr := time.UnixMilli(timestamp).Format(time.DateOnly)
		for h := range *numHosts {
			hostLabelValue := fmt.Sprintf("host-%09d", h)
			dateLabelValue := dateStr
			if h >= maxRotationHost {
				dateLabelValue = "1970-01-01"
			}
			mrs := make([]storage.MetricRow, *numMetrics)
			for m := range *numMetrics {
				labels := []prompb.Label{
					{Name: "__name__", Value: fmt.Sprintf("metric_%09d", m)},
					{Name: "shard", Value: shardLabelValue},
					{Name: "host", Value: hostLabelValue},
					{Name: "date", Value: dateLabelValue},
				}
				mrs[m].MetricNameRaw = storage.MarshalMetricNameRaw(nil, 0, 0, labels)
				mrs[m].Timestamp = timestamp
			}
			mrsCh <- mrs
		}
	}

	close(mrsCh)
	wg.Wait()
	close(doneCh)

	end := time.Now()
	logger.Infof("datagen completed in %s", end.Sub(begin))
}
