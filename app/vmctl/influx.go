package main

import (
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

type influxProcessor struct {
	ic          *influx.Client
	im          *vm.Importer
	cc          int
	separator   string
	skipDbLabel bool
	promMode    bool
	isVerbose   bool
}

func newInfluxProcessor(ic *influx.Client, im *vm.Importer, cc int, separator string, skipDbLabel, promMode, verbose bool) *influxProcessor {
	if cc < 1 {
		cc = 1
	}

	return &influxProcessor{
		ic:          ic,
		im:          im,
		cc:          cc,
		separator:   separator,
		skipDbLabel: skipDbLabel,
		promMode:    promMode,
		isVerbose:   verbose,
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
	if !prompt(question) {
		return nil
	}

	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing series"), len(series))
	if err := barpool.Start(); err != nil {
		return err
	}
	defer barpool.Stop()

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
				bar.Increment()
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
