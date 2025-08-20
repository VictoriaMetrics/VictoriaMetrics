package promutil

import (
	"log"
	"sync"
	"time"
	"weak"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

type Key struct {
	labelRefs []labelRef
}

func (k Key) Decompress() []prompb.Label {
	res := make([]prompb.Label, 0, len(k.labelRefs))
	for i := range k.labelRefs {
		res = append(res, cloneLabel(*k.labelRefs[i].label))
	}

	return res
}

type labelRef struct {
	label *prompb.Label
}

type LabelsCompressorV2 struct {
	mux    sync.Mutex
	labels map[prompb.Label]weak.Pointer[prompb.Label]
}

func NewLabelsCompressorV2() *LabelsCompressorV2 {
	lc := &LabelsCompressorV2{
		labels: make(map[prompb.Label]weak.Pointer[prompb.Label]),
	}

	go lc.cleanup()

	return lc
}

func (lc *LabelsCompressorV2) Compress(labels []prompb.Label) Key {
	lc.mux.Lock()
	defer lc.mux.Unlock()

	labelRefs := make([]labelRef, 0, len(labels))
	for i := range labels {
		wl := lc.labels[labels[i]]
		l := wl.Value()
		if l == nil {
			labelKey := cloneLabel(labels[i])
			labelVal := cloneLabel(labels[i])

			wl = weak.Make(&labelVal)
			lc.labels[labelKey] = wl

			l = wl.Value()
		}

		labelRefs = append(labelRefs, labelRef{
			label: l,
		})
	}

	return Key{
		labelRefs: labelRefs,
	}
}

func (lc *LabelsCompressorV2) cleanup() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			lc.Cleanup()
		}
	}
}

func (lc *LabelsCompressorV2) Cleanup() int {
	lc.mux.Lock()
	defer lc.mux.Unlock()

	count := 0

	for l, wl := range lc.labels {
		if wl.Value() != nil {
			continue
		}

		log.Println(l)

		count++
		delete(lc.labels, l)
	}

	return count
}
