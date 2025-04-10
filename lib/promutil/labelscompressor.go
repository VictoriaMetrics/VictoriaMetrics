package promutil

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// LabelsCompressor compresses []prompbmarshal.Label into short binary strings
type LabelsCompressor struct {
	labelToIdx sync.Map
	idxToLabel labelsMap

	nextIdx atomic.Uint64

	totalSizeBytes atomic.Uint64
}

// SizeBytes returns the size of lc data in bytes
func (lc *LabelsCompressor) SizeBytes() uint64 {
	return uint64(unsafe.Sizeof(*lc)) + lc.totalSizeBytes.Load()
}

// ItemsCount returns the number of items in lc
func (lc *LabelsCompressor) ItemsCount() uint64 {
	return lc.nextIdx.Load()
}

// Compress compresses labels, appends the compressed labels to dst and returns the result.
//
// It is safe calling Compress from concurrent goroutines.
func (lc *LabelsCompressor) Compress(dst []byte, labels []prompbmarshal.Label) []byte {
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

func (lc *LabelsCompressor) compress(dst []uint64, labels []prompbmarshal.Label) {
	if len(labels) == 0 {
		return
	}
	_ = dst[len(labels)-1]
	for i, label := range labels {
		v, ok := lc.labelToIdx.Load(label)
		if !ok {
			idx := lc.nextIdx.Add(1)
			v = idx
			labelCopy := cloneLabel(label)

			// Must store idxToLabel entry before labelToIdx,
			// so it can be found by possible concurrent goroutines.
			//
			// We might store duplicated entries for single label with different indexes,
			// and it's fine, see https://github.com/VictoriaMetrics/VictoriaMetrics/pull/7118.
			lc.idxToLabel.Store(idx, labelCopy)
			vNew, loaded := lc.labelToIdx.LoadOrStore(labelCopy, v)
			if loaded {
				// This label has been stored by a concurrent goroutine with different index,
				// use it for key consistency in aggrState.
				v = vNew
			}

			// Update lc.totalSizeBytes
			labelSizeBytes := uint64(len(label.Name) + len(label.Value))
			entrySizeBytes := labelSizeBytes + uint64(2*(unsafe.Sizeof(label)+unsafe.Sizeof(&label))+unsafe.Sizeof(v))
			lc.totalSizeBytes.Add(entrySizeBytes)
		}
		dst[i] = v.(uint64)
	}
}

func cloneLabel(label prompbmarshal.Label) prompbmarshal.Label {
	// pre-allocate memory for label name and value
	n := len(label.Name) + len(label.Value)
	buf := make([]byte, 0, n)

	buf = append(buf, label.Name...)
	labelName := bytesutil.ToUnsafeString(buf)

	buf = append(buf, label.Value...)
	labelValue := bytesutil.ToUnsafeString(buf[len(labelName):])
	return prompbmarshal.Label{
		Name:  labelName,
		Value: labelValue,
	}
}

// Decompress decompresses src into []prompbmarshal.Label, appends it to dst and returns the result.
//
// It is safe calling Decompress from concurrent goroutines.
func (lc *LabelsCompressor) Decompress(dst []prompbmarshal.Label, src []byte) []prompbmarshal.Label {
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

func (lc *LabelsCompressor) decompress(dst []prompbmarshal.Label, src []uint64) []prompbmarshal.Label {
	for _, idx := range src {
		label, ok := lc.idxToLabel.Load(idx)
		if !ok {
			logger.Panicf("BUG: missing label for idx=%d", idx)
		}
		dst = append(dst, label)
	}
	return dst
}

// labelsMap maps uint64 key to prompbmarshal.Label
//
// uint64 keys must be packed close to 0. Otherwise the labelsMap structure will consume too much memory.
type labelsMap struct {
	readOnly atomic.Pointer[[]*prompbmarshal.Label]

	mutableLock sync.Mutex
	mutable     map[uint64]*prompbmarshal.Label
	misses      uint64
}

// Store stores label under the given idx.
//
// It is safe calling Store from concurrent goroutines.
func (lm *labelsMap) Store(idx uint64, label prompbmarshal.Label) {
	lm.mutableLock.Lock()
	if lm.mutable == nil {
		lm.mutable = make(map[uint64]*prompbmarshal.Label)
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
func (lm *labelsMap) Load(idx uint64) (prompbmarshal.Label, bool) {
	if pReadOnly := lm.readOnly.Load(); pReadOnly != nil && idx < uint64(len(*pReadOnly)) {
		if pLabel := (*pReadOnly)[idx]; pLabel != nil {
			// Fast path - the label for the given idx has been found in lm.readOnly.
			return *pLabel, true
		}
	}

	// Slow path - search in lm.mutable.
	return lm.loadSlow(idx)
}

func (lm *labelsMap) loadSlow(idx uint64) (prompbmarshal.Label, bool) {
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
		return prompbmarshal.Label{}, false
	}
	return *pLabel, true
}

func (lm *labelsMap) moveMutableToReadOnlyLocked(pReadOnly *[]*prompbmarshal.Label) {
	if len(lm.mutable) == 0 {
		// Nothing to move
		return
	}

	var labels []*prompbmarshal.Label
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
