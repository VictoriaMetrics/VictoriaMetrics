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
	mu             sync.RWMutex
	m              map[string]*uint64set.Set
	pendingEntries []pendingHourMetricIDEntry
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
	n := len(iidx.pendingEntries)
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
	pendingEntries := append([]pendingHourMetricIDEntry{}, iidx.pendingEntries...)
	iidx.mu.RUnlock()
	return &inmemoryInvertedIndex{
		m:              mCopy,
		pendingEntries: pendingEntries,
	}
}

func (iidx *inmemoryInvertedIndex) MustUpdate(idb *indexDB, byTenant map[accountProjectKey]*uint64set.Set) {
	var entries []pendingHourMetricIDEntry
	var metricIDs []uint64
	for k, v := range byTenant {
		var e pendingHourMetricIDEntry
		e.AccountID = k.AccountID
		e.ProjectID = k.ProjectID
		metricIDs = v.AppendTo(metricIDs[:0])
		for _, metricID := range metricIDs {
			e.MetricID = metricID
			entries = append(entries, e)
		}
	}

	iidx.mu.Lock()
	iidx.pendingEntries = append(iidx.pendingEntries, entries...)
	if err := iidx.addPendingEntriesLocked(idb); err != nil {
		logger.Panicf("FATAL: cannot update inmemoryInvertedIndex with pendingEntries: %s", err)
	}
	iidx.mu.Unlock()
}

func (iidx *inmemoryInvertedIndex) AddMetricID(idb *indexDB, e pendingHourMetricIDEntry) {
	iidx.mu.Lock()
	iidx.pendingEntries = append(iidx.pendingEntries, e)
	if err := iidx.addPendingEntriesLocked(idb); err != nil {
		logger.Panicf("FATAL: cannot update inmemoryInvertedIndex with pendingEntries: %s", err)
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
		break
	}

	iidx.mu.RLock()
	defer iidx.mu.RUnlock()

	if tfFirst == nil {
		result = allMetricIDs.Clone()
	} else {
		result = iidx.getMetricIDsForTagFilterLocked(tfFirst, tfs.commonPrefix)
		result.Intersect(allMetricIDs) // This line is required for filtering metrics by (accountID, projectID)
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

func (iidx *inmemoryInvertedIndex) addPendingEntriesLocked(idb *indexDB) error {
	entries := iidx.pendingEntries
	iidx.pendingEntries = iidx.pendingEntries[:0]

	kb := kbPool.Get()
	defer kbPool.Put(kb)

	mn := GetMetricName()
	defer PutMetricName(mn)
	for _, e := range entries {
		var err error
		metricID := e.MetricID
		kb.B, err = idb.searchMetricName(kb.B[:0], metricID, e.AccountID, e.ProjectID)
		if err != nil {
			if err == io.EOF {
				iidx.pendingEntries = append(iidx.pendingEntries, e)
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
