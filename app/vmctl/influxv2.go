package main

import (
	"context"
	"fmt"
	v2 "github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx/v2"
	"log"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

type influxV2Processor struct {
	adapter     *v2.Adapter
	importer    *vm.Importer
	cc          int
	separator   string
	skipDbLabel bool
	promMode    bool
	isVerbose   bool
}

func newInfluxV2Processor(a *v2.Adapter, im *vm.Importer, cc int, separator string, skipDbLabel, promMode, verbose bool) *influxV2Processor {
	if cc < 1 {
		cc = 1
	}
	return &influxV2Processor{
		adapter:     a,
		importer:    im,
		cc:          cc,
		separator:   separator,
		skipDbLabel: skipDbLabel,
		promMode:    promMode,
		isVerbose:   verbose,
	}
}

func (ip *influxV2Processor) run(ctx context.Context) error {
	defer ip.adapter.Close()
	series, err := ip.adapter.Explore(ctx)
	if err != nil {
		return fmt.Errorf("explore query failed: %w", err)
	}
	if len(series) < 1 {
		return fmt.Errorf("found no timeseries to import")
	}
	question := fmt.Sprintf("Found more than %d timeseries to import. Continue?", len(series))
	if !prompt(question) {
		return nil
	}
	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing series"), len(series))
	if err := barpool.Start(); err != nil {
		return err
	}
	defer barpool.Stop()
	seriesCh := make(chan v2.Series)
	errCh := make(chan error)
	ip.importer.ResetStats()
	var wg sync.WaitGroup
	wg.Add(ip.cc)
	for i := 0; i < ip.cc; i++ {
		go func() {
			defer wg.Done()
			for s := range seriesCh {
				if err := ip.do(ctx, s); err != nil {
					errCh <- fmt.Errorf("request failed for %q.%q: %w", s.Measurement, s.Field, err)
					return
				}
				bar.Increment()
			}
		}()
	}
	// any error breaks the import
	for _, s := range series {
		select {
		case infErr := <-errCh:
			return fmt.Errorf("influx error: %w", infErr)
		case vmErr := <-ip.importer.Errors():
			return fmt.Errorf("import process failed: %w", wrapErr(vmErr, ip.isVerbose))
		case seriesCh <- s:
		}
	}
	close(seriesCh)
	wg.Wait()
	ip.importer.Close()
	close(errCh)
	// drain import errors channel
	for vmErr := range ip.importer.Errors() {
		if vmErr.Err != nil {
			return fmt.Errorf("import process failed: %w", wrapErr(vmErr, ip.isVerbose))
		}
	}
	for err := range errCh {
		return fmt.Errorf("import process failed: %w", err)
	}
	log.Println("Import finished!")
	log.Print(ip.importer.Stats())
	return nil
}

func (ip *influxV2Processor) do(ctx context.Context, s v2.Series) error {
	datapoints, err := ip.adapter.Fetch(ctx, s)
	if err != nil {
		return fmt.Errorf(
			"failed to fetch datapoints for measurement '%s' and field '%s': %w",
			s.Measurement, s.Field, err,
		)
	}
	defer func() {
		_ = datapoints.Close()
	}()
	var metric string
	if s.Measurement != "" {
		metric = fmt.Sprintf("%s%s%s", s.Measurement, ip.separator, s.Field)
	} else {
		metric = s.Field
	}
	for datapoints.Next() {
		if err := datapoints.Err(); err != nil {
			return fmt.Errorf(
				"failed to parse Influx record for measurement '%s' and field '%s': %w",
				s.Measurement, s.Field, err,
			)
		}
		datapoint := datapoints.Record()
		labels := make([]vm.LabelPair, 0)
		var containsDBLabel bool
		for name, value := range datapoint.Values() {
			if name == dbLabel {
				containsDBLabel = true
			} else if name == nameLabel && s.Field == valueField && ip.promMode {
				name = fmt.Sprintf("%v", value)
			} else if strings.HasPrefix(name, "_") || name == "result" || name == "table" {
				continue
			}
			labels = append(labels, vm.LabelPair{
				Name:  name,
				Value: fmt.Sprintf("%v", value),
			})
		}
		if !containsDBLabel && !ip.skipDbLabel {
			labels = append(labels, vm.LabelPair{
				Name:  dbLabel,
				Value: ip.adapter.Bucket(),
			})
		}
		ts := vm.TimeSeries{
			Name:       metric,
			LabelPairs: labels,
			Timestamps: []int64{datapoint.Time().UnixMilli()},
			Values:     []float64{datapoint.Value().(float64)},
		}
		if err := ip.importer.Input(&ts); err != nil {
			return err
		}
	}
	return nil
}
