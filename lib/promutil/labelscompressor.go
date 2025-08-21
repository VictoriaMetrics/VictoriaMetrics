package promutil

import (
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/cespare/xxhash/v2"
)

var hashBBP = bytesutil.ByteBufferPool{}

type storedLabel struct {
	prompb.Label
	used atomic.Bool
}

type state struct {
	inUse   *sync.Map // map[uint64]*storedLabel
	deleted *sync.Map // map[uint64]*storedLabel
}

type LabelsCompressor struct {
	state atomic.Pointer[state]

	totalSizeBytes atomic.Uint64
	totalItems     atomic.Uint64
}

func NewLabelsCompressor() *LabelsCompressor {
	lc := &LabelsCompressor{}
	lc.state.Store(&state{
		inUse:   &sync.Map{},
		deleted: &sync.Map{},
	})

	go lc.cleanupLoop()
	return lc
}

// SizeBytes returns the size of lc data in bytes
func (lc *LabelsCompressor) SizeBytes() uint64 {
	return uint64(unsafe.Sizeof(*lc)) + lc.totalSizeBytes.Load()
}

// ItemsCount returns the number of items in lc
func (lc *LabelsCompressor) ItemsCount() uint64 {
	return lc.totalItems.Load()
}

func (lc *LabelsCompressor) Compress(dst []byte, labels []prompb.Label) []byte {
	if len(labels) == 0 {
		// Fast path
		return append(dst, 0)
	}

	a := encoding.GetUint64s(len(labels) + 2)
	a.A[0] = uint64(len(labels))
	lc.compress(a.A[2:], labels)

	minA := uint64(math.MaxUint64)
	for _, v := range a.A[2:] {
		minA = min(minA, v)
	}
	a.A[1] = minA
	for i := range a.A[2:] {
		a.A[i+2] -= minA
	}

	dst = encoding.MarshalVarUint64s(dst, a.A)
	encoding.PutUint64s(a)
	return dst
}

func (lc *LabelsCompressor) compress(dst []uint64, labels []prompb.Label) {
	if len(labels) == 0 {
		return
	}

	var maxSize int
	for i := range labels {
		maxSize = max(maxSize, len(labels[i].Name)+len(labels[i].Value))
	}
	maxSize += 3 // for 2x'=' and collisionIdx

	bb := hashBBP.Get()
	defer hashBBP.Put(bb)
	bb.Grow(maxSize)

	state := lc.state.Load()

	_ = dst[len(labels)-1]

	var totalSizeBytes, totalItems uint64
	for i, label := range labels {
		var loaded bool
		dst[i], loaded = lc.compressLabel(label, state, bb)
		if !loaded {
			// Update lc.totalSizeBytes
			labelSizeBytes := uint64(len(label.Name) + len(label.Value))
			entrySizeBytes := labelSizeBytes + uint64(2*(unsafe.Sizeof(label)+unsafe.Sizeof(&label))+unsafe.Sizeof(label))
			totalSizeBytes += entrySizeBytes

			totalItems += 1
		}
	}

	if totalItems > 0 {
		lc.totalSizeBytes.Add(totalSizeBytes)
		lc.totalItems.Add(totalItems)
	}
}

func (lc *LabelsCompressor) compressLabel(label prompb.Label, state *state, bb *bytesutil.ByteBuffer) (uint64, bool) {
	var collisionIdx int64
	for i := 0; i < 5; i++ {
		bb.Reset()
		bb.Write(bytesutil.ToUnsafeBytes(label.Name))
		bb.Write([]byte(`=`))
		bb.B = binary.AppendVarint(bb.B, collisionIdx)
		bb.Write([]byte(`=`))
		bb.Write(bytesutil.ToUnsafeBytes(label.Value))

		idx := xxhash.Sum64(bb.B)

		sl0, loaded := state.inUse.Load(idx)
		//fast path
		if loaded {
			sl := sl0.(*storedLabel)
			// hash collision detected
			if sl.Name != label.Name || sl.Value != label.Value {
				collisionIdx++
				continue
			}

			return ptrToUint64(unsafe.Pointer(sl)), true
		}

		sl0, loaded = state.deleted.Load(idx)
		if loaded {
			sl := sl0.(*storedLabel)
			if sl.Name != label.Name || sl.Value != label.Value {
				collisionIdx++
				continue
			}
			sl.used.Store(true)
		} else {
			sl := &storedLabel{
				Label: cloneLabel(label),
			}
			sl.used.Store(true)
			sl0 = sl
		}

		sl0, loaded = state.inUse.LoadOrStore(idx, sl0)
		sl := sl0.(*storedLabel)
		if loaded {
			if sl.Name != label.Name || sl.Value != label.Value {
				collisionIdx++
				continue
			}
		}

		return ptrToUint64(unsafe.Pointer(sl)), loaded
	}

	panic("FATAL: too many hash collisions detected")
}

//go:unsafe
func ptrToUint64(p unsafe.Pointer) uint64 {
	return uint64(uintptr(p))
}

//go:unsafe
//go:nocheckptr
func uint64ToPtr(u uint64) unsafe.Pointer {
	return unsafe.Pointer(uintptr(u))
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
	minA, nSize := encoding.UnmarshalVarUint64(tail)
	if nSize <= 0 {
		logger.Panicf("BUG: cannot unmarshal minA from uvarint")
	}
	tail = tail[nSize:]

	a := encoding.GetUint64s(int(labelsLen))
	var err error
	tail, err = encoding.UnmarshalVarUint64s(a.A, tail)
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal label indexes: %s", err)
	}
	if len(tail) > 0 {
		logger.Panicf("BUG: unexpected non-empty tail left: len(tail)=%d; tail=%X", len(tail), tail)
	}

	for i := range a.A {
		a.A[i] += minA
	}

	dst = lc.decompress(dst, a.A)
	encoding.PutUint64s(a)
	return dst
}

func (lc *LabelsCompressor) decompress(dst []prompb.Label, src []uint64) []prompb.Label {
	for _, ref := range src {
		sl := (*storedLabel)(uint64ToPtr(ref))
		if !sl.used.Load() {
			sl.used.Store(true)
		}
		dst = append(dst, sl.Label)
	}
	return dst
}

func (lc *LabelsCompressor) cleanupLoop() {
	// ticker should be 3x bigger than any aggr interval
	t := time.NewTicker(time.Minute)
	mark := true
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if mark {
				lc.markUnused()
				mark = false
			} else {
				lc.moveUnused()
				mark = true
			}
		}
	}
}

func (lc *LabelsCompressor) markUnused() {
	start := time.Now()
	lcState := lc.state.Load()
	var marked int
	lcState.inUse.Range(func(_, label0 interface{}) bool {
		label0.(*storedLabel).used.Store(false)
		marked++
		return true
	})

	lc.state.Store(&state{
		inUse:   lcState.inUse,
		deleted: &sync.Map{},
	})

	logger.Infof("marked %d in %s", marked, time.Since(start))
}

func (lc *LabelsCompressor) moveUnused() {
	start := time.Now()
	state := lc.state.Load()
	var moved int
	state.inUse.Range(func(key0, label0 interface{}) bool {
		sl := label0.(*storedLabel)
		if !sl.used.Load() {
			// move unused label to deleted
			state.deleted.Store(key0, sl)
			state.inUse.Delete(key0)
			moved++
			return true
		}

		return true
	})

	logger.Infof("moved %d in %s", moved, time.Since(start))
}
