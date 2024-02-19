package streamaggr

import (
	"encoding/binary"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"strings"
	"sync"
	"sync/atomic"
)

type bimap struct {
	n atomic.Uint32
	// map[prompbmarshal.Label]string
	labelToHash sync.Map
	// map[string]prompbmarshal.Label
	hashToLabel sync.Map
}

func (bm *bimap) getHash(l prompbmarshal.Label) uint32 {
	key, ok := bm.labelToHash.Load(l)
	if !ok {
		return 0
	}
	return key.(uint32)
}

func (bm *bimap) getLabel(k uint32) prompbmarshal.Label {
	l, ok := bm.hashToLabel.Load(k)
	if !ok {
		return prompbmarshal.Label{}
	}
	return l.(prompbmarshal.Label)
}

func (bm *bimap) loadOrStore(k uint32, l prompbmarshal.Label) uint32 {
	key, loaded := bm.labelToHash.LoadOrStore(l, k)
	if loaded {
		// key could have been already created by concurrent goroutine - use it instead
		k = key.(uint32)
	}
	bm.hashToLabel.Store(k, l)
	return k
}

func (bm *bimap) compress(bb []byte, lss []prompbmarshal.Label) []byte {
	for _, ls := range lss {
		k := bm.getHash(ls)
		if k == 0 {
			k = bm.n.Add(1)
			k = bm.loadOrStore(k, prompbmarshal.Label{
				Name:  strings.Clone(ls.Name),
				Value: strings.Clone(ls.Value),
			})
		}
		bb = binary.LittleEndian.AppendUint32(bb, k)
	}
	return bb
}

func (bm *bimap) decompress(labels *promutils.Labels, s string) *promutils.Labels {
	bb := bytesutil.ToUnsafeBytes(s)
	for len(bb) != 0 {
		k := binary.LittleEndian.Uint32(bb)
		bb = bb[4:]
		l := bm.getLabel(k)
		if l.Name == "" || l.Value == "" {
			panic(fmt.Sprintf("got empty label for key: %d", k))
		}
		labels.Labels = append(labels.Labels, l)
	}
	return labels
}
