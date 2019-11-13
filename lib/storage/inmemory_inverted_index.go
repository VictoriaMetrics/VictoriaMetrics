package storage

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

type inmemoryInvertedIndex struct {
	mu               sync.RWMutex
	m                map[string]*uint64set.Set
	pendingMetricIDs []uint64
}

func (iidx *inmemoryInvertedIndex) Marshal(dst []byte) []byte {
	iidx.mu.RLock()
	defer iidx.mu.RUnlock()

	// Marshal iidx.m
	var metricIDs []uint64
	dst = encoding.MarshalUint64(dst, uint64(len(iidx.m)))
	for k, v := range iidx.m {
		dst = encoding.MarshalBytes(dst, []byte(k))
		metricIDs = v.AppendTo(metricIDs[:0])
		dst = marshalMetricIDs(dst, metricIDs)
	}

	// Marshal iidx.pendingMetricIDs
	dst = marshalMetricIDs(dst, iidx.pendingMetricIDs)

	return dst
}

func (iidx *inmemoryInvertedIndex) Unmarshal(src []byte) ([]byte, error) {
	iidx.mu.Lock()
	defer iidx.mu.Unlock()

	// Unmarshal iidx.m
	if len(src) < 8 {
		return src, fmt.Errorf("cannot read len(iidx.m) from %d bytes; want at least 8 bytes", len(src))
	}
	mLen := int(encoding.UnmarshalUint64(src))
	src = src[8:]
	m := make(map[string]*uint64set.Set, mLen)
	var metricIDs []uint64
	for i := 0; i < mLen; i++ {
		tail, k, err := encoding.UnmarshalBytes(src)
		if err != nil {
			return tail, fmt.Errorf("cannot unmarshal key #%d for iidx.m: %s", i, err)
		}
		src = tail
		tail, metricIDs, err = unmarshalMetricIDs(metricIDs[:0], src)
		if err != nil {
			return tail, fmt.Errorf("cannot unmarshal value #%d for iidx.m: %s", i, err)
		}
		src = tail
		var v uint64set.Set
		for _, metricID := range metricIDs {
			v.Add(metricID)
		}
		m[string(k)] = &v
	}
	iidx.m = m

	// Unmarshal iidx.pendingMetricIDs
	var err error
	var tail []byte
	tail, metricIDs, err = unmarshalMetricIDs(metricIDs[:0], src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal iidx.pendingMetricIDs: %s", err)
	}
	src = tail
	iidx.pendingMetricIDs = append(iidx.pendingMetricIDs[:0], metricIDs...)

	return src, nil
}

func marshalMetricIDs(dst []byte, metricIDs []uint64) []byte {
	dst = encoding.MarshalUint64(dst, uint64(len(metricIDs)))
	for _, metricID := range metricIDs {
		dst = encoding.MarshalUint64(dst, metricID)
	}
	return dst
}

func unmarshalMetricIDs(dst []uint64, src []byte) ([]byte, []uint64, error) {
	if len(src) < 8 {
		return src, dst, fmt.Errorf("cannot unmarshal metricIDs len from %d bytes; want at least 8 bytes", len(src))
	}
	metricIDsLen := int(encoding.UnmarshalUint64(src))
	src = src[8:]
	if len(src) < 8*metricIDsLen {
		return src, dst, fmt.Errorf("not enough bytes for unmarshaling %d metricIDs; want %d bytes; got %d bytes", metricIDsLen, 8*metricIDsLen, len(src))
	}
	for i := 0; i < metricIDsLen; i++ {
		metricID := encoding.UnmarshalUint64(src)
		src = src[8:]
		dst = append(dst, metricID)
	}
	return src, dst, nil
}

func (iidx *inmemoryInvertedIndex) SizeBytes() uint64 {
	n := uint64(0)
	iidx.mu.RLock()
	for k, v := range iidx.m {
		n += uint64(len(k))
		n += v.SizeBytes()
	}
	n += uint64(len(iidx.pendingMetricIDs)) * 8
	iidx.mu.RUnlock()
	return n
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

func (iidx *inmemoryInvertedIndex) MustUpdate(idb *indexDB, src *uint64set.Set) {
	metricIDs := src.AppendTo(nil)
	iidx.mu.Lock()
	iidx.pendingMetricIDs = append(iidx.pendingMetricIDs, metricIDs...)
	if err := iidx.addPendingEntriesLocked(idb); err != nil {
		logger.Panicf("FATAL: cannot update inmemoryInvertedIndex with pendingMetricIDs: %s", err)
	}
	iidx.mu.Unlock()
}

func (iidx *inmemoryInvertedIndex) AddMetricID(idb *indexDB, metricID uint64) {
	iidx.mu.Lock()
	iidx.pendingMetricIDs = append(iidx.pendingMetricIDs, metricID)
	if err := iidx.addPendingEntriesLocked(idb); err != nil {
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
		break
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

func (iidx *inmemoryInvertedIndex) addPendingEntriesLocked(idb *indexDB) error {
	metricIDs := iidx.pendingMetricIDs
	iidx.pendingMetricIDs = iidx.pendingMetricIDs[:0]

	kb := kbPool.Get()
	defer kbPool.Put(kb)

	mn := GetMetricName()
	defer PutMetricName(mn)
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
