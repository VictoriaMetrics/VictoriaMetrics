package promutils

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
	idxToLabel sync.Map

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
	for i := range labels {
		label := &labels[i]
		v, ok := lc.labelToIdx.Load(*label)
		if !ok {
			v = lc.nextIdx.Add(1)
			labelCopy := cloneLabel(label)
			lc.idxToLabel.Store(v, labelCopy)
			lc.labelToIdx.Store(*labelCopy, v)

			// Update lc.totalSizeBytes
			labelSizeBytes := uint64(len(label.Name) + len(label.Value))
			entrySizeBytes := labelSizeBytes + uint64(unsafe.Sizeof(label)+unsafe.Sizeof(*label)+2*unsafe.Sizeof(v))
			lc.totalSizeBytes.Add(entrySizeBytes)
		}
		dst[i] = v.(uint64)
	}
}

func cloneLabel(label *prompbmarshal.Label) *prompbmarshal.Label {
	// pre-allocate memory for label name and value
	n := len(label.Name) + len(label.Value)
	buf := make([]byte, 0, n)

	buf = append(buf, label.Name...)
	labelName := bytesutil.ToUnsafeString(buf)

	buf = append(buf, label.Value...)
	labelValue := bytesutil.ToUnsafeString(buf[len(labelName):])
	return &prompbmarshal.Label{
		Name:  labelName,
		Value: labelValue,
	}
}

// Decompress decompresses src into []prompbmarshal.Label, appends it to dst and returns the result.
func (lc *LabelsCompressor) Decompress(dst []prompbmarshal.Label, src []byte) []prompbmarshal.Label {
	tail, labelsLen, err := encoding.UnmarshalVarUint64(src)
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal labels length: %s", err)
	}
	if labelsLen == 0 {
		// fast path - nothing to decode
		if len(tail) > 0 {
			logger.Panicf("BUG: unexpected non-empty tail left; len(tail)=%d; tail=%X", len(tail), tail)
		}
		return dst
	}

	a := encoding.GetUint64s(int(labelsLen))
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
		v, ok := lc.idxToLabel.Load(idx)
		if !ok {
			logger.Panicf("BUG: missing label for idx=%d", idx)
		}
		label := *(v.(*prompbmarshal.Label))
		dst = append(dst, label)
	}
	return dst
}
