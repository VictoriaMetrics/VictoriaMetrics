package promutil

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// LabelsCompressor compresses []prompb.Label into short binary strings
type LabelsCompressor struct {
	labelToIdx sync.Map
	idxToLabel atomic.Pointer[[]prompb.Label]

	totalSizeBytes uint64
	mu             sync.Mutex
	freeList       []uint32
	pendingDelete  map[uint32]struct{}
	pendingFree    map[uint32]struct{}
}

// SizeBytes returns the size of lc data in bytes
func (lc *LabelsCompressor) SizeBytes() uint64 {
	lc.mu.Lock()
	n := uint64(unsafe.Sizeof(*lc)) + lc.totalSizeBytes
	lc.mu.Unlock()
	return n
}

// ItemsCount returns the number of items in lc
func (lc *LabelsCompressor) ItemsCount() uint64 {
	p := lc.idxToLabel.Load()
	if p == nil {
		return 0
	}
	lc.mu.Lock()
	n := uint64(len(*p) - len(lc.freeList))
	lc.mu.Unlock()
	return n
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

	if lc.compressFast(a.A[1:], labels) {
		dst = encoding.MarshalVarUint64s(dst, a.A)
		encoding.PutUint64s(a)
		return dst
	}

	lc.mu.Lock()
	lc.compressSlow(a.A[1:], labels)
	lc.mu.Unlock()

	dst = encoding.MarshalVarUint64s(dst, a.A)
	encoding.PutUint64s(a)
	return dst
}

func (lc *LabelsCompressor) compressFast(dst []uint64, labels []prompb.Label) bool {
	for i, label := range labels {
		v, ok := lc.labelToIdx.Load(label)
		if !ok {
			return false
		}
		dst[i] = uint64(v.(uint32))
	}
	return true
}

func (lc *LabelsCompressor) compressSlow(dst []uint64, labels []prompb.Label) {
	for i, label := range labels {
		v, ok := lc.labelToIdx.Load(label)
		if !ok {
			cloned := cloneLabel(label)
			idx := lc.newIndex(cloned)
			lc.labelToIdx.Store(cloned, idx)
			v = idx
		}
		dst[i] = uint64(v.(uint32))
	}
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

// newIndex assigns a new index for cloned and updates idxToLabel atomically.
// Must be called with lc.mu held.
func (lc *LabelsCompressor) newIndex(cloned prompb.Label) uint32 {
	var idx uint32
	p := lc.idxToLabel.Load()
	var newIdxToLabel []prompb.Label
	if p != nil {
		newIdxToLabel = *p
	}

	if len(lc.freeList) > 0 {
		idx = lc.freeList[len(lc.freeList)-1]
		lc.freeList = lc.freeList[:len(lc.freeList)-1]
		next := make([]prompb.Label, len(newIdxToLabel))
		copy(next, newIdxToLabel)
		next[idx] = cloned
		lc.idxToLabel.Store(&next)
	} else {
		idx = uint32(len(newIdxToLabel))
		next := append(newIdxToLabel, cloned)
		lc.idxToLabel.Store(&next)
	}

	lc.totalSizeBytes += uint64(len(cloned.Name)+len(cloned.Value)) + uint64(unsafe.Sizeof(cloned)) + 8
	return idx
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

	p := lc.idxToLabel.Load()
	if p == nil {
		encoding.PutUint64s(a)
		logger.Panicf("BUG: idxToLabel is nil in Decompress")
	}
	labels := *p
	for _, idx := range a.A {
		if int(idx) >= len(labels) {
			encoding.PutUint64s(a)
			logger.Panicf("BUG: missing label for idx=%d; idxToLabel len=%d", idx, len(labels))
		}
		dst = append(dst, labels[idx])
	}

	encoding.PutUint64s(a)
	return dst
}

// Cleanup removes entries not referenced by any key in liveKeys.
// Pass nil or empty liveKeys to reclaim all entries.
func (lc *LabelsCompressor) Cleanup(liveKeys []string) {
	if len(liveKeys) == 0 {
		lc.labelToIdx.Range(func(k, _ any) bool {
			lc.labelToIdx.Delete(k)
			return true
		})
		lc.mu.Lock()
		empty := make([]prompb.Label, 0)
		lc.idxToLabel.Store(&empty)
		lc.freeList = lc.freeList[:0]
		lc.pendingDelete = nil
		lc.pendingFree = nil
		lc.totalSizeBytes = 0
		lc.mu.Unlock()
		return
	}

	usedIdxs := make(map[uint32]struct{}, len(liveKeys)*4)
	for _, key := range liveKeys {
		src := bytesutil.ToUnsafeBytes(key)
		labelsLen, n := encoding.UnmarshalVarUint64(src)
		if n <= 0 {
			logger.Panicf("BUG: cannot unmarshal labels length in Cleanup")
		}
		src = src[n:]
		for i := uint64(0); i < labelsLen; i++ {
			idx, n := encoding.UnmarshalVarUint64(src)
			if n <= 0 {
				logger.Panicf("BUG: cannot unmarshal label index in Cleanup")
			}
			usedIdxs[uint32(idx)] = struct{}{}
			src = src[n:]
		}
	}

	lc.mu.Lock()
	pendingDelete := lc.pendingDelete
	pendingFree := lc.pendingFree
	lc.mu.Unlock()

	// Phase 2→3: pendingFree entries are already deleted from labelToIdx;
	// safe to free the slot once no live key references the index.
	var freeIdxs []uint32
	var newPendingFree map[uint32]struct{}
	for idx := range pendingFree {
		if _, used := usedIdxs[idx]; used {
			if newPendingFree == nil {
				newPendingFree = make(map[uint32]struct{})
			}
			newPendingFree[idx] = struct{}{}
		} else {
			freeIdxs = append(freeIdxs, idx)
		}
	}

	// Phase 1→2 and new phase 1: iterate labelToIdx.
	var toPromote []uint32
	var newPendingDelete map[uint32]struct{}
	lc.labelToIdx.Range(func(k, v any) bool {
		idx := v.(uint32)
		if _, used := usedIdxs[idx]; used {
			return true
		}
		if _, pending := pendingDelete[idx]; pending {
			lc.labelToIdx.Delete(k)
			toPromote = append(toPromote, idx)
		} else {
			if newPendingDelete == nil {
				newPendingDelete = make(map[uint32]struct{})
			}
			newPendingDelete[idx] = struct{}{}
		}
		return true
	})

	lc.mu.Lock()
	if len(freeIdxs) > 0 {
		p := lc.idxToLabel.Load()
		var newIdxToLabel []prompb.Label
		if p != nil {
			newIdxToLabel = *p
		}
		next := make([]prompb.Label, len(newIdxToLabel))
		copy(next, newIdxToLabel)
		for _, idx := range freeIdxs {
			label := next[idx]
			entrySize := uint64(len(label.Name)+len(label.Value)) + uint64(unsafe.Sizeof(label)) + 8
			if lc.totalSizeBytes >= entrySize {
				lc.totalSizeBytes -= entrySize
			}
			next[idx] = prompb.Label{}
			lc.freeList = append(lc.freeList, idx)
		}
		lc.idxToLabel.Store(&next)
	}
	for _, idx := range toPromote {
		if newPendingFree == nil {
			newPendingFree = make(map[uint32]struct{})
		}
		newPendingFree[idx] = struct{}{}
	}
	lc.pendingDelete = newPendingDelete
	lc.pendingFree = newPendingFree
	lc.mu.Unlock()
}
