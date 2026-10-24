package netstorage

import (
	"container/heap"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// mergeSortBlocksRef is a verbatim copy of mergeSortBlocks from v1.154.1-cluster.
//
// It is the reference for differential tests and the baseline for benchmarks.
func mergeSortBlocksRef(dst *Result, sbh *sortBlocksHeap, dedupInterval int64) {
	rowsSkipped := 0
	// Skip empty sort blocks, since they cannot be passed to heap.Init.
	sbs := sbh.sbs[:0]
	for _, sb := range sbh.sbs {
		rowsSkipped += sb.rowsSkipped
		if len(sb.Timestamps) == 0 {
			putSortBlock(sb)
			continue
		}
		sbs = append(sbs, sb)
	}
	sbh.sbs = sbs
	dst.rowsSkipped = rowsSkipped
	if sbh.Len() == 0 {
		return
	}
	heap.Init(sbh)
	var dedupSamples int
	for {
		sbs := sbh.sbs
		top := sbs[0]
		if len(sbs) == 1 {
			dst.Timestamps = append(dst.Timestamps, top.Timestamps[top.NextIdx:]...)
			dst.Values = append(dst.Values, top.Values[top.NextIdx:]...)
			putSortBlock(top)
			break
		}
		sbNext := sbh.getNextBlock()
		tsNext := sbNext.Timestamps[sbNext.NextIdx]
		topNextIdx := top.NextIdx
		if n := equalSamplesPrefix(top, sbNext); n > 0 && dedupInterval > 0 {
			// Skip n replicated samples at top if deduplication is enabled.
			top.NextIdx = topNextIdx + n
			dedupSamples += n
		} else {
			// Copy samples from top to dst with timestamps not exceeding tsNext.
			top.NextIdx = topNextIdx + binarySearchTimestamps(top.Timestamps[topNextIdx:], tsNext)
			dst.Timestamps = append(dst.Timestamps, top.Timestamps[topNextIdx:top.NextIdx]...)
			dst.Values = append(dst.Values, top.Values[topNextIdx:top.NextIdx]...)
		}
		if top.NextIdx < len(top.Timestamps) {
			heap.Fix(sbh, 0)
		} else {
			heap.Pop(sbh)
			putSortBlock(top)
		}
	}
	timestamps, values := storage.DeduplicateSamples(dst.Timestamps, dst.Values, dedupInterval)
	dedupSamples += len(dst.Timestamps) - len(timestamps)
	dedupsDuringSelect.Add(dedupSamples)
	dst.Timestamps = timestamps
	dst.Values = values
}
