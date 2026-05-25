package main

// influx2.go is the migration processor for InfluxDB v2.
// It sits between the influx2.Client (which talks to InfluxDB) and vm.Importer
// (which pushes data into VictoriaMetrics). Its only job is to orchestrate:
// discover series → fan out workers → fetch → convert → push.

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx2"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

// influx2Processor holds everything the migration loop needs.
// It's intentionally thin — all InfluxDB knowledge lives in influx2.Client,
// all VictoriaMetrics knowledge lives in vm.Importer.
type influx2Processor struct {
	ic              *influx2.Client  // speaks to InfluxDB v2
	im              *vm.Importer     // pushes into VictoriaMetrics
	cc              int              // number of concurrent fetch workers
	separator       string           // joins measurement + field into a metric name, e.g. "cpu_usage_idle"
	skipBucketLabel bool             // when true, don't add a "bucket=mybucket" label
	isVerbose       bool             // print full error payloads on failure
}

func newInflux2Processor(ic *influx2.Client, im *vm.Importer, cc int, separator string, skipBucketLabel, verbose bool) *influx2Processor {
	if cc < 1 {
		// Always have at least one worker — zero concurrency makes no sense.
		cc = 1
	}
	return &influx2Processor{
		ic:              ic,
		im:              im,
		cc:              cc,
		separator:       separator,
		skipBucketLabel: skipBucketLabel,
		isVerbose:       verbose,
	}
}

// run is the main migration loop. It:
//  1. Discovers all series in the bucket (Explore).
//  2. Asks the user to confirm before starting (safety check on large datasets).
//  3. Fans out cc worker goroutines that each pull series off a channel and call do().
//  4. Feeds series into the channel while listening for errors from workers or the importer.
//  5. Drains everything cleanly on success or bails immediately on the first error.
func (ip *influx2Processor) run(ctx context.Context) error {
	series, err := ip.ic.Explore()
	if err != nil {
		return fmt.Errorf("explore failed: %s", err)
	}
	if len(series) < 1 {
		return fmt.Errorf("found no timeseries to import")
	}

	// Prompt before starting — migrating a million series by accident is no fun.
	question := fmt.Sprintf("Found %d timeseries to import. Continue?", len(series))
	if !prompt(ctx, question) {
		return nil
	}

	// Track total series count in the /metrics endpoint so operators can
	// monitor migration progress via Grafana or similar dashboards.
	influx2SeriesTotal.Add(len(series))

	// barpool renders a terminal progress bar — same pattern used by all
	// other vmctl processors (influx, remoteread, etc.) for consistency.
	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing series"), len(series))
	if err := barpool.Start(); err != nil {
		return err
	}
	defer barpool.Stop()

	// seriesCh delivers work to the pool of cc goroutines.
	// It's unbuffered so the main loop blocks until a worker picks up each item —
	// this provides natural backpressure and keeps memory bounded.
	seriesCh := make(chan *influx2.Series)

	// errCh carries errors back from workers to the main goroutine.
	// Workers send one error then exit, so a small buffer is fine here.
	errCh := make(chan error)

	ip.im.ResetStats()

	var wg sync.WaitGroup
	for range ip.cc {
		// Each goroutine drains seriesCh until it's closed.
		// On error it sends to errCh and returns — other workers keep running
		// until the main loop (below) detects the error and stops feeding seriesCh.
		wg.Go(func() {
			for s := range seriesCh {
				if err := ip.do(s); err != nil {
					influx2ErrorsTotal.Inc()
					errCh <- fmt.Errorf("request failed for %q.%q: %s", s.Measurement, s.Field, err)
					return
				}
				influx2SeriesProcessed.Inc()
				bar.Increment()
			}
		})
	}

	// Feed series into the channel. The select also watches for errors from
	// workers (errCh) and from the importer (ip.im.Errors()) so we bail
	// immediately instead of continuing to push data when something is broken.
	for _, s := range series {
		select {
		case infErr := <-errCh:
			return fmt.Errorf("influx2 error: %s", infErr)
		case vmErr := <-ip.im.Errors():
			influx2ErrorsTotal.Inc()
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, ip.isVerbose))
		case seriesCh <- s:
			// Worker picked it up — keep going.
		}
	}

	// No more series to send. Close the channel so workers know to exit
	// after finishing whatever they're currently processing.
	close(seriesCh)
	wg.Wait()

	// Close the importer to flush any buffered batches before we check for errors.
	ip.im.Close()
	close(errCh)

	// Drain any errors that arrived after the main loop finished.
	// These are errors that workers sent just as we closed seriesCh.
	for vmErr := range ip.im.Errors() {
		if vmErr.Err != nil {
			influx2ErrorsTotal.Inc()
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, ip.isVerbose))
		}
	}
	for err := range errCh {
		return fmt.Errorf("import process failed: %s", err)
	}

	log.Println("Import finished!")
	log.Print(ip.im.Stats())
	return nil
}

