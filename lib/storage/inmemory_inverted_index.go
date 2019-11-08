package storage

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

type inmemoryInvertedIndex struct {
	mu               sync.RWMutex
	m                map[string]*uint64set.Set
	pendingMetricIDs []uint64
}

func (iidx *inmemoryInvertedIndex) GetUniqueTagPairsLen() int {
	if iidx == nil {
		return 0
	}
	iidx.mu.RLock()
	n := len(iidx.m)
	iidx.mu.RUnlock()
	return n
}

func (iidx *inmemoryInvertedIndex) GetEntriesCount() int {
	if iidx == nil {
		return 0
	}
	n := 0
	iidx.mu.RLock()
	for _, v := range iidx.m {
		n += v.Len()
	}
	iidx.mu.RUnlock()
	return n
}

func (iidx *inmemoryInvertedIndex) GetPendingMetricIDsLen() int {
	if iidx == nil {
		return 0
	}
	iidx.mu.RLock()
	n := len(iidx.pendingMetricIDs)
	iidx.mu.RUnlock()
	return n
}

func newInmemoryInvertedIndex() *inmemoryInvertedIndex {
	return &inmemoryInvertedIndex{
		m: make(map[string]*uint64set.Set),
	}
}

func (iidx *inmemoryInvertedIndex) Clone() *inmemoryInvertedIndex {
	if iidx == nil {
		return newInmemoryInvertedIndex()
	}
	iidx.mu.RLock()
	mCopy := make(map[string]*uint64set.Set, len(iidx.m))
	for k, v := range iidx.m {
		mCopy[k] = v.Clone()
	}
	pendingMetricIDs := append([]uint64{}, iidx.pendingMetricIDs...)
	iidx.mu.RUnlock()
	return &inmemoryInvertedIndex{
		m:                mCopy,
		pendingMetricIDs: pendingMetricIDs,
	}
}

func (iidx *inmemoryInvertedIndex) MustUpdate(idb *indexDB, src *uint64set.Set) {
	metricIDs := src.AppendTo(nil)
	iidx.mu.Lock()
	iidx.pendingMetricIDs = append(iidx.pendingMetricIDs, metricIDs...)
	if err := iidx.updateLocked(idb); err != nil {
		logger.Panicf("FATAL: cannot update inmemoryInvertedIndex with pendingMetricIDs: %s", err)
	}
	iidx.mu.Unlock()
}

func (iidx *inmemoryInvertedIndex) AddMetricID(idb *indexDB, metricID uint64) {
	iidx.mu.Lock()
	iidx.pendingMetricIDs = append(iidx.pendingMetricIDs, metricID)
	if err := iidx.updateLocked(idb); err != nil {
		logger.Panicf("FATAL: cannot update inmemoryInvertedIndex with pendingMetricIDs: %s", err)
	}
	iidx.mu.Unlock()
}

func (iidx *inmemoryInvertedIndex) UpdateMetricIDsForTagFilters(metricIDs, allMetricIDs *uint64set.Set, tfs *TagFilters) {
	if iidx == nil {
		return
	}
	var result *uint64set.Set
	var tfFirst *tagFilter
	for i := range tfs.tfs {
		if tfs.tfs[i].isNegative {
			continue
		}
		tfFirst = &tfs.tfs[i]
	}

	iidx.mu.RLock()
	defer iidx.mu.RUnlock()

	if tfFirst == nil {
		result = allMetricIDs.Clone()
	} else {
		result = iidx.getMetricIDsForTagFilterLocked(tfFirst, tfs.commonPrefix)
	}
	for i := range tfs.tfs {
		tf := &tfs.tfs[i]
		if tf == tfFirst {
			continue
		}
		m := iidx.getMetricIDsForTagFilterLocked(tf, tfs.commonPrefix)
		if tf.isNegative {
			result.Subtract(m)
		} else {
			result.Intersect(m)
		}
		if result.Len() == 0 {
			return
		}
	}
	metricIDs.Union(result)
}

func (iidx *inmemoryInvertedIndex) getMetricIDsForTagFilterLocked(tf *tagFilter, commonPrefix []byte) *uint64set.Set {
	if !bytes.HasPrefix(tf.prefix, commonPrefix) {
		logger.Panicf("BUG: tf.prefix must start with commonPrefix=%q; got %q", commonPrefix, tf.prefix)
	}
	prefix := tf.prefix[len(commonPrefix):]
	var m uint64set.Set
	kb := kbPool.Get()
	defer kbPool.Put(kb)
	for k, v := range iidx.m {
		if len(k) < len(prefix) || k[:len(prefix)] != string(prefix) {
			continue
		}
		kb.B = append(kb.B[:0], k[len(prefix):]...)
		ok, err := tf.matchSuffix(kb.B)
		if err != nil {
			logger.Panicf("BUG: unexpected error from matchSuffix(%q): %s", kb.B, err)
		}
		if !ok {
			continue
		}
		m.Union(v)
	}
	return &m
}

func (iidx *inmemoryInvertedIndex) updateLocked(idb *indexDB) error {
	metricIDs := iidx.pendingMetricIDs
	iidx.pendingMetricIDs = iidx.pendingMetricIDs[:0]

	kb := kbPool.Get()
	defer kbPool.Put(kb)

	var mn MetricName
	for _, metricID := range metricIDs {
		var err error
		kb.B, err = idb.searchMetricName(kb.B[:0], metricID)
		if err != nil {
			if err == io.EOF {
				iidx.pendingMetricIDs = append(iidx.pendingMetricIDs, metricID)
				continue
			}
			return fmt.Errorf("cannot find metricName by metricID %d: %s", metricID, err)
		}
		if err = mn.Unmarshal(kb.B); err != nil {
			return fmt.Errorf("cannot unmarshal metricName %q: %s", kb.B, err)
		}
		kb.B = marshalTagValue(kb.B[:0], nil)
		kb.B = marshalTagValue(kb.B, mn.MetricGroup)
		iidx.addMetricIDLocked(kb.B, metricID)
		for i := range mn.Tags {
			kb.B = mn.Tags[i].Marshal(kb.B[:0])
			iidx.addMetricIDLocked(kb.B, metricID)
		}
	}
	return nil
}

func (iidx *inmemoryInvertedIndex) addMetricIDLocked(key []byte, metricID uint64) {
	v := iidx.m[string(key)]
	if v == nil {
		v = &uint64set.Set{}
		iidx.m[string(key)] = v
	}
	v.Add(metricID)
}
