package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/thanos"
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
	// aggrTypes specifies which aggregate types to import from Thanos downsampled blocks.
	// If empty, downsampled blocks will be skipped.
	aggrTypes []thanos.AggrType
}

func (pp *prometheusProcessor) run(ctx context.Context) error {
	// If aggrTypes is specified, use the new method with AggrChunk support
	if len(pp.aggrTypes) > 0 {
		return pp.runWithAggrSupport(ctx)
	}

	// Original flow for standard Prometheus blocks
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

	if err := pp.processBlocks(blocks); err != nil {
		return fmt.Errorf("migration failed: %s", err)
	}

	log.Println("Import finished!")
	log.Println(pp.im.Stats())
	return nil
}

// runWithAggrSupport runs migration with Thanos AggrChunk support.
// Processes both raw blocks (resolution=0) and downsampled blocks with specified aggregates.
func (pp *prometheusProcessor) runWithAggrSupport(ctx context.Context) error {
	// Reset stats before processing
	pp.im.ResetStats()

	log.Printf("Processing blocks with aggregate types: %v", pp.aggrTypes)

	// Use the first aggregate type to explore blocks (they're the same for all types)
	blocks, err := pp.cl.ExploreWithAggrSupport(pp.aggrTypes[0])
	if err != nil {
		return fmt.Errorf("explore failed: %s", err)
	}
	if len(blocks) < 1 {
		return fmt.Errorf("found no blocks to import")
	}

	// Separate blocks into raw (resolution=0) and downsampled (resolution>0)
	var rawBlocks, downsampledBlocks []prometheus.BlockWithInfo
	for _, block := range blocks {
		if block.Resolution == thanos.ResolutionRaw {
			rawBlocks = append(rawBlocks, block)
		} else {
			downsampledBlocks = append(downsampledBlocks, block)
		}
	}

	log.Printf("Found %d raw blocks and %d downsampled blocks", len(rawBlocks), len(downsampledBlocks))

	question := fmt.Sprintf("Found %d blocks to import (%d raw + %d downsampled with %d aggregate types). Continue?",
		len(blocks), len(rawBlocks), len(downsampledBlocks), len(pp.aggrTypes))
	if !prompt(ctx, question) {
		return nil
	}

	// Process raw blocks first (these don't have aggregate suffixes)
	if len(rawBlocks) > 0 {
		log.Println("Processing raw blocks (resolution=0)...")
		if err := pp.processBlocksWithInfo(rawBlocks, thanos.AggrType(255)); err != nil { // Use special marker for raw
			return fmt.Errorf("migration failed for raw blocks: %s", err)
		}
	}

	// Process downsampled blocks for each aggregate type
	if len(downsampledBlocks) > 0 {
		for _, aggrType := range pp.aggrTypes {
			log.Printf("Processing downsampled blocks with aggregate type: %s", aggrType)

			// Reopen blocks with the appropriate chunk pool for this aggregate type
			blocks, err := pp.cl.ExploreWithAggrSupport(aggrType)
			if err != nil {
				return fmt.Errorf("explore failed for aggr type %s: %s", aggrType, err)
			}

			// Filter only downsampled blocks
			var downsampledOnly []prometheus.BlockWithInfo
			for _, block := range blocks {
				if block.Resolution != thanos.ResolutionRaw {
					downsampledOnly = append(downsampledOnly, block)
				}
			}

			if len(downsampledOnly) < 1 {
				log.Printf("No downsampled blocks found for aggregate type %s, skipping", aggrType)
				continue
			}

			if err := pp.processBlocksWithInfo(downsampledOnly, aggrType); err != nil {
				return fmt.Errorf("migration failed for aggr type %s: %s", aggrType, err)
			}
		}
	}

	// Close importer after all aggregate types are processed
	log.Println("Closing importer and waiting for final flush...")
	pp.im.Close()

	log.Println("Import finished!")
	log.Println(pp.im.Stats())
	return nil
}

