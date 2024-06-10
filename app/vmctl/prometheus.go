package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
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

	// isVerbose enables verbose output
	isVerbose bool
}

func (pp *prometheusProcessor) run() error {
	blocks, err := pp.cl.Explore()
	if err != nil {
		return fmt.Errorf("explore failed: %s", err)
	}
	if len(blocks) < 1 {
		return fmt.Errorf("found no blocks to import")
	}
	question := fmt.Sprintf("Found %d blocks to import. Continue?", len(blocks))
	if !prompt(question) {
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
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, pp.isVerbose))
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
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, pp.isVerbose))
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
	var it chunkenc.Iterator
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
