package streamaggr

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

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
	n atomic.Uint32
	// map[prompbmarshal.Label]string
	labelToHash sync.Map
	// map[string]prompbmarshal.Label
	hashToLabel sync.Map
}

func (le *labelsEncoder) getKey(l prompbmarshal.Label) uint32 {
	key, ok := le.labelToHash.Load(l)
	if !ok {
		return 0
	}
	return key.(uint32)
}

func (le *labelsEncoder) getLabel(k uint32) prompbmarshal.Label {
	l, ok := le.hashToLabel.Load(k)
	if !ok {
		return prompbmarshal.Label{}
	}
	return l.(prompbmarshal.Label)
}

func (le *labelsEncoder) loadOrStore(k uint32, l prompbmarshal.Label) uint32 {
	key, loaded := le.labelToHash.LoadOrStore(l, k)
	if loaded {
		// key could have been already created by concurrent goroutine - use it instead
		k = key.(uint32)
	}
	le.hashToLabel.Store(k, l)
	return k
}

// encode encodes the given lss into a byte slice.
// The resulting byte slice can be decoded back to lss via decode.
func (le *labelsEncoder) encode(bb []byte, lss []prompbmarshal.Label) []byte {
	for _, ls := range lss {
		k := le.getKey(ls)
		if k == 0 {
			k = le.n.Add(1)
			k = le.loadOrStore(k, prompbmarshal.Label{
				Name:  strings.Clone(ls.Name),
				Value: strings.Clone(ls.Value),
			})
		}
		bb = binary.LittleEndian.AppendUint32(bb, k)
	}
	return bb
}

// decode decodes the given s into dst.
// It is expected that s was generated via encode.
func (le *labelsEncoder) decode(dst []prompbmarshal.Label, s string) ([]prompbmarshal.Label, error) {
	bb := bytesutil.ToUnsafeBytes(s)
	for len(bb) != 0 {
		k := binary.LittleEndian.Uint32(bb)
		bb = bb[4:]
		l := le.getLabel(k)
		if l.Name == "" || l.Value == "" {
			return nil, fmt.Errorf("failed to decode key: %d", k)
		}
		dst = append(dst, l)
	}
	return dst, nil
}
