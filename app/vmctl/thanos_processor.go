package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunkenc"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/thanos"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

type thanosProcessor struct {
	cl *thanos.Client
	im *vm.Importer
	cc int

	isVerbose bool
	aggrTypes []thanos.AggrType
}

func (tp *thanosProcessor) run(ctx context.Context) error {
	if len(tp.aggrTypes) == 0 {
		tp.aggrTypes = thanos.AllAggrTypes
	}

	log.Printf("Processing blocks with aggregate types: %v", tp.aggrTypes)

	// Use the first aggregate type to explore blocks (block list is the same for all types)
	blocks, err := tp.cl.Explore(tp.aggrTypes[0])
	if err != nil {
		return fmt.Errorf("explore failed: %s", err)
	}
	if len(blocks) < 1 {
		return fmt.Errorf("found no blocks to import")
	}

	// Separate blocks into raw (resolution=0) and downsampled (resolution>0)
	var rawBlocks, downsampledBlocks []thanos.BlockInfo
	for _, block := range blocks {
		if block.Resolution == thanos.ResolutionRaw {
			rawBlocks = append(rawBlocks, block)
		} else {
			downsampledBlocks = append(downsampledBlocks, block)
		}
	}

	log.Printf("Found %d raw blocks and %d downsampled blocks", len(rawBlocks), len(downsampledBlocks))

	question := fmt.Sprintf("Found %d blocks to import (%d raw + %d downsampled with %d aggregate types). Continue?",
		len(blocks), len(rawBlocks), len(downsampledBlocks), len(tp.aggrTypes))
	if !prompt(ctx, question) {
		return nil
	}

	// Calculate total number of block processing passes for the progress bar:
	// raw blocks are processed once, downsampled blocks are processed once per aggregate type.
	totalPasses := len(rawBlocks) + len(downsampledBlocks)*len(tp.aggrTypes)
	thanosBlocksTotal.Add(totalPasses)
	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing blocks"), totalPasses)
	if err := barpool.Start(); err != nil {
		return err
	}
	defer barpool.Stop()

	tp.im.ResetStats()

	type phaseStats struct {
		name    string
		series  uint64
		samples uint64
	}
	var phases []phaseStats

	// Process raw blocks first (no aggregate suffix)
	if len(rawBlocks) > 0 {
		log.Println("Processing raw blocks (resolution=0)...")
		stats, err := tp.processBlocks(rawBlocks, thanos.AggrTypeNone, bar)
		if err != nil {
			return fmt.Errorf("migration failed for raw blocks: %s", err)
		}
		phases = append(phases, phaseStats{
			name:    "raw",
			series:  stats.series,
			samples: stats.samples,
		})
	}

	// Close blocks from the initial Explore. The querierSeriesSet wrapper
	// has already released all querier read references, so Close won't hang.
	thanos.CloseBlocks(blocks)

	// Process downsampled blocks for each aggregate type.
	// Each type needs its own AggrChunkPool, so we reopen blocks per type.
	for _, aggrType := range tp.aggrTypes {
		if len(downsampledBlocks) < 1 {
			break
		}

		log.Printf("Processing downsampled blocks with aggregate type: %s", aggrType)

		aggrBlocks, err := tp.cl.Explore(aggrType)
		if err != nil {
			return fmt.Errorf("explore failed for aggr type %s: %s", aggrType, err)
		}

		var downsampledOnly []thanos.BlockInfo
		for _, block := range aggrBlocks {
			if block.Resolution != thanos.ResolutionRaw {
				downsampledOnly = append(downsampledOnly, block)
			}
		}

		if len(downsampledOnly) < 1 {
			log.Printf("No downsampled blocks found for aggregate type %s, skipping", aggrType)
			thanos.CloseBlocks(aggrBlocks)
			continue
		}

		log.Printf("Processing %d blocks for aggregate type: %s", len(downsampledOnly), aggrType)
		stats, err := tp.processBlocks(downsampledOnly, aggrType, bar)
		thanos.CloseBlocks(aggrBlocks)
		if err != nil {
			return fmt.Errorf("migration failed for aggr type %s: %s", aggrType, err)
		}
		phases = append(phases, phaseStats{
			name:    aggrType.String(),
			series:  stats.series,
			samples: stats.samples,
		})
	}

	// Print per-phase and total statistics
	var totalSeries, totalSamples uint64
	log.Printf("Migration statistics (%d raw blocks, %d downsampled blocks):", len(rawBlocks), len(downsampledBlocks))
	for _, p := range phases {
		log.Printf("  %s: %d series, %d samples", p.name, p.series, p.samples)
		totalSeries += p.series
		totalSamples += p.samples
	}
	log.Printf("  total: %d series, %d samples", totalSeries, totalSamples)

	// Wait for all buffers to flush
	tp.im.Close()
	// Drain import errors channel
	for vmErr := range tp.im.Errors() {
		if vmErr.Err != nil {
			thanosErrorsTotal.Inc()
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, tp.isVerbose))
		}
	}

	log.Println("Import finished!")
	log.Println(tp.im.Stats())
	return nil
}

