package netstorage

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// seriesUpdateContext holds context values for timeseries updates
// grouped by given revisionLabel.
//
// T generic type is needed to reduce code diff between single and cluster branches.
type seriesUpdateContext[T blockRef] struct {
	revisionLabel string

	// temporary values below allows to reduce memory allocations
	// for subsequent tryAddUpdate calls.
	mn             storage.MetricName
	metricNamePrev []byte
	metricNameCurr []byte
	updateRevision int64

	// metricNamesBuf holds strings from updatesByMetricName map.
	metricNamesBuf      []byte
	updatesByMetricName map[string]map[int64][]T

	revsBuf []int64
}

func newSeriesUpdateContext[T blockRef](mergeLabel string, maxSeriesCount int) *seriesUpdateContext[T] {
	metricNamesBufCap := min(maxSeriesCount*100, maxFastAllocBlockSize)
	return &seriesUpdateContext[T]{
		revisionLabel:       mergeLabel,
		metricNamesBuf:      make([]byte, 0, metricNamesBufCap),
		updatesByMetricName: make(map[string]map[int64][]T, maxSeriesCount),
	}
}

func (suc *seriesUpdateContext[T]) tryAddUpdate(metricName []byte, patch T) (bool, error) {
	if suc == nil {
		return false, nil
	}
	if string(metricName) != string(suc.metricNamePrev) {
		suc.metricNamePrev = append(suc.metricNamePrev[:0], metricName...)
		var err error
		suc.metricNameCurr, suc.updateRevision, err = getMetricNameRevisionValue(suc.metricNameCurr[:0], &suc.mn, metricName, suc.revisionLabel)
		if err != nil {
			return false, err
		}
	}
	if len(suc.metricNameCurr) == 0 {
		return false, nil
	}
	revs := suc.updatesByMetricName[string(suc.metricNameCurr)]
	if revs == nil {
		var metricNameStr string
		suc.metricNamesBuf, metricNameStr = cloneMetricNameWithBuf(suc.metricNamesBuf, suc.metricNameCurr)
		revs = make(map[int64][]T)
		suc.updatesByMetricName[metricNameStr] = revs
	}
	// TODO: reduce allocations if needed.
	revs[suc.updateRevision] = append(revs[suc.updateRevision], patch)
	return true, nil
}

func (suc *seriesUpdateContext[T]) getUpdatesByMetricName(metricName string) [][]T {
	if suc == nil || len(suc.updatesByMetricName) == 0 {
		return nil
	}
	revs := suc.updatesByMetricName[metricName]
	if len(revs) == 0 {
		return nil
	}
	return suc.getUpdatesByRevision(revs)
}

func (suc *seriesUpdateContext[T]) getUpdatesByRevision(revs map[int64][]T) [][]T {
	revsBuf := suc.revsBuf[:0]
	for rev := range revs {
		revsBuf = append(revsBuf, rev)
	}
	// TODO: reduce allocations if needed.
	results := make([][]T, 0, len(revs))
	slices.Sort(revsBuf)
	for _, rev := range revsBuf {
		// TODO: make a shallow copy of revs[rev] to remove reference for suc.
		results = append(results, revs[rev])
	}
	suc.revsBuf = revsBuf
	return results
}

func cloneMetricNameWithBuf(metricNamesBuf []byte, metricName []byte) ([]byte, string) {
	if cap(metricNamesBuf) >= maxFastAllocBlockSize && len(metricNamesBuf)+len(metricName) > cap(metricNamesBuf) {
		// Allocate a new metricNamesBuf in order to avoid slow allocation of byte slice
		// bigger than maxFastAllocBlockSize bytes at append() below.
		metricNamesBuf = make([]byte, 0, maxFastAllocBlockSize)
	}
	metricNamesBufLen := len(metricNamesBuf)
	metricNamesBuf = append(metricNamesBuf, metricName...)
	metricNameStr := bytesutil.ToUnsafeString(metricNamesBuf[metricNamesBufLen:])
	return metricNamesBuf, metricNameStr
}

func getMetricNameRevisionValue(dst []byte, mn *storage.MetricName, metricName []byte, mergeLabel string) ([]byte, int64, error) {
	if err := mn.Unmarshal(metricName); err != nil {
		return dst, 0, fmt.Errorf("cannot unmarshal metricName %q: %w", metricName, err)
	}
	v := mn.GetTagValue(mergeLabel)
	if len(v) == 0 {
		return dst, 0, nil
	}
	rev, err := strconv.ParseInt(string(v), 10, 64)
	if err != nil {
		return dst, 0, fmt.Errorf("cannot parse revision from %q label value of series %s: %w", mergeLabel, mn.String(), err)
	}
	mn.RemoveTag(mergeLabel)
	return mn.Marshal(dst), rev, nil
}
