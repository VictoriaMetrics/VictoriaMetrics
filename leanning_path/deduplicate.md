# deduplicate

vmstore的去重逻辑

> VictoriaMetrics stores timestamps with millisecond precision, so -dedup.minScrapeInterval=1ms command-line flag must be passed to vmselect nodes when the replication is enabled, so they could de-duplicate replicated samples obtained from distinct vmstorage nodes during querying.
> 
> If duplicate data is pushed to VictoriaMetrics from identically configured vmagent instances or Prometheus instances, then the -dedup.minScrapeInterval must be set to scrape_interval from scrape configs according to deduplication docs.

相关源码：

```shell
func deduplicateSamplesDuringMerge(srcTimestamps, srcValues []int64, dedupInterval int64) ([]int64, []int64) {
	if !needsDedup(srcTimestamps, dedupInterval) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}
	tsNext := srcTimestamps[0] + dedupInterval - 1
	tsNext -= tsNext % dedupInterval
	dstTimestamps := srcTimestamps[:0]
	dstValues := srcValues[:0]
	for i, ts := range srcTimestamps[1:] {
		if ts <= tsNext {
			continue
		}
		// Choose the maximum value with the timestamp equal to tsPrev.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3333
		j := i
		tsPrev := srcTimestamps[j]
		vPrev := srcValues[j]
		for j > 0 && srcTimestamps[j-1] == tsPrev {
			j--
			if srcValues[j] > vPrev {
				vPrev = srcValues[j]
			}
		}
		dstTimestamps = append(dstTimestamps, tsPrev)
		dstValues = append(dstValues, vPrev)
		tsNext += dedupInterval
		if tsNext < ts {
			tsNext = ts + dedupInterval - 1
			tsNext -= tsNext % dedupInterval
		}
	}
	j := len(srcTimestamps) - 1
	tsPrev := srcTimestamps[j]
	vPrev := srcValues[j]
	for j > 0 && srcTimestamps[j-1] == tsPrev {
		j--
		if srcValues[j] > vPrev {
			vPrev = srcValues[j]
		}
	}
	dstTimestamps = append(dstTimestamps, tsPrev)
	dstValues = append(dstValues, vPrev)
	return dstTimestamps, dstValues
}
```