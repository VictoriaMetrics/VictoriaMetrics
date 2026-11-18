package netstorage

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// seriesUpdateContext holds context values for timeseries updates
// grouped by given updateIDLabel.
//
// T generic update type is needed to reduce code diff between single and cluster branches.
type seriesUpdateContext[T any] struct {
	updateRevisionLabel string

	// Temporary values below allow to reduce memory allocations
	// for subsequent tryAddUpdate calls.
	mn             storage.MetricName
	metricNamePrev []byte
	metricNameCurr []byte
	updateRevision int64

	// metricNamesBuf holds strings from updatesByMetricName map.
	metricNamesBuf      []byte
	updatesByMetricName map[string]map[int64][]T

	updateRevisionsBuf []int64
}

func newSeriesUpdateContext[T any](updateRevisionLabel string, maxSeriesCount int) *seriesUpdateContext[T] {
	metricNamesBufCap := min(maxSeriesCount*100, maxFastAllocBlockSize)
	return &seriesUpdateContext[T]{
		updateRevisionLabel: updateRevisionLabel,
		metricNamesBuf:      make([]byte, 0, metricNamesBufCap),
		updatesByMetricName: make(map[string]map[int64][]T, maxSeriesCount),
	}
}

func (suc *seriesUpdateContext[T]) tryAddUpdate(metricName []byte, update T) (bool, error) {
	if suc == nil {
		return false, nil
	}
	if string(metricName) != string(suc.metricNamePrev) {
		suc.metricNamePrev = append(suc.metricNamePrev[:0], metricName...)
		var err error
		suc.metricNameCurr, suc.updateRevision, err = getSeriesUpdateRevision(suc.metricNameCurr[:0], &suc.mn, metricName, suc.updateRevisionLabel)
		if err != nil {
			return false, err
		}
	}
	if len(suc.metricNameCurr) == 0 {
		return false, nil
	}
	updatesByRevision := suc.updatesByMetricName[string(suc.metricNameCurr)]
	if updatesByRevision == nil {
		var metricNameStr string
		suc.metricNamesBuf, metricNameStr = cloneMetricNameWithBuf(suc.metricNamesBuf, suc.metricNameCurr)
		updatesByRevision = make(map[int64][]T)
		suc.updatesByMetricName[metricNameStr] = updatesByRevision
	}
	// TODO: reduce allocations if needed.
	updatesByRevision[suc.updateRevision] = append(updatesByRevision[suc.updateRevision], update)
	return true, nil
}

func (suc *seriesUpdateContext[T]) getUpdatesByMetricName(metricName string) [][]T {
	if suc == nil || len(suc.updatesByMetricName) == 0 {
		return nil
	}
	updates := suc.updatesByMetricName[metricName]
	if len(updates) == 0 {
		return nil
	}
	return suc.sortUpdates(updates)
}

func (suc *seriesUpdateContext[T]) sortUpdates(updateRevisions map[int64][]T) [][]T {
	updateIDsBuf := suc.updateRevisionsBuf[:0]
	for rev := range updateRevisions {
		updateIDsBuf = append(updateIDsBuf, rev)
	}
	slices.Sort(updateIDsBuf)
	// TODO: reduce allocations if needed.
	results := make([][]T, 0, len(updateRevisions))
	for _, rev := range updateIDsBuf {
		// TODO: make a shallow copy of updateRevisions[rev] in order to remove reference for suc.
		results = append(results, updateRevisions[rev])
	}
	suc.updateRevisionsBuf = updateIDsBuf
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

func getSeriesUpdateRevision(dst []byte, mn *storage.MetricName, metricName []byte, mergeLabel string) ([]byte, int64, error) {
	if err := mn.Unmarshal(metricName); err != nil {
		return dst, 0, fmt.Errorf("cannot unmarshal metricName %q: %w", metricName, err)
	}
	v := mn.GetTagValue(mergeLabel)
	if len(v) == 0 {
		return dst, 0, nil
	}
	revision, err := strconv.ParseInt(string(v), 10, 64)
	if err != nil {
		return dst, 0, fmt.Errorf("cannot parse update revision from %q label value of series %s: %w", mergeLabel, mn.String(), err)
	}
	mn.RemoveTag(mergeLabel)
	return mn.Marshal(dst), revision, nil
}
