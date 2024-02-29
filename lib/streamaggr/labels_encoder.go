package streamaggr

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// labelsEncoder allows encoding list of prompbmarshal.Label into
// a hash key and vice versa.
//
// During encoding, each prompbmarshal.Label is encoded with incremented n.
// This allows to have as little memory footprint for hash key as possible.
// Generated hash keys are collision-free.
type labelsEncoder struct {
	// n is used as hash for prompbmarshal.Label entries.
	// It gets incremented for each unique prompbmarshal.Label.
	n atomic.Uint32
	// size represents amount of records in hashToLabel map
	// It gets incremented for each new registered prompbmarshal.Label entry.
	size atomic.Uint32

	// map[prompbmarshal.Label]string
	labelToHash sync.Map
	// map[string]prompbmarshal.Label
	hashToLabel sync.Map
}

func newLabelsEncoder(ms *metrics.Set, aggregator string) *labelsEncoder {
	le := &labelsEncoder{}

	ms.GetOrCreateGauge(fmt.Sprintf(`vmagent_streamaggr_labels_encoder_size{aggregator=%q}`, aggregator), func() float64 {
		return float64(le.size.Load())
	})

	return le
}

// encode encodes the given inputLabels and outputLabels lists into a byte slice.
// The resulting byte slice can be decoded back to lss via decode.
func (le *labelsEncoder) encode(bb []byte, inputLabels, outputLabels []prompbmarshal.Label) []byte {
	// encode size of the inputLabels as first value,
	// so it would be easy to distinguish lists without actual decoding.
	// see getInputOutputKeys().
	bb = binary.LittleEndian.AppendUint32(bb, uint32(len(inputLabels)))
	bb = le.encodeLabels(bb, inputLabels)
	bb = le.encodeLabels(bb, outputLabels)
	return bb
}

// getInputOutputKeys returns inputLabels and outputLabels encoded lists from encodedLabels.
// Each list can be independently decoded via decodeList func.
func getInputOutputKeys(encodedLabels string) (inputKey, outputKey string) {
	bb := bytesutil.ToUnsafeBytes(encodedLabels)
	inputKeyLength := binary.LittleEndian.Uint32(bb)
	offset := inputKeyLength * 4
	encodedLabels = encodedLabels[4:]
	return encodedLabels[:offset], encodedLabels[offset:]
}

func (le *labelsEncoder) encodeLabels(dst []byte, labels []prompbmarshal.Label) []byte {
	for _, ls := range labels {
		k, ok := le.labelToHash.Load(ls)
		if !ok {
			k = le.n.Add(1)
			ls = prompbmarshal.Label{
				Name:  strings.Clone(ls.Name),
				Value: strings.Clone(ls.Value),
			}
			key, loaded := le.labelToHash.LoadOrStore(ls, k)
			if loaded {
				// key could have been already created by concurrent goroutine - use it instead
				k = key
			} else {
				le.hashToLabel.Store(k, &ls)
				le.size.Add(1)
			}
		}
		dst = binary.LittleEndian.AppendUint32(dst, k.(uint32))
	}
	return dst
}

// decode decodes the given s into dst.
// It is expected that s was encoded via encode.
func (le *labelsEncoder) decode(dst []prompbmarshal.Label, s string) []prompbmarshal.Label {
	// skip first 4 bytes: the length of first encoded list
	return le.decodeList(dst, s[4:])
}

// decodeList decodes the given s into dst.
// It is expected that s was received via getInputOutputKeys
func (le *labelsEncoder) decodeList(dst []prompbmarshal.Label, s string) []prompbmarshal.Label {
	bb := bytesutil.ToUnsafeBytes(s)
	for len(bb) != 0 {
		k := binary.LittleEndian.Uint32(bb)
		bb = bb[4:]
		l, ok := le.hashToLabel.Load(k)
		if !ok {
			panic(fmt.Sprintf("failed to decode key: %d", k))
		}
		dst = append(dst, *(l.(*prompbmarshal.Label)))
	}
	return dst
}
