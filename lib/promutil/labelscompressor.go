package promutil

import (
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// LabelsCompressor compresses []prompb.Label into short binary strings
type LabelsCompressor struct {
	labelToIdx sync.Map
	idxToLabel labelsMap

	totalSizeBytes atomic.Uint64
	itemsCount     atomic.Uint64

	cleanupLock sync.Mutex

	mu           sync.Mutex
	nextIdx      uint64
	freeIdxs     []uint64
	standby      map[prompb.Label]*labelEntry
	standbySince time.Time
}

// labelEntry is the labelToIdx value of a label.
type labelEntry struct {
	idx uint64

	// referenced is set by Compress and cleared by Cleanup.
	referenced atomic.Bool
}

// SizeBytes returns the size of lc data in bytes
func (lc *LabelsCompressor) SizeBytes() uint64 {
	return uint64(unsafe.Sizeof(*lc)) + lc.totalSizeBytes.Load()
}

// ItemsCount returns the number of items in lc
func (lc *LabelsCompressor) ItemsCount() uint64 {
	return lc.itemsCount.Load()
}

// Compress compresses labels, appends the compressed labels to dst and returns the result.
//
// It is safe calling Compress from concurrent goroutines.
func (lc *LabelsCompressor) Compress(dst []byte, labels []prompb.Label) []byte {
	if len(labels) == 0 {
		// Fast path
		return append(dst, 0)
	}

	a := encoding.GetUint64s(len(labels) + 1)
	a.A[0] = uint64(len(labels))
	lc.compress(a.A[1:], labels)
	dst = encoding.MarshalVarUint64s(dst, a.A)
	encoding.PutUint64s(a)
	return dst
}

func (lc *LabelsCompressor) compress(dst []uint64, labels []prompb.Label) {
	if len(labels) == 0 {
		return
	}
	_ = dst[len(labels)-1]
	for i, label := range labels {
		var e *labelEntry
		if v, ok := lc.labelToIdx.Load(label); ok {
			e = v.(*labelEntry)
		} else {
			e = lc.insert(label)
		}
		if !e.referenced.Load() {
			e.referenced.Store(true)
		}
		dst[i] = e.idx
	}
}

func (lc *LabelsCompressor) insert(label prompb.Label) *labelEntry {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if v, ok := lc.labelToIdx.Load(label); ok {
		return v.(*labelEntry)
	}
	if e, ok := lc.standby[label]; ok {
		// The label is on standby. Restore it with the same idx, since keys may still use it.
		delete(lc.standby, label)
		labelCopy, _ := lc.idxToLabel.Load(e.idx)
		lc.labelToIdx.Store(labelCopy, e)
		return e
	}

	var idx uint64
	if n := len(lc.freeIdxs); n > 0 {
		idx = lc.freeIdxs[n-1]
		lc.freeIdxs = lc.freeIdxs[:n-1]
	} else {
		lc.nextIdx++
		idx = lc.nextIdx
	}
	e := &labelEntry{
		idx: idx,
	}
	labelCopy := cloneLabel(label)
	// Must store idxToLabel entry before labelToIdx, so it can be found by concurrent goroutines.
	lc.idxToLabel.Store(idx, labelCopy)
	lc.labelToIdx.Store(labelCopy, e)

	lc.totalSizeBytes.Add(labelEntrySizeBytes(label))
	lc.itemsCount.Add(1)
	return e
}

func labelEntrySizeBytes(label prompb.Label) uint64 {
	labelSizeBytes := uint64(len(label.Name) + len(label.Value))
	return labelSizeBytes + uint64(2*(unsafe.Sizeof(label)+unsafe.Sizeof(&label))+unsafe.Sizeof(labelEntry{}))
}

// Cleanup frees labels that no key uses and returns the number of freed labels.
//
// The caller must make sure that every key in use was compressed at or after keysGoneBefore.
// now returns the time for the labels moved to standby. It must not be earlier than any key
// compressed before now is called.
//
// Compress marks every label it uses, and Cleanup clears the marks. Labels that were not used
// since the last Cleanup go to standby. A standby label can still be decoded, and Compress
// restores it with the same idx. A later Cleanup frees standby labels moved before keysGoneBefore.
//
// It is safe calling Cleanup concurrently with Compress and Decompress.
func (lc *LabelsCompressor) Cleanup(keysGoneBefore time.Time, now func() time.Time) int {
	lc.cleanupLock.Lock()
	defer lc.cleanupLock.Unlock()

	lc.mu.Lock()
	if len(lc.standby) > 0 && !lc.standbySince.Before(keysGoneBefore) {
		lc.mu.Unlock()
		return 0
	}

	// No key uses the standby labels anymore. A key that used one was compressed before standbySince,
	// and a later Compress would have restored the label.
	standby := lc.standby
	lc.standby = make(map[prompb.Label]*labelEntry)
	lc.mu.Unlock()
	freed := lc.free(standby)

	// Move labels that were not used since the last Cleanup to standby.
	// lc.mu is held only while a batch is moved, so inserts are blocked only for a short time.
	labels := make([]prompb.Label, 0, cleanupBatchSize)
	entries := make([]*labelEntry, 0, cleanupBatchSize)
	moveToStandby := func() {
		lc.mu.Lock()
		for i, label := range labels {
			if lc.labelToIdx.CompareAndDelete(label, entries[i]) {
				lc.standby[label] = entries[i]
			}
		}
		lc.mu.Unlock()
		labels, entries = labels[:0], entries[:0]
	}
	lc.labelToIdx.Range(func(k, v any) bool {
		if e := v.(*labelEntry); !e.referenced.Swap(false) {
			labels = append(labels, k.(prompb.Label))
			entries = append(entries, e)
			if len(labels) == cleanupBatchSize {
				moveToStandby()
			}
		}
		return true
	})
	moveToStandby()

	// A Compress that runs during the scan may still use a label that was moved to standby.
	// So set the time after the scan.
	lc.mu.Lock()
	lc.standbySince = now()
	lc.mu.Unlock()
	return freed
}

// cleanupBatchSize is the number of labels Cleanup moves or frees at once.
const cleanupBatchSize = 4096

// free frees the given standby labels in batches, so inserts and loads are blocked only for a short time.
func (lc *LabelsCompressor) free(standby map[prompb.Label]*labelEntry) int {
	idxs := make([]uint64, 0, len(standby))
	var sizeBytes uint64
	for label, e := range standby {
		idxs = append(idxs, e.idx)
		sizeBytes += labelEntrySizeBytes(label)
	}
	for batch := range slices.Chunk(idxs, cleanupBatchSize) {
		lc.idxToLabel.deleteBatch(batch)
	}
	lc.totalSizeBytes.Add(-sizeBytes)
	lc.itemsCount.Add(-uint64(len(idxs)))

	lc.mu.Lock()
	lc.freeIdxs = append(lc.freeIdxs, idxs...)
	lc.mu.Unlock()
	return len(idxs)
}

func cloneLabel(label prompb.Label) prompb.Label {
	// pre-allocate memory for label name and value
	n := len(label.Name) + len(label.Value)
	buf := make([]byte, 0, n)

	buf = append(buf, label.Name...)
	labelName := bytesutil.ToUnsafeString(buf)

	buf = append(buf, label.Value...)
	labelValue := bytesutil.ToUnsafeString(buf[len(labelName):])
	return prompb.Label{
		Name:  labelName,
		Value: labelValue,
	}
}

// Decompress decompresses src into []prompb.Label, appends it to dst and returns the result.
//
// It is safe calling Decompress from concurrent goroutines.
func (lc *LabelsCompressor) Decompress(dst []prompb.Label, src []byte) []prompb.Label {
	labelsLen, nSize := encoding.UnmarshalVarUint64(src)
	if nSize <= 0 {
		logger.Panicf("BUG: cannot unmarshal labels length from uvarint")
	}
	tail := src[nSize:]
	if labelsLen == 0 {
		// fast path - nothing to decode
		if len(tail) > 0 {
			logger.Panicf("BUG: unexpected non-empty tail left; len(tail)=%d; tail=%X", len(tail), tail)
		}
		return dst
	}

	a := encoding.GetUint64s(int(labelsLen))
	var err error
	tail, err = encoding.UnmarshalVarUint64s(a.A, tail)
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal label indexes: %s", err)
	}
	if len(tail) > 0 {
		logger.Panicf("BUG: unexpected non-empty tail left: len(tail)=%d; tail=%X", len(tail), tail)
	}
	dst = lc.decompress(dst, a.A)
	encoding.PutUint64s(a)
	return dst
}