// processBlocksStats holds statistics collected during block processing.
type processBlocksStats struct {
	blocks  uint64
	series  uint64
	samples uint64
}

func (tp *thanosProcessor) processBlocks(blocks []thanos.BlockInfo, aggrType thanos.AggrType, bar barpool.Bar) (processBlocksStats, error) {
	blockReadersCh := make(chan thanos.BlockInfo)
	errCh := make(chan error, tp.cc)

	var processedBlocks, totalSeries, totalSamples uint64
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := range tp.cc {
		workerID := i
		wg.Go(func() {
			for bi := range blockReadersCh {
				seriesCount, samplesCount, err := tp.do(bi, aggrType)
				if err != nil {
					thanosErrorsTotal.Inc()
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

				thanosBlocksProcessed.Inc()
				bar.Increment()
			}
		})
	}

	// any error breaks the import
	for _, bi := range blocks {
		select {
		case thanosErr := <-errCh:
			close(blockReadersCh)
			wg.Wait()
			return processBlocksStats{}, fmt.Errorf("thanos error: %s", thanosErr)
		case vmErr := <-tp.im.Errors():
			close(blockReadersCh)
			wg.Wait()
			thanosErrorsTotal.Inc()
			return processBlocksStats{}, fmt.Errorf("import process failed: %s", wrapErr(vmErr, tp.isVerbose))
		case blockReadersCh <- bi:
		}
	}

	close(blockReadersCh)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		return processBlocksStats{}, fmt.Errorf("import process failed: %s", err)
	}

	return processBlocksStats{
		blocks:  processedBlocks,
		series:  totalSeries,
		samples: totalSamples,
	}, nil
}

func (tp *thanosProcessor) do(bi thanos.BlockInfo, aggrType thanos.AggrType) (uint64, uint64, error) {
	ss, err := tp.cl.Read(bi)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read block: %s", err)
	}
	defer ss.Close() // Ensure querier is closed even on early returns

	var it chunkenc.Iterator
	var seriesCount, samplesCount uint64

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
			return seriesCount, samplesCount, fmt.Errorf("failed to find `__name__` label in labelset for block %v", bi.Block.Meta().ULID)
		}

		// Add resolution and aggregate type suffix to metric name for downsampled blocks
		if bi.Resolution != thanos.ResolutionRaw && aggrType != thanos.AggrTypeNone {
			name = fmt.Sprintf("%s:%s:%s", name, bi.Resolution.String(), aggrType.String())
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
			LabelPairs: labelPairs,
			Timestamps: timestamps,
			Values:     values,
		}
		if err := tp.im.Input(&ts); err != nil {
			return seriesCount, samplesCount, err
		}
	}
	return seriesCount, samplesCount, ss.Err()
}

var (
	thanosBlocksTotal     = metrics.NewCounter(`vmctl_thanos_migration_blocks_total`)
	thanosBlocksProcessed = metrics.NewCounter(`vmctl_thanos_migration_blocks_processed`)
	thanosErrorsTotal     = metrics.NewCounter(`vmctl_thanos_migration_errors_total`)
)
