package remotewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
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
		labelsLen := len(labels)
		labels = append(labels, ts.Labels...)
		found := false
		for _, labelName := range obfuscateLabels {
			tmp := promrelabel.GetLabelByName(labels[labelsLen:], labelName)
			if tmp == nil {
				continue
			}
			found = true
			if obfuscatedValue, ok := olctx.cacheResults[tmp.Value]; ok {
				// fast path: the obfuscated result was calculated before
				tmp.Value = obfuscatedValue
				continue
			}
			res := sha256.Sum256(bytesutil.ToUnsafeBytes(tmp.Value))
			hashed := hex.EncodeToString(res[:])
			olctx.cacheResults[tmp.Value] = hashed
			tmp.Value = hashed
		}
		if found {
			ts.Labels = labels[labelsLen:]
		} else {
			labels = labels[:labelsLen]
		}
	}
	return tss
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
		if !slices.Contains(rwctx.obfuscateLabels, label) {
			rwctx.obfuscateLabels = append(rwctx.obfuscateLabels, label)
		}
	}
}