func (lc *LabelsCompressor) decompress(dst []prompb.Label, src []uint64) []prompb.Label {
	for _, idx := range src {
		label, ok := lc.idxToLabel.Load(idx)
		if !ok {
			logger.Panicf("BUG: missing label for idx=%d", idx)
		}
		dst = append(dst, label)
	}
	return dst
}

// labelsMap maps uint64 key to prompb.Label
//
// uint64 keys must be packed close to 0. Otherwise the labelsMap structure will consume too much memory.
type labelsMap struct {
	readOnly atomic.Pointer[[]*prompb.Label]

	mutableLock sync.Mutex
	mutable     map[uint64]*prompb.Label
	misses      uint64
}

// Store stores label under the given idx.
//
// It is safe calling Store from concurrent goroutines.
func (lm *labelsMap) Store(idx uint64, label prompb.Label) {
	lm.mutableLock.Lock()
	if lm.mutable == nil {
		lm.mutable = make(map[uint64]*prompb.Label)
	}
	lm.mutable[idx] = &label
	lm.mutableLock.Unlock()
}

// Load returns the label for the given idx.
//
// Load returns false if lm doesn't contain label for the given idx.
//
// It is safe calling Load from concurrent goroutines.
//
// The performance of Load() scales linearly with CPU cores.
func (lm *labelsMap) Load(idx uint64) (prompb.Label, bool) {
	if pReadOnly := lm.readOnly.Load(); pReadOnly != nil && idx < uint64(len(*pReadOnly)) {
		if pLabel := (*pReadOnly)[idx]; pLabel != nil {
			// Fast path - the label for the given idx has been found in lm.readOnly.
			return *pLabel, true
		}
	}

	// Slow path - search in lm.mutable.
	return lm.loadSlow(idx)
}