func (pp *prometheusProcessor) processBlocksWithInfo(blocks []prometheus.BlockWithInfo, aggrType thanos.AggrType) error {
	log.Printf("Processing %d blocks for aggregate type: %s", len(blocks), aggrType)

	blockReadersCh := make(chan prometheus.BlockWithInfo)
	errCh := make(chan error, pp.cc)

	var processedBlocks, totalSeries, totalSamples uint64
	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(pp.cc)
	for i := 0; i < pp.cc; i++ {
		workerID := i
		go func() {
			defer wg.Done()
			for bi := range blockReadersCh {
				seriesCount, samplesCount, err := pp.doWithAggrTypeAndStats(bi, aggrType)
				if err != nil {
					errCh <- fmt.Errorf("read failed for block %q with aggr %s: %s", bi.Block.Meta().ULID, aggrType, err)
					return
				}

				mu.Lock()
				processedBlocks++
				totalSeries += seriesCount
				totalSamples += samplesCount
				log.Printf("[Worker %d] Block %s: %d series, %d samples | Total: %d/%d blocks, %d series, %d samples",
					workerID, bi.Block.Meta().ULID.String()[:8], seriesCount, samplesCount,
					processedBlocks, len(blocks), totalSeries, totalSamples)
				mu.Unlock()
			}
		}()
	}

	// any error breaks the import
	for _, bi := range blocks {
		select {
		case promErr := <-errCh:
			close(blockReadersCh)
			return fmt.Errorf("prometheus error: %s", promErr)
		case vmErr := <-pp.im.Errors():
			close(blockReadersCh)
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, pp.isVerbose))
		case blockReadersCh <- bi:
		}
	}

	close(blockReadersCh)
	wg.Wait()
	log.Printf("✓ Finished processing %s: %d blocks, %d series, %d samples",
		aggrType, processedBlocks, totalSeries, totalSamples)
	close(errCh)

	// Drain the error channel (non-blocking check for any remaining errors)
	for err := range errCh {
		return fmt.Errorf("import process failed: %s", err)
	}

	// Don't close blocks here - they will be closed automatically when the program exits
	// Closing them manually causes deadlock in Block.Close() waiting for internal goroutines

	return nil
}

// doWithAggrTypeAndStats processes block and returns statistics (series count, samples count, error)
func (pp *prometheusProcessor) doWithAggrTypeAndStats(bi prometheus.BlockWithInfo, aggrType thanos.AggrType) (uint64, uint64, error) {
	ss, err := pp.cl.ReadWithAggrSupport(bi, aggrType)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read block: %s", err)
	}

	var it chunkenc.Iterator
	var seriesCount, samplesCount uint64

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
			return seriesCount, samplesCount, fmt.Errorf("failed to find `__name__` label in labelset for block %v", bi.Block.Meta().ULID)
		}

		// Add resolution and aggregate type suffix to metric name
		name = pp.getMetricNameWithSuffix(name, bi.Resolution, aggrType)

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
			return seriesCount, samplesCount, err
		}

		samplesCount += uint64(len(timestamps))
		seriesCount++

		ts := vm.TimeSeries{
			Name:       name,
			LabelPairs: labels,
			Timestamps: timestamps,
			Values:     values,
		}
		if err := pp.im.Input(&ts); err != nil {
			return seriesCount, samplesCount, err
		}
	}
	return seriesCount, samplesCount, ss.Err()
}

func (pp *prometheusProcessor) doWithAggrType(bi prometheus.BlockWithInfo, aggrType thanos.AggrType) error {
	_, _, err := pp.doWithAggrTypeAndStats(bi, aggrType)
	return err
}

// getMetricNameWithSuffix adds resolution and aggregate type suffix to metric name.
// For example: metric_name -> metric_name:5m:sum
// Special case: aggrType=255 means raw blocks (no suffix)
func (pp *prometheusProcessor) getMetricNameWithSuffix(name string, resolution thanos.ResolutionLevel, aggrType thanos.AggrType) string {
	if resolution == thanos.ResolutionRaw || aggrType == 255 {
		// No suffix for raw data
		return name
	}
	return fmt.Sprintf("%s:%s:%s", name, resolution.String(), aggrType.String())
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

func (pp *prometheusProcessor) processBlocks(blocks []tsdb.BlockReader) error {
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

	return nil
}
