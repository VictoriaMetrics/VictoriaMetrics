package main

import (
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/cheggaaa/pb/v3"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

type influxProcessor struct {
	ic                 *influx.Client
	im                 *vm.Importer
	cc                 int
	separator          string
	skipDbLabel        bool
	promMode           bool
	isSilent           bool
	isVerbose          bool
	disableProgressBar bool
}

type InfluxProcessorOption func(*influxProcessor)

func newInfluxProcessor(opt ...InfluxProcessorOption) *influxProcessor {
	ip := &influxProcessor{}
	for _, fn := range opt {
		fn(ip)
	}
	return ip
}

// WithInfluxClient sets Influx client for processor
func WithInfluxClient(ic *influx.Client) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		ip.ic = ic
	}
}

// WithImporter sets importer for processor
func WithImporter(im *vm.Importer) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		ip.im = im
	}
}

// WithConcurrency sets concurrency for processor
func WithConcurrency(cc int) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		if cc < 1 {
			cc = 1
		}

		ip.cc = cc
	}
}

// WithSeparator sets separator for processor
func WithSeparator(separator string) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		ip.separator = separator
	}
}

// WithSkipDbLabel sets skip Label for processor
func WithSkipDbLabel(skipDbLabel bool) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		ip.skipDbLabel = skipDbLabel
	}
}

// WithPromMode sets prometheus mode for processor
func WithPromMode(promMode bool) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		ip.promMode = promMode
	}
}

// WithSilent sets silent mode for processor
func WithSilent(silent bool) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		ip.isSilent = silent
	}
}

// WithVerbose sets verbose mode for processor
func WithVerbose(verbose bool) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		ip.isVerbose = verbose
	}
}

// WithDisableProgressBar sets disable progress bar for processor
func WithDisableProgressBar(disableProgressBar bool) InfluxProcessorOption {
	return func(ip *influxProcessor) {
		ip.disableProgressBar = disableProgressBar
	}
}

func (ip *influxProcessor) run() error {
	series, err := ip.ic.Explore()
	if err != nil {
		return fmt.Errorf("explore query failed: %s", err)
	}
	if len(series) < 1 {
		return fmt.Errorf("found no timeseries to import")
	}

	question := fmt.Sprintf("Found %d timeseries to import. Continue?", len(series))
	if !ip.isSilent && !prompt(question) {
		return nil
	}

	var bar *pb.ProgressBar
	if !ip.isSilent && !ip.disableProgressBar {
		bar = barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing series"), len(series))
		if err := barpool.Start(); err != nil {
			return err
		}
		defer barpool.Stop()
	}

	seriesCh := make(chan *influx.Series)
	errCh := make(chan error)
	ip.im.ResetStats()

	var wg sync.WaitGroup
	wg.Add(ip.cc)
	for i := 0; i < ip.cc; i++ {
		go func() {
			defer wg.Done()
			for s := range seriesCh {
				if err := ip.do(s); err != nil {
					errCh <- fmt.Errorf("request failed for %q.%q: %s", s.Measurement, s.Field, err)
					return
				}
				if bar != nil {
					bar.Increment()
				}
			}
		}()
	}

	// any error breaks the import
	for _, s := range series {
		select {
		case infErr := <-errCh:
			return fmt.Errorf("influx error: %s", infErr)
		case vmErr := <-ip.im.Errors():
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, ip.isVerbose))
		case seriesCh <- s:
		}
	}

	close(seriesCh)
	wg.Wait()
	ip.im.Close()
	close(errCh)
	// drain import errors channel
	for vmErr := range ip.im.Errors() {
		if vmErr.Err != nil {
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

const dbLabel = "db"
const nameLabel = "__name__"
const valueField = "value"

func (ip *influxProcessor) do(s *influx.Series) error {
	cr, err := ip.ic.FetchDataPoints(s)
	if err != nil {
		return fmt.Errorf("failed to fetch datapoints: %s", err)
	}
	defer func() {
		_ = cr.Close()
	}()
	var name string
	if s.Measurement != "" {
		name = fmt.Sprintf("%s%s%s", s.Measurement, ip.separator, s.Field)
	} else {
		name = s.Field
	}

	labels := make([]vm.LabelPair, len(s.LabelPairs))
	var containsDBLabel bool
	for i, lp := range s.LabelPairs {
		if lp.Name == dbLabel {
			containsDBLabel = true
		} else if lp.Name == nameLabel && s.Field == valueField && ip.promMode {
			name = lp.Value
		}
		labels[i] = vm.LabelPair{
			Name:  lp.Name,
			Value: lp.Value,
		}
	}
	if !containsDBLabel && !ip.skipDbLabel {
		labels = append(labels, vm.LabelPair{
			Name:  dbLabel,
			Value: ip.ic.Database(),
		})
	}

	for {
		time, values, err := cr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		// skip empty results
		if len(time) < 1 {
			continue
		}
		ts := vm.TimeSeries{
			Name:       name,
			LabelPairs: labels,
			Timestamps: time,
			Values:     values,
		}
		if err := ip.im.Input(&ts); err != nil {
			return err
		}
	}
}