func (lm *labelsMap) loadSlow(idx uint64) (prompb.Label, bool) {
	lm.mutableLock.Lock()

	// Try loading label from readOnly, since it could be updated while acquiring mutableLock.
	pReadOnly := lm.readOnly.Load()
	if pReadOnly != nil && idx < uint64(len(*pReadOnly)) {
		if pLabel := (*pReadOnly)[idx]; pLabel != nil {
			lm.mutableLock.Unlock()
			return *pLabel, true
		}
	}

	// The label for the idx wasn't found in readOnly. Search it in mutable.
	lm.misses++
	pLabel := lm.mutable[idx]
	if pReadOnly == nil || lm.misses > uint64(len(*pReadOnly)) {
		lm.moveMutableToReadOnlyLocked(pReadOnly)
		lm.misses = 0
	}
	lm.mutableLock.Unlock()

	if pLabel == nil {
		return prompb.Label{}, false
	}
	return *pLabel, true
}

// deleteBatch removes the labels at idxs, so the idxs can be used again.
//
// No key uses idxs, so nothing loads them and readOnly can be changed in place.
func (lm *labelsMap) deleteBatch(idxs []uint64) {
	lm.mutableLock.Lock()
	defer lm.mutableLock.Unlock()

	var readOnly []*prompb.Label
	if p := lm.readOnly.Load(); p != nil {
		readOnly = *p
	}
	for _, idx := range idxs {
		delete(lm.mutable, idx)
		if idx < uint64(len(readOnly)) {
			readOnly[idx] = nil
		}
	}
}

func (lm *labelsMap) moveMutableToReadOnlyLocked(pReadOnly *[]*prompb.Label) {
	if len(lm.mutable) == 0 {
		// Nothing to move
		return
	}

	var labels []*prompb.Label
	if pReadOnly != nil {
		labels = append(labels, *pReadOnly...)
	}
	for idx, pLabel := range lm.mutable {
		if idx < uint64(len(labels)) {
			labels[idx] = pLabel
		} else {
			for idx > uint64(len(labels)) {
				labels = append(labels, nil)
			}
			labels = append(labels, pLabel)
		}
	}
	clear(lm.mutable)
	lm.readOnly.Store(&labels)
}
