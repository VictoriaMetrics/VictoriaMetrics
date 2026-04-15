package promutil

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

const minRotationInterval = time.Hour

// labelsCompressor compresses []prompb.Label into short binary strings.
type labelsCompressor struct {
	labelToIdx sync.Map
	idxToLabel labelsMap

	nextIdx atomic.Uint64

	totalSizeBytes atomic.Uint64
}

func (lc *labelsCompressor) sizeBytes() uint64 {
	return uint64(unsafe.Sizeof(*lc)) + lc.totalSizeBytes.Load()
}

func (lc *labelsCompressor) itemsCount() uint64 {
	return lc.nextIdx.Load()
}

// compress compresses labels, appends the compressed labels to dst and returns the result.
//
// It is safe calling compress from concurrent goroutines.
func (lc *labelsCompressor) compress(dst []byte, labels []prompb.Label) []byte {
	if len(labels) == 0 {
		// Fast path
		return append(dst, 0)
	}

	a := encoding.GetUint64s(len(labels) + 1)
	a.A[0] = uint64(len(labels))
	lc.compressInto(a.A[1:], labels)
	dst = encoding.MarshalVarUint64s(dst, a.A)
	encoding.PutUint64s(a)
	return dst
}

func (lc *labelsCompressor) compressInto(dst []uint64, labels []prompb.Label) {
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

// decompress decompresses src into []prompb.Label, appends it to dst and returns the result.
//
// It is safe calling decompress from concurrent goroutines.
func (lc *labelsCompressor) decompress(dst []prompb.Label, src []byte) []prompb.Label {
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
	dst = lc.decompressInternal(dst, a.A)
	encoding.PutUint64s(a)
	return dst
}

func (lc *labelsCompressor) decompressInternal(dst []prompb.Label, src []uint64) []prompb.Label {
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

// labelsCompressorState holds the current and previous labelsCompressor instances and generation byte that changes between rotations
// and is used to pick a right compressor during decompression
type labelsCompressorState struct {
	gen      byte
	current  *labelsCompressor
	previous *labelsCompressor
}

// LabelsCompressor is a rotating compressor that maintains two labelsCompressor
// instances to bound memory growth from stale label sets.
//
// Consumers must call Register on creation and Unregister on shutdown for a proper rotation period calculation.
type LabelsCompressor struct {
	state atomic.Pointer[labelsCompressorState]

	rotationInterval atomic.Int64
	startOnce        sync.Once

	registryMu sync.Mutex
	registry   []time.Duration
}

// getState returns current labelsCompressorState, which is initialized if needed.
func (lc *LabelsCompressor) getState() *labelsCompressorState {
	if s := lc.state.Load(); s != nil {
		return s
	}
	s := &labelsCompressorState{gen: 0, current: &labelsCompressor{}}
	// use CompareAndSwap to avoid overwriting pointer which could be stored by another thread
	lc.state.CompareAndSwap(nil, s)
	return lc.state.Load()
}

// rotate resets current compressor and moves its state to previous.
func (lc *LabelsCompressor) rotate() {
	old := lc.getState()
	lc.state.Store(&labelsCompressorState{
		gen:      old.gen ^ 1,
		current:  &labelsCompressor{},
		previous: old.current,
	})
}

// Register records maxStaleness for a new consumer, recomputes the rotation
// interval, starts the background rotation goroutine on the first call, and
// returns an id that must be passed to Unregister when the consumer stops.
func (lc *LabelsCompressor) Register(maxStaleness time.Duration) {
	lc.registryMu.Lock()
	lc.registry = append(lc.registry, maxStaleness)
	max := lc.maxStaleness()
	lc.registryMu.Unlock()

	lc.rotationInterval.Store(int64(max * 2))
	lc.startOnce.Do(func() {
		lc.getState()
		go func() {
			for {
				time.Sleep(time.Duration(lc.rotationInterval.Load()))
				lc.rotate()
			}
		}()
	})
}

// Unregister removes the given consumer ID from the registry and recomputes
// the rotation interval from the remaining registered consumers.
func (lc *LabelsCompressor) Unregister(maxStaleness time.Duration) {
	lc.registryMu.Lock()
	for i, s := range lc.registry {
		if s == maxStaleness {
			lc.registry = append(lc.registry[:i], lc.registry[i+1:]...)
			break
		}
	}
	max := lc.maxStaleness()
	lc.registryMu.Unlock()
	lc.rotationInterval.Store(int64(max * 2))
}

// maxStaleness returns the maximum staleness across all registered consumers.
// Must be called with registryMu held.
func (lc *LabelsCompressor) maxStaleness() time.Duration {
	maxStaleness := time.Duration(0)
	for _, d := range lc.registry {
		if d > maxStaleness {
			maxStaleness = d
		}
	}
	return max(maxStaleness, minRotationInterval)
}

// Compress appends the generation byte followed by the compressed labels
// to dst and returns the result.
//
// It is safe calling Compress from concurrent goroutines.
func (lc *LabelsCompressor) Compress(dst []byte, labels []prompb.Label) []byte {
	s := lc.getState()
	dst = append(dst, s.gen)
	return s.current.compress(dst, labels)
}

// Decompress reads the generation byte from key and decompresses the
// remaining bytes using the corresponding labelsCompressor instance.
func (lc *LabelsCompressor) Decompress(dst []prompb.Label, key []byte) []prompb.Label {
	if len(key) == 0 {
		logger.Panicf("BUG: unexpected empty key in Decompress")
	}
	gen := key[0]
	s := lc.getState()
	var c *labelsCompressor
	if s.gen == gen {
		c = s.current
	} else if s.previous != nil {
		c = s.previous
	} else {
		logger.Panicf("BUG: compressor for generation %d is not available; current generation is %d", gen, s.gen)
	}
	return c.decompress(dst, key[1:])
}

// SizeBytes returns the total memory used by the active compressor instances
func (lc *LabelsCompressor) SizeBytes() uint64 {
	s := lc.getState()
	n := s.current.sizeBytes()
	if s.previous != nil {
		n += s.previous.sizeBytes()
	}
	return n
}

// ItemsCount returns the total number of label entries stored across the active
func (lc *LabelsCompressor) ItemsCount() uint64 {
	s := lc.getState()
	n := s.current.itemsCount()
	if s.previous != nil {
		n += s.previous.itemsCount()
	}
	return n
}
