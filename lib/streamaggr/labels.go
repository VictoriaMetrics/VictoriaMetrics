package streamaggr

import (
	"encoding/binary"
	"flag"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

var streamAggrDedupCacheCleanInterval = flag.Duration("remoteWrite.streamAggr.dedupCacheCleanInterval", 5*time.Minute, "TODO")

type bimap struct {
	n atomic.Uint32
	// map[prompbmarshal.Label]string
	labelToHash sync.Map
	// map[string]prompbmarshal.Label
	hashToLabel sync.Map
}

var bm atomic.Pointer[bimap]

func init() {
	bm.Store(&bimap{})
	t := time.NewTicker(*streamAggrDedupCacheCleanInterval)
	go func() {
		for range t.C {
			bm.Store(&bimap{})
		}
	}()
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

func (bm *bimap) set(k uint32, l prompbmarshal.Label) {
	bm.labelToHash.Store(l, k)
	bm.hashToLabel.Store(k, l)
}

func (bm *bimap) compress(bb []byte, lss []prompbmarshal.Label) []byte {
	n := len(lss) * 4
	if cap(bb)-len(bb) < n {
		bb = append(make([]byte, 0, len(bb)+n), bb...)
	}
	for _, ls := range lss {
		k := bm.getHash(ls)
		if k == 0 {
			k = bm.n.Add(1)
			bm.set(k, prompbmarshal.Label{
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
			panic("got empty label")
		}
		labels.Labels = append(labels.Labels, l)
	}
	return labels
}
