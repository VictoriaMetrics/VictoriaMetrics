package streamaggr

import (
	"hash"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

type bimap struct {
	// map[prompbmarshal.Label]string
	labelToHash sync.Map
	// map[string]prompbmarshal.Label
	hashToLabel sync.Map
}

func (bm *bimap) getHash(l prompbmarshal.Label) string {
	key, ok := bm.labelToHash.Load(l)
	if !ok {
		return ""
	}
	return key.(string)
}

func (bm *bimap) getLabel(k string) prompbmarshal.Label {
	l, ok := bm.hashToLabel.Load(k)
	if !ok {
		return prompbmarshal.Label{}
	}
	return l.(prompbmarshal.Label)
}

func (bm *bimap) set(k string, l prompbmarshal.Label) {
	bm.labelToHash.Store(l, k)
	bm.hashToLabel.Store(k, l)
}

var bm bimap

func compress(bb []byte, lss []prompbmarshal.Label) []byte {
	var h hash.Hash32
	labelToKey := func(l prompbmarshal.Label) []byte {
		h.Reset()
		h.Write([]byte(l.Name))
		h.Write([]byte("="))
		h.Write([]byte(l.Value))
		return h.Sum(nil)
	}

	for _, ls := range lss {
		k := bm.getHash(ls)
		if k == "" {
			if h == nil {
				h = fnv.New32a()
			}
			k = string(labelToKey(ls))
			name, value := strings.Clone(ls.Name), strings.Clone(ls.Value)
			bm.set(k, prompbmarshal.Label{Name: name, Value: value})
		}
		bb = append(bb, k...)
	}
	return bb
}

func decompress(labels *promutils.Labels, s string) *promutils.Labels {
	bb := bytesutil.ToUnsafeBytes(s)
	for len(bb) != 0 {
		key := bb[:4]
		bb = bb[4:]
		l := bm.getLabel(bytesutil.ToUnsafeString(key))
		if l.Name == "" || l.Value == "" {
			panic("got empty label")
		}
		labels.Labels = append(labels.Labels, l)
	}
	return labels
}
