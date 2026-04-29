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

// LabelsCompressor compresses []prompb.Label into short binary strings.
// Zero value is ready to use. Each Register call must be paired with Unregister.
type LabelsCompressor struct {
	labelToIdx sync.Map
	idxToLabel atomic.Pointer[[]prompb.Label]

	// usedBitset tracks which indices were used in the current rotation period.
	// Bits are set atomically from compressFast without mu; grown under mu.
	usedBitset atomic.Pointer[[]uint64]

	totalSizeBytes uint64
	mu             sync.Mutex

	// freeIdxs holds index slots available for reuse.
	freeIdxs []uint32

	// pendingIdxs holds indices evicted from labelToIdx last rotate, not yet zeroed in idxToLabel.
	pendingIdxs map[uint32]struct{}

	// prevBitset is the usedBitset snapshot from the previous non-zero rotation.
	// Requires absence from both prevBitset and usedBitset to evict a label,
	// guarding against partial snapshots when rotate fires mid-compress loop.
	// Only accessed in rotate, which runs in a single goroutine.
	prevBitset []uint64

	// registry holds staleness intervals of active callers; rotation period = max(registry).
	registry []time.Duration

	// tickerCh signals runRotate to re-evaluate the rotation period.
	tickerCh chan struct{}
}

// idleRotationPeriod is the rotation period used when no callers are registered.
// It must be long enough that a sleeping goroutine does not consume measurable resources.
const idleRotationPeriod = time.Hour

// Register adds stalenessInterval to the registry and starts the rotation goroutine on first call.
// Rotation period equals max(registry). Must be paired with Unregister.
func (lc *LabelsCompressor) Register(stalenessInterval time.Duration) {
	lc.mu.Lock()
	if lc.tickerCh == nil {
		lc.tickerCh = make(chan struct{}, 1)
		go lc.runRotate()
	}
	lc.registry = append(lc.registry, stalenessInterval)
	tickerCh := lc.tickerCh
	lc.mu.Unlock()
	select {
	case tickerCh <- struct{}{}:
	default:
	}
}

// Unregister removes stalenessInterval from the registry.
// The rotation goroutine keeps running at idleRotationPeriod when the registry is empty.
func (lc *LabelsCompressor) Unregister(stalenessInterval time.Duration) {
	lc.mu.Lock()
	for i, d := range lc.registry {
		if d == stalenessInterval {
			lc.registry = append(lc.registry[:i], lc.registry[i+1:]...)
			break
		}
	}
	tickerCh := lc.tickerCh
	lc.mu.Unlock()
	if tickerCh != nil {
		select {
		case tickerCh <- struct{}{}:
		default:
		}
	}
}

func (lc *LabelsCompressor) maxRegisteredStaleness() time.Duration {
	// must be called with lc.mu held
	var max time.Duration
	for _, d := range lc.registry {
		if d > max {
			max = d
		}
	}
	if max == 0 {
		return idleRotationPeriod
	}
	return max
}

func (lc *LabelsCompressor) runRotate() {
	lc.mu.Lock()
	period := lc.maxRegisteredStaleness()
	lc.mu.Unlock()

	timer := time.NewTimer(period)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			lc.rotate()
			lc.mu.Lock()
			period = lc.maxRegisteredStaleness()
			lc.mu.Unlock()
			timer.Reset(period)
		case <-lc.tickerCh:
			lc.mu.Lock()
			newPeriod := lc.maxRegisteredStaleness()
			lc.mu.Unlock()
			if newPeriod != period {
				period = newPeriod
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(period)
			}
		}
	}
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
	lc.mu.Lock()
	p := lc.idxToLabel.Load()
	var n uint64
	if p != nil && len(*p) > len(lc.freeIdxs)+len(lc.pendingIdxs) {
		n = uint64(len(*p) - len(lc.freeIdxs) - len(lc.pendingIdxs))
	}
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
	p := lc.usedBitset.Load()
	var bits []uint64
	if p != nil {
		bits = *p
	}
	for i, label := range labels {
		v, ok := lc.labelToIdx.Load(label)
		if !ok {
			return false
		}
		idx := v.(uint32)
		dst[i] = uint64(idx)
		word := idx >> 6
		if int(word) < len(bits) {
			mask := uint64(1) << (idx & 63)
			if atomic.LoadUint64(&bits[word])&mask == 0 {
				atomic.OrUint64(&bits[word], mask)
			}
		}
	}
	// usedBitset was swapped mid-loop; re-mark all indices in the new bitset.
	if lc.usedBitset.Load() != p {
		for _, idx64 := range dst {
			lc.markUsed(uint32(idx64))
		}
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
		} else {
			lc.markUsed(v.(uint32))
		}
		dst[i] = uint64(v.(uint32))
	}
}

// markUsed sets the bit for idx in usedBitset. Safe to call without mu.
func (lc *LabelsCompressor) markUsed(idx uint32) {
	p := lc.usedBitset.Load()
	if p == nil {
		return
	}
	bits := *p
	word := idx >> 6
	if int(word) < len(bits) {
		mask := uint64(1) << (idx & 63)
		if atomic.LoadUint64(&bits[word])&mask == 0 {
			atomic.OrUint64(&bits[word], mask)
		}
	}
}

