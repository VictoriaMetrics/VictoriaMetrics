package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

// influxProcessor drives migration from any Source (InfluxDB v1 or v2) into VictoriaMetrics.
// All protocol-specific knowledge lives in the Source implementation; this file
// only orchestrates: discover → fan-out workers → fetch → convert → push.
type influxProcessor struct {
	src       influx.Source
	im        *vm.Importer
	cc        int
	separator string
	skipLabel bool // skip attaching the source label ("db" for v1, "bucket" for v2)
	promMode  bool // v1 only: rewrite metric name from __name__ label when field == "value"
	isVerbose bool

	seriesTotal     *metrics.Counter
	seriesProcessed *metrics.Counter
	errorsTotal     *metrics.Counter
}

func newInfluxProcessor(
	src influx.Source,
	im *vm.Importer,
	cc int,
	separator string,
	skipLabel bool,
	promMode bool,
	verbose bool,
	seriesTotal, seriesProcessed, errorsTotal *metrics.Counter,
) *influxProcessor {
	if cc < 1 {
		cc = 1
	}
	return &influxProcessor{
		src:             src,
		im:              im,
		cc:              cc,
		separator:       separator,
		skipLabel:       skipLabel,
		promMode:        promMode,
		isVerbose:       verbose,
		seriesTotal:     seriesTotal,
		seriesProcessed: seriesProcessed,
		errorsTotal:     errorsTotal,
	}
}

func (ip *influxProcessor) run(ctx context.Context) error {
	series, err := ip.src.Explore()
	if err != nil {
		return fmt.Errorf("explore failed: %s", err)
	}
	if len(series) < 1 {
		return fmt.Errorf("found no timeseries to import")
	}

	question := fmt.Sprintf("Found %d timeseries to import. Continue?", len(series))
	if !prompt(ctx, question) {
		return nil
	}

	ip.seriesTotal.Add(len(series))
	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing series"), len(series))
	if err := barpool.Start(); err != nil {
		return err
	}
	defer barpool.Stop()

	seriesCh := make(chan *influx.Series)
	errCh := make(chan error)
	ip.im.ResetStats()

	var wg sync.WaitGroup
	for range ip.cc {
		wg.Go(func() {
			for s := range seriesCh {
				if err := ip.do(s); err != nil {
					ip.errorsTotal.Inc()
					errCh <- fmt.Errorf("request failed for %q.%q: %s", s.Measurement, s.Field, err)
					return
				}
				ip.seriesProcessed.Inc()
				bar.Increment()
			}
		})
	}

	for _, s := range series {
		select {
		case infErr := <-errCh:
			return fmt.Errorf("influx error: %s", infErr)
		case vmErr := <-ip.im.Errors():
			ip.errorsTotal.Inc()
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, ip.isVerbose))
		case seriesCh <- s:
		}
	}

	close(seriesCh)
	wg.Wait()
	ip.im.Close()
	close(errCh)

	for vmErr := range ip.im.Errors() {
		if vmErr.Err != nil {
			ip.errorsTotal.Inc()
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

const (
	dbLabel    = "db"
	nameLabel  = "__name__"
	valueField = "value"
)

func (ip *influxProcessor) do(s *influx.Series) error {
	cr, err := ip.src.FetchDataPoints(s)
	if err != nil {
		return fmt.Errorf("failed to fetch datapoints: %s", err)
	}
	defer func() { _ = cr.Close() }()

	var name string
	if s.Measurement != "" {
		name = fmt.Sprintf("%s%s%s", s.Measurement, ip.separator, s.Field)
	} else {
		name = s.Field
	}

	labelName, labelValue := ip.src.Label()
	labels := make([]vm.LabelPair, len(s.LabelPairs))
	var containsLabel bool
	for i, lp := range s.LabelPairs {
		if lp.Name == labelName {
			containsLabel = true
		} else if ip.promMode && lp.Name == nameLabel && s.Field == valueField {
			name = lp.Value
		}
		labels[i] = vm.LabelPair{Name: lp.Name, Value: lp.Value}
	}
	if !containsLabel && !ip.skipLabel {
		labels = append(labels, vm.LabelPair{Name: labelName, Value: labelValue})
	}

	for {
		timestamps, values, err := cr.Next()
		if err != nil {
			return err
		}
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

// Per-source Prometheus-style counters exposed at /metrics.
// v1 keeps vmctl_influx_* names; v2 keeps vmctl_influx2_* names so existing
// dashboards that watch either counter set are unaffected by this refactor.
var (
	influxSeriesTotal     = metrics.NewCounter(`vmctl_influx_migration_series_total`)
	influxSeriesProcessed = metrics.NewCounter(`vmctl_influx_migration_series_processed`)
	influxErrorsTotal     = metrics.NewCounter(`vmctl_influx_migration_errors_total`)

	influx2SeriesTotal     = metrics.NewCounter(`vmctl_influx2_migration_series_total`)
	influx2SeriesProcessed = metrics.NewCounter(`vmctl_influx2_migration_series_processed`)
	influx2ErrorsTotal     = metrics.NewCounter(`vmctl_influx2_migration_errors_total`)
)
