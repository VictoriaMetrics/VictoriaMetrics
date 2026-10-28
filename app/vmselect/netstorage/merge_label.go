package netstorage

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// mergeLabelContext holds context values for timeseries patches
// with given mergeLabel.
//
// T generic type is needed to reduce code diff between single and cluster branches.
type mergeLabelContext[T blockRef] struct {
	mergeLabel string

	mn            storage.MetricName
	patchNamePrev []byte
	patchKey      []byte
	patchRev      int64

	// metricNamesBuf holds strings from patches map.
	metricNamesBuf []byte
	patches        map[string]map[int64][]T

	// patchRevsCount is the number of unique (metricName, revision) pairs at patches.
	patchRevsCount int
	patchesPool    [][]T
	revsBuf        []int64
}

func newMergeLabelContext[T blockRef](mergeLabel string, maxSeriesCount int) *mergeLabelContext[T] {
	metricNamesBufCap := min(maxSeriesCount*100, maxFastAllocBlockSize)
	return &mergeLabelContext[T]{
		mergeLabel:     mergeLabel,
		metricNamesBuf: make([]byte, 0, metricNamesBufCap),
		patches:        make(map[string]map[int64][]T, maxSeriesCount),
	}
}

func (mlc *mergeLabelContext[T]) addPatch(metricName []byte, patch T) (bool, error) {
	if mlc == nil {
		return false, nil
	}
	if string(metricName) != string(mlc.patchNamePrev) {
		mlc.patchNamePrev = append(mlc.patchNamePrev[:0], metricName...)
		var err error
		mlc.patchKey, mlc.patchRev, err = getMetricNameRevisionValue(mlc.patchKey[:0], &mlc.mn, metricName, mlc.mergeLabel)
		if err != nil {
			return false, err
		}
	}
	if len(mlc.patchKey) == 0 {
		return false, nil
	}
	revs := mlc.patches[string(mlc.patchKey)]
	if revs == nil {
		var metricNameStr string
		mlc.metricNamesBuf, metricNameStr = cloneMetricNameWithBuf(mlc.metricNamesBuf, mlc.patchKey)
		revs = make(map[int64][]T)
		mlc.patches[metricNameStr] = revs
	}
	if len(revs[mlc.patchRev]) == 0 {
		mlc.patchRevsCount++
	}
	// TODO: reduce allocations if needed
	revs[mlc.patchRev] = append(revs[mlc.patchRev], patch)
	return true, nil
}

func (mlc *mergeLabelContext[T]) getPatchesByMetricName(metricName string) [][]T {
	if mlc == nil || mlc.patchRevsCount == 0 {
		return nil
	}
	revs := mlc.patches[metricName]
	if len(revs) == 0 {
		return nil
	}
	if mlc.patchesPool == nil {
		mlc.patchesPool = make([][]T, 0, mlc.patchRevsCount)
	}
	return mlc.getSortedPatches(revs)
}

func (mlc *mergeLabelContext[T]) getSortedPatches(revs map[int64][]T) [][]T {
	revsBuf := mlc.revsBuf[:0]
	for rev := range revs {
		revsBuf = append(revsBuf, rev)
	}
	slices.Sort(revsBuf)
	n := len(mlc.patchesPool)
	for _, rev := range revsBuf {
		mlc.patchesPool = append(mlc.patchesPool, revs[rev])
	}
	mlc.revsBuf = revsBuf
	return mlc.patchesPool[n:len(mlc.patchesPool):len(mlc.patchesPool)]
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

// mergeResults expects that timestamps are sorted.
func mergeResult(dst, patch *Result) {
	if len(patch.Timestamps) == 0 {
		return
	}
	// search for patch.Timestamps[0]-1 in order to get first timestamp greater or equal first timestamp at patch.
	start := binarySearchTimestamps(dst.Timestamps, patch.Timestamps[0]-1)
	end := start + binarySearchTimestamps(dst.Timestamps[start:], patch.Timestamps[len(patch.Timestamps)-1])
	dst.Timestamps = slices.Replace(dst.Timestamps, start, end, patch.Timestamps...)
	dst.Values = slices.Replace(dst.Values, start, end, patch.Values...)
}