// growBitset extends usedBitset to cover idx. Must be called with lc.mu held.
func (lc *LabelsCompressor) growBitset(idx uint32) {
	needed := int(idx>>6) + 1
	p := lc.usedBitset.Load()
	var bits []uint64
	if p != nil {
		bits = *p
	}
	if needed <= len(bits) {
		return
	}
	newBits := make([]uint64, needed)
	copy(newBits, bits)
	lc.usedBitset.Store(&newBits)
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

	if len(lc.freeIdxs) > 0 {
		idx = lc.freeIdxs[len(lc.freeIdxs)-1]
		lc.freeIdxs = lc.freeIdxs[:len(lc.freeIdxs)-1]
		next := make([]prompb.Label, len(newIdxToLabel))
		copy(next, newIdxToLabel)
		next[idx] = cloned
		lc.idxToLabel.Store(&next)
	} else {
		idx = uint32(len(newIdxToLabel))
		next := append(newIdxToLabel, cloned)
		lc.idxToLabel.Store(&next)
	}

	lc.growBitset(idx)
	lc.markUsed(idx)
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
		return dst
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

// rotate evicts unused labels and recycles their slots.
// Phase 1 removes the label from labelToIdx (preventing new keys from referencing it).
// Phase 2, one rotation later, zeroes idxToLabel and returns the slot to freeIdxs,
// giving in-flight keys a full rotation period to drain via Decompress.
// Must not be called concurrently with itself.
func (lc *LabelsCompressor) rotate() {
	// Snapshot and reset usedBitset under mu to prevent races with growBitset/newIndex.
	// Also snapshot idxToLabel for the phase-1 scan.
	lc.mu.Lock()
	var currentBits []uint64
	if p := lc.usedBitset.Load(); p != nil {
		currentBits = make([]uint64, len(*p))
		copy(currentBits, *p)
		newBits := make([]uint64, len(*p))
		lc.usedBitset.Store(&newBits)
	}

	// Skip if nothing was used; all-zero snapshots from rapid rotation would
	// advance pendingIdxs and reclaim slots before in-flight keys drain.
	anyUsed := false
	for _, w := range currentBits {
		if w != 0 {
			anyUsed = true
			break
		}
	}
	if !anyUsed {
		lc.mu.Unlock()
		return
	}

	// Advance prevBitset; labels missing from a partial snapshot are still covered by it.
	prevSnap := lc.prevBitset
	lc.prevBitset = currentBits
	pendingIdxs := lc.pendingIdxs
	idxSnap := lc.idxToLabel.Load()
	lc.mu.Unlock()

	// A label is used if present in either the current or previous period snapshot.
	isUsed := func(idx uint32) bool {
		word := idx >> 6
		inCurrent := int(word) < len(currentBits) && currentBits[word]>>(idx&63)&1 == 1
		inPrev := int(word) < len(prevSnap) && prevSnap[word]>>(idx&63)&1 == 1
		return inCurrent || inPrev
	}

	// Phase 2: reclaim slots evicted last rotation if still unused.
	var toReclaim []uint32
	var nextRelease map[uint32]struct{}
	for idx := range pendingIdxs {
		if isUsed(idx) {
			if nextRelease == nil {
				nextRelease = make(map[uint32]struct{})
			}
			nextRelease[idx] = struct{}{}
		} else {
			toReclaim = append(toReclaim, idx)
		}
	}

	// Phase 1: scan idxToLabel rather than sync.Map.Range to avoid
	// cache-line contention with concurrent compressFast lookups.
	if idxSnap != nil {
		for i, label := range *idxSnap {
			if label == (prompb.Label{}) {
				continue // freed slot
			}
			idx := uint32(i)
			if _, inPending := pendingIdxs[idx]; inPending {
				continue // already queued for release
			}
			if isUsed(idx) {
				continue
			}
			lc.labelToIdx.Delete(label)
			if nextRelease == nil {
				nextRelease = make(map[uint32]struct{})
			}
			nextRelease[idx] = struct{}{}
		}
	}

	lc.mu.Lock()
	if len(toReclaim) > 0 {
		p := lc.idxToLabel.Load()
		var newIdxToLabel []prompb.Label
		if p != nil {
			newIdxToLabel = *p
		}
		next := make([]prompb.Label, len(newIdxToLabel))
		copy(next, newIdxToLabel)
		for _, idx := range toReclaim {
			label := next[idx]
			entrySize := uint64(len(label.Name)+len(label.Value)) + uint64(unsafe.Sizeof(label)) + 8
			if lc.totalSizeBytes >= entrySize {
				lc.totalSizeBytes -= entrySize
			}
			next[idx] = prompb.Label{}
			lc.freeIdxs = append(lc.freeIdxs, idx)
		}
		lc.idxToLabel.Store(&next)
	}
	lc.pendingIdxs = nextRelease
	lc.mu.Unlock()
}
