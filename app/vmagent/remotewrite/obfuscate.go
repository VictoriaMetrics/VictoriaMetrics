package remotewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

type obfuscateLabelsCtx struct {
	labels       []prompb.Label
	cacheResults map[string]string
}

func (olctx *obfuscateLabelsCtx) reset() {
	promrelabel.CleanLabels(olctx.labels)
	olctx.labels = olctx.labels[:0]
	clear(olctx.cacheResults)
}

var obfuscateLabelsCtxPool = &sync.Pool{
	New: func() any {
		return &obfuscateLabelsCtx{
			cacheResults: make(map[string]string),
		}
	},
}

func getObfuscateLabelsCtx() *obfuscateLabelsCtx {
	return obfuscateLabelsCtxPool.Get().(*obfuscateLabelsCtx)
}

func putObfuscateLabelsCtx(ctx *obfuscateLabelsCtx) {
	ctx.reset()
	obfuscateLabelsCtxPool.Put(ctx)
}

func (olctx *obfuscateLabelsCtx) obfuscate(tss []prompb.TimeSeries, obfuscateLabels []string) []prompb.TimeSeries {
	if len(obfuscateLabels) == 0 || len(tss) == 0 {
		return tss
	}
	labels := olctx.labels[:0]
	for i := range tss {
		ts := &tss[i]
		if !olctx.shouldObfuscate(ts.Labels, obfuscateLabels) {
			continue
		}

		labelsLen := len(labels)
		labels = append(labels, ts.Labels...)
		for i := range obfuscateLabels {
			tmp := promrelabel.GetLabelByName(labels[labelsLen:], obfuscateLabels[i])
			if tmp == nil {
				continue
			}
			if obfuscatedValue, ok := olctx.cacheResults[tmp.Value]; ok {
				// fast path: the obfuscated result was calculated before
				tmp.Value = obfuscatedValue
				continue
			}
			res := sha256.Sum256(bytesutil.ToUnsafeBytes(tmp.Value))
			olctx.cacheResults[tmp.Value] = hex.EncodeToString(res[:])
			tmp.Value = olctx.cacheResults[tmp.Value]
		}
		ts.Labels = labels[labelsLen:]
	}
	return tss
}

func (olctx *obfuscateLabelsCtx) shouldObfuscate(labels []prompb.Label, obfuscateLabels []string) bool {
	for i := range obfuscateLabels {
		if tmp := promrelabel.GetLabelByName(labels, obfuscateLabels[i]); tmp == nil {
			continue
		}
		return true
	}
	return false
}

func (rwctx *remoteWriteCtx) initObfuscateLabels() {
	if len(*obfuscateLabels) == 0 {
		return
	}
	idx := rwctx.idx
	rwObfuscateLabels := obfuscateLabels.GetOptionalArg(idx)
	rwObfuscateLabelsList := strings.Split(rwObfuscateLabels, "^^")

	for _, label := range rwObfuscateLabelsList {
		if label == "" {
			continue
		}
		rwctx.obfuscateLabels = append(rwctx.obfuscateLabels, label)
	}
}