// bucketLabel is the label name we attach to every migrated series so you can
// tell which InfluxDB bucket the data came from after it's in VictoriaMetrics.
// This is the v2 equivalent of v1's "db" label.
const bucketLabel = "bucket"

// do fetches all data points for one Series and pushes them into VictoriaMetrics.
// It runs in one of the cc worker goroutines.
func (ip *influx2Processor) do(s *influx2.Series) error {
	cr, err := ip.ic.FetchDataPoints(s)
	if err != nil {
		return fmt.Errorf("failed to fetch datapoints: %s", err)
	}
	// Always close the response to release the HTTP connection back to the pool.
	defer func() { _ = cr.Close() }()

	// Build the metric name by joining measurement and field with the separator.
	// e.g. measurement="cpu", field="usage_idle", separator="_" → "cpu_usage_idle"
	// If there's no measurement (some InfluxDB setups omit it), just use the field name.
	var name string
	if s.Measurement != "" {
		name = fmt.Sprintf("%s%s%s", s.Measurement, ip.separator, s.Field)
	} else {
		name = s.Field
	}

	// Convert the series's tag pairs into vm.LabelPair format.
	// We also check if the user's data already has a "bucket" tag so we don't
	// add a duplicate label if it's already there.
	labels := make([]vm.LabelPair, len(s.LabelPairs))
	var containsBucketLabel bool
	for i, lp := range s.LabelPairs {
		if lp.Name == bucketLabel {
			containsBucketLabel = true
		}
		labels[i] = vm.LabelPair{
			Name:  lp.Name,
			Value: lp.Value,
		}
	}

	// Attach the bucket label unless the user opted out (--influx2-skip-bucket-label)
	// or the data already has one. This helps identify data origin after migration.
	if !containsBucketLabel && !ip.skipBucketLabel {
		labels = append(labels, vm.LabelPair{
			Name:  bucketLabel,
			Value: ip.ic.Bucket(),
		})
	}

	// Pull chunks of (timestamps, values) from the streaming response
	// and feed each batch into the importer. We loop until Next() returns
	// empty slices, signalling the stream is fully consumed.
	for {
		timestamps, values, err := cr.Next()
		if err != nil {
			return err
		}
		// Empty batch means the stream is done for this series.
		if len(timestamps) == 0 {
			return nil
		}
		ts := vm.TimeSeries{
			Name:       name,
			LabelPairs: labels,
			Timestamps: timestamps,
			Values:     values,
		}
		if err := ip.im.Input(&ts); err != nil {
			return err
		}
	}
}

// Prometheus-style counters exposed at /metrics so the migration can be
// monitored externally. Named influx2_* to distinguish from the v1 influx_* counters.
var (
	influx2SeriesTotal     = metrics.NewCounter(`vmctl_influx2_migration_series_total`)
	influx2SeriesProcessed = metrics.NewCounter(`vmctl_influx2_migration_series_processed`)
	influx2ErrorsTotal     = metrics.NewCounter(`vmctl_influx2_migration_errors_total`)
)
