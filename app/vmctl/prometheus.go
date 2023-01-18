package main

import (
	"flag"
	"fmt"
	"log"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

var (
	promSnapshot         = flag.String("prom-snapshot", "", "Path to Prometheus snapshot. Pls see for details https://www.robustperception.io/taking-snapshots-of-prometheus-data")
	promConcurrency      = flag.Int("prom-concurrency", 1, "Number of concurrently running snapshot readers")
	promFilterTimeStart  = flag.String("prom-filter-time-start", "", "The time filter in RFC3339 format to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'")
	promFilterTimeEnd    = flag.String("prom-filter-time-end", "", "The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'")
	promFilterLabel      = flag.String("prom-filter-label", "", "Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name.")
	promFilterLabelValue = flag.String("prom-filter-label-value", ".*", "Prometheus regular expression to filter label from \"prom-filter-label\" flag.")
)

type prometheusProcessor struct {
	// prometheus client fetches and reads
	// snapshot blocks
	cl *prometheus.Client
	// importer performs import requests
	// for timeseries data returned from
	// snapshot blocks
	im *vm.Importer
	// cc stands for concurrency
	// and defines number of concurrently
	// running snapshot block readers
	cc int
}

func (pp *prometheusProcessor) run(silent, verbose bool) error {
	blocks, err := pp.cl.Explore()
	if err != nil {
		return fmt.Errorf("explore failed: %s", err)
	}
	if len(blocks) < 1 {
		return fmt.Errorf("found no blocks to import")
	}
	question := fmt.Sprintf("Found %d blocks to import. Continue?", len(blocks))
	if !silent && !prompt(question) {
		return nil
	}

	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing blocks"), len(blocks))

	if err := barpool.Start(); err != nil {
		return err
	}
	defer barpool.Stop()

	blockReadersCh := make(chan tsdb.BlockReader)
	errCh := make(chan error, pp.cc)
	pp.im.ResetStats()

	var wg sync.WaitGroup
	wg.Add(pp.cc)
	for i := 0; i < pp.cc; i++ {
		go func() {
			defer wg.Done()
			for br := range blockReadersCh {
				if err := pp.do(br); err != nil {
					errCh <- fmt.Errorf("read failed for block %q: %s", br.Meta().ULID, err)
					return
				}
				bar.Increment()
			}
		}()
	}
	// any error breaks the import
	for _, br := range blocks {
		select {
		case promErr := <-errCh:
			close(blockReadersCh)
			return fmt.Errorf("prometheus error: %s", promErr)
		case vmErr := <-pp.im.Errors():
			close(blockReadersCh)
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, verbose))
		case blockReadersCh <- br:
		}
	}

	close(blockReadersCh)
	wg.Wait()
	// wait for all buffers to flush
	pp.im.Close()
	close(errCh)
	// drain import errors channel
	for vmErr := range pp.im.Errors() {
		if vmErr.Err != nil {
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, verbose))
		}
	}
	for err := range errCh {
		return fmt.Errorf("import process failed: %s", err)
	}

	log.Println("Import finished!")
	log.Print(pp.im.Stats())
	return nil
}

func (pp *prometheusProcessor) do(b tsdb.BlockReader) error {
	ss, err := pp.cl.Read(b)
	if err != nil {
		return fmt.Errorf("failed to read block: %s", err)
	}
	for ss.Next() {
		var name string
		var labels []vm.LabelPair
		series := ss.At()

		for _, label := range series.Labels() {
			if label.Name == "__name__" {
				name = label.Value
				continue
			}
			labels = append(labels, vm.LabelPair{
				Name:  label.Name,
				Value: label.Value,
			})
		}
		if name == "" {
			return fmt.Errorf("failed to find `__name__` label in labelset for block %v", b.Meta().ULID)
		}

		var timestamps []int64
		var values []float64
		it := series.Iterator()
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
			return err
		}
		ts := vm.TimeSeries{
			Name:       name,
			LabelPairs: labels,
			Timestamps: timestamps,
			Values:     values,
		}
		if err := pp.im.Input(&ts); err != nil {
			return err
		}
	}
	return ss.Err()
}

func prometheusImport(importer *vm.Importer) flagutil.Action {
	return func(args []string) {
		fmt.Println("Prometheus import mode")

		if *promSnapshot == "" {
			logger.Fatalf("flag --prom-snapshot should contain path to Prometheus snapshot")
		}

		err := flagutil.SetFlagsFromEnvironment()
		if err != nil {
			logger.Fatalf("error set flags from environment variables: %s", err)
		}

		vmCfg := initConfigVM()
		importer, err = vm.NewImporter(vmCfg)
		if err != nil {
			logger.Fatalf("failed to create VM importer: %s", err)
		}

		promCfg := prometheus.Config{
			Snapshot: *promSnapshot,
			Filter: prometheus.Filter{
				TimeMin:    *promFilterTimeStart,
				TimeMax:    *promFilterTimeEnd,
				Label:      *promFilterLabel,
				LabelValue: *promFilterLabelValue,
			},
		}
		cl, err := prometheus.NewClient(promCfg)
		if err != nil {
			logger.Fatalf("failed to create prometheus client: %s", err)
		}
		pp := prometheusProcessor{
			cl: cl,
			im: importer,
			cc: *promConcurrency,
		}
		if err := pp.run(*globalSilent, *globalVerbose); err != nil {
			logger.Fatalf("error run prometheus import process: %s", err)
		}
	}
}
