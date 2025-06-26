package storage

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

// mergeBlockStreams merges bsrs into bsw and updates ph.
//
// mergeBlockStreams returns immediately if stopCh is closed.
//
// rowsMerged is atomically updated with the number of merged rows during the merge.
func mergeBlockStreams(ph *partHeader, bsw *blockStreamWriter, bsrs []*blockStreamReader, stopCh <-chan struct{}, dmis *uint64set.Set, retentionDeadline int64,
	rowsMerged, rowsDeleted *atomic.Uint64, useSparseCache bool) error {
	ph.Reset()

	bsm := bsmPool.Get().(*blockStreamMerger)
	bsm.Init(bsrs, retentionDeadline, useSparseCache)
	err := mergeBlockStreamsInternal(ph, bsw, bsm, stopCh, dmis, rowsMerged, rowsDeleted)
	bsm.reset()
	bsmPool.Put(bsm)
	bsw.MustClose()
	if err == nil {
		return nil
	}
	return fmt.Errorf("cannot merge %d streams: %s: %w", len(bsrs), bsrs, err)
}

var bsmPool = &sync.Pool{
	New: func() any {
		return &blockStreamMerger{}
	},
}

var errForciblyStopped = fmt.Errorf("forcibly stopped")

func mergeBlockStreamsInternal(ph *partHeader, bsw *blockStreamWriter, bsm *blockStreamMerger, stopCh <-chan struct{}, dmis *uint64set.Set, rowsMerged, rowsDeleted *atomic.Uint64) error {
	pendingBlockIsEmpty := true
	pendingBlock := getBlock()
	defer putBlock(pendingBlock)
	tmpBlock := getBlock()
	defer putBlock(tmpBlock)

	// Use local variables for tracking the number of merged and deleted rows
	// and periodically propagate the collected stats to the caller, so it could be reflected in the exposed metrics.
	//
	// This minimizes expensive updates of rowsMerged and rowsDeleted vars from concurrently running goroutines,
	// and improves concurrent merge scalability on multi-CPU systems - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8682 .
	var updateStatsDeadline uint64
	var localRowsMerged, localRowsDeleted uint64
	updateStats := func() {
		rowsDeleted.Add(localRowsDeleted)
		localRowsDeleted = 0

		rowsMerged.Add(localRowsMerged)
		localRowsMerged = 0
	}
	defer updateStats()

	for bsm.NextBlock() {
		ct := fasttime.UnixTimestamp()
		if ct > updateStatsDeadline {
			updateStats()
			// Update the external stats once per second
			updateStatsDeadline = ct + 1
		}

		select {
		case <-stopCh:
			return errForciblyStopped
		default:
		}

		b := bsm.Block
		if dmis.Has(b.bh.TSID.MetricID) {
			// Skip blocks for deleted metrics.
			localRowsDeleted += uint64(b.bh.RowsCount)
			continue
		}
		retentionDeadline := bsm.getRetentionDeadline(&b.bh)
		if b.bh.MaxTimestamp < retentionDeadline {
			// Skip blocks out of the given retention.
			localRowsDeleted += uint64(b.bh.RowsCount)
			continue
		}
		if pendingBlockIsEmpty {
			// Load the next block if pendingBlock is empty.
			pendingBlock.CopyFrom(b)
			pendingBlockIsEmpty = false
			continue
		}

		// Verify whether pendingBlock may be merged with b (the current block).
		if pendingBlock.bh.TSID.MetricID != b.bh.TSID.MetricID {
			// Fast path - blocks belong to distinct time series.
			// Write the pendingBlock and then deal with b.
			if b.bh.TSID.Less(&pendingBlock.bh.TSID) {
				logger.Panicf("BUG: the next TSID=%+v is smaller than the current TSID=%+v", &b.bh.TSID, &pendingBlock.bh.TSID)
			}
			bsw.WriteExternalBlock(pendingBlock, ph, &localRowsMerged)
			pendingBlock.CopyFrom(b)
			continue
		}
		if pendingBlock.tooBig() && pendingBlock.bh.MaxTimestamp <= b.bh.MinTimestamp {
			// Fast path - pendingBlock is too big and it doesn't overlap with b.
			// Write the pendingBlock and then deal with b.
			bsw.WriteExternalBlock(pendingBlock, ph, &localRowsMerged)
			pendingBlock.CopyFrom(b)
			continue
		}

		// Slow path - pendingBlock and b belong to the same time series,
		// so they must be merged.
		if err := unmarshalAndCalibrateScale(pendingBlock, b); err != nil {
			return fmt.Errorf("cannot unmarshal and calibrate scale for blocks to be merged: %w", err)
		}
		tmpBlock.Reset()
		tmpBlock.bh.TSID = b.bh.TSID
		tmpBlock.bh.Scale = b.bh.Scale
		tmpBlock.bh.PrecisionBits = min(pendingBlock.bh.PrecisionBits, b.bh.PrecisionBits)
		mergeBlocks(tmpBlock, pendingBlock, b, retentionDeadline, &localRowsDeleted)
		if len(tmpBlock.timestamps) <= maxRowsPerBlock {
			// More entries may be added to tmpBlock. Swap it with pendingBlock,
			// so more entries may be added to pendingBlock on the next iteration.
			if len(tmpBlock.timestamps) > 0 {
				tmpBlock.fixupTimestamps()
			} else {
				pendingBlockIsEmpty = true
			}
			pendingBlock, tmpBlock = tmpBlock, pendingBlock
			continue
		}

		// Write the first maxRowsPerBlock of tmpBlock.timestamps to bsw,
		// leave the rest in pendingBlock.
		tmpBlock.nextIdx = maxRowsPerBlock
		pendingBlock.CopyFrom(tmpBlock)
		pendingBlock.fixupTimestamps()
		tmpBlock.nextIdx = 0
		tmpBlock.timestamps = tmpBlock.timestamps[:maxRowsPerBlock]
		tmpBlock.values = tmpBlock.values[:maxRowsPerBlock]
		tmpBlock.fixupTimestamps()
		bsw.WriteExternalBlock(tmpBlock, ph, &localRowsMerged)
	}
	if err := bsm.Error(); err != nil {
		return fmt.Errorf("cannot read block to be merged: %w", err)
	}
	if !pendingBlockIsEmpty {
		bsw.WriteExternalBlock(pendingBlock, ph, &localRowsMerged)
	}
	return nil
}

