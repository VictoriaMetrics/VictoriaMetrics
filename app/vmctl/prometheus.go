package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

// Runner is an interface for fetching and reading
// snapshot blocks
type Runner interface {
	Explore() ([]tsdb.BlockReader, error)
	Read(context.Context, tsdb.BlockReader) (*prometheus.CloseableSeriesSet, error)
}

type prometheusProcessor struct {
	// Runner fetches and reads
	// snapshot blocks
	cl Runner
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

func (pp *prometheusProcessor) run(ctx context.Context) error {
	blocks, err := pp.cl.Explore()
	if err != nil {
		return fmt.Errorf("explore failed: %s", err)
	}
	if len(blocks) < 1 {
		return fmt.Errorf("found no blocks to import")
	}
	question := fmt.Sprintf("Found %d blocks to import. Continue?", len(blocks))
	if !prompt(ctx, question) {
		return nil
	}

	if err := pp.processBlocks(ctx, blocks); err != nil {
		return fmt.Errorf("migration failed: %s", err)
	}

	log.Println("Import finished!")
	log.Println(pp.im.Stats())
	return nil
}

func (pp *prometheusProcessor) do(ctx context.Context, b tsdb.BlockReader) error {
	css, err := pp.cl.Read(ctx, b)
	if err != nil {
		return fmt.Errorf("failed to read block: %s", err)
	}
	defer func() {
		if err := css.Close(); err != nil {
			log.Printf("cannot close SeriesSet for block: %q : %s\n", b.Meta().ULID, err)
		}
	}()
	ss := css.SeriesSet
	var it chunkenc.Iterator
	for ss.Next() {
		var name string
		var labelPairs []vm.LabelPair
		series := ss.At()

		series.Labels().Range(func(label labels.Label) {
			if label.Name == "__name__" {
				name = label.Value
				return
			}
			labelPairs = append(labelPairs, vm.LabelPair{
				Name:  strings.Clone(label.Name),
				Value: strings.Clone(label.Value),
			})
		})
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
			LabelPairs: labelPairs,
			Timestamps: timestamps,
			Values:     values,
		}
		if err := pp.im.Input(&ts); err != nil {
			return err
		}
	}
	return ss.Err()
}

func (pp *prometheusProcessor) processBlocks(ctx context.Context, blocks []tsdb.BlockReader) error {
	promBlocksTotal.Add(len(blocks))
	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing blocks"), len(blocks))
	if err := barpool.Start(); err != nil {
		return err
	}
	defer barpool.Stop()

	blockReadersCh := make(chan tsdb.BlockReader)
	errCh := make(chan error, pp.cc)
	pp.im.ResetStats()

	var wg sync.WaitGroup
	for range pp.cc {
		wg.Go(func() {
			for br := range blockReadersCh {
				if err := pp.do(ctx, br); err != nil {
					promErrorsTotal.Inc()
					errCh <- fmt.Errorf("cannot read block %q: %s", br.Meta().ULID, err)
					return
				}
				if cb, ok := br.(io.Closer); ok {
					if err := cb.Close(); err != nil {
						errCh <- fmt.Errorf("cannot close block: %q: %w", br.Meta().ULID, err)
					}
				}
				promBlocksProcessed.Inc()
				bar.Increment()
			}
		})
	}
	// any error breaks the import
	for _, br := range blocks {
		select {
		case promErr := <-errCh:
			close(blockReadersCh)
			return fmt.Errorf("prometheus error: %s", promErr)
		case vmErr := <-pp.im.Errors():
			close(blockReadersCh)
			promErrorsTotal.Inc()
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
			promErrorsTotal.Inc()
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, pp.isVerbose))
		}
	}
	for err := range errCh {
		return fmt.Errorf("import process failed: %s", err)
	}

	return nil
}

var (
	promBlocksTotal     = metrics.NewCounter(`vmctl_prometheus_migration_blocks_total`)
	promBlocksProcessed = metrics.NewCounter(`vmctl_prometheus_migration_blocks_processed`)
	promErrorsTotal     = metrics.NewCounter(`vmctl_prometheus_migration_errors_total`)
)