// mergeBlocks merges ib1 and ib2 to ob.
func mergeBlocks(ob, ib1, ib2 *Block, retentionDeadline int64, rowsDeleted *uint64) {
	ib1.assertMergeable(ib2)
	ib1.assertUnmarshaled()
	ib2.assertUnmarshaled()

	skipSamplesOutsideRetention(ib1, retentionDeadline, rowsDeleted)
	skipSamplesOutsideRetention(ib2, retentionDeadline, rowsDeleted)

	if ib1.bh.MaxTimestamp < ib2.bh.MinTimestamp {
		// Fast path - ib1 values have smaller timestamps than ib2 values.
		appendRows(ob, ib1)
		appendRows(ob, ib2)
		return
	}
	if ib2.bh.MaxTimestamp < ib1.bh.MinTimestamp {
		// Fast path - ib2 values have smaller timestamps than ib1 values.
		appendRows(ob, ib2)
		appendRows(ob, ib1)
		return
	}
	if ib1.nextIdx >= len(ib1.timestamps) {
		appendRows(ob, ib2)
		return
	}
	if ib2.nextIdx >= len(ib2.timestamps) {
		appendRows(ob, ib1)
		return
	}
	for {
		i := ib1.nextIdx
		ts2 := ib2.timestamps[ib2.nextIdx]
		for i < len(ib1.timestamps) && ib1.timestamps[i] <= ts2 {
			i++
		}
		ob.timestamps = append(ob.timestamps, ib1.timestamps[ib1.nextIdx:i]...)
		ob.values = append(ob.values, ib1.values[ib1.nextIdx:i]...)
		ib1.nextIdx = i
		if ib1.nextIdx >= len(ib1.timestamps) {
			appendRows(ob, ib2)
			return
		}
		ib1, ib2 = ib2, ib1
	}
}

func skipSamplesOutsideRetention(b *Block, retentionDeadline int64, rowsDeleted *uint64) {
	if b.bh.MinTimestamp >= retentionDeadline {
		// Fast path - the block contains only samples with timestamps bigger than retentionDeadline.
		return
	}
	timestamps := b.timestamps
	nextIdx := b.nextIdx
	nextIdxOrig := nextIdx
	for nextIdx < len(timestamps) && timestamps[nextIdx] < retentionDeadline {
		nextIdx++
	}
	if n := nextIdx - nextIdxOrig; n > 0 {
		*rowsDeleted += uint64(n)
		b.nextIdx = nextIdx
	}
}

func appendRows(ob, ib *Block) {
	ob.timestamps = append(ob.timestamps, ib.timestamps[ib.nextIdx:]...)
	ob.values = append(ob.values, ib.values[ib.nextIdx:]...)
}

func unmarshalAndCalibrateScale(b1, b2 *Block) error {
	if err := b1.UnmarshalData(); err != nil {
		return err
	}
	if err := b2.UnmarshalData(); err != nil {
		return err
	}

	scale := decimal.CalibrateScale(b1.values[b1.nextIdx:], b1.bh.Scale, b2.values[b2.nextIdx:], b2.bh.Scale)
	b1.bh.Scale = scale
	b2.bh.Scale = scale
	return nil
}
