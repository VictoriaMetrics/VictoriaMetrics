package remotewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

type obfuscateLabelsCtx struct {
	labels                []prompb.Label
	cacheObfuscatedResult map[string]string
}

func (ctx *obfuscateLabelsCtx) Reset() {
	promrelabel.CleanLabels(ctx.labels)
	ctx.labels = ctx.labels[:0]
	clear(ctx.cacheObfuscatedResult)
}

var obfuscateLabelsCtxPool = &sync.Pool{
	New: func() any {
		return &obfuscateLabelsCtx{
			cacheObfuscatedResult: make(map[string]string),
		}
	},
}

func getObfuscateLabelsCtx() *obfuscateLabelsCtx {
	return obfuscateLabelsCtxPool.Get().(*obfuscateLabelsCtx)
}

func putObfuscateLabelsCtx(ctx *obfuscateLabelsCtx) {
	ctx.Reset()
	obfuscateLabelsCtxPool.Put(ctx)
}

func (ctx *obfuscateLabelsCtx) apply(tss []prompb.TimeSeries, obfuscateLabels map[string]struct{}) []prompb.TimeSeries {
	if len(tss) == 0 {
		return tss
	}
	poolLabels := ctx.labels[:0]
	for i := range tss {
		ts := &tss[i]
		labels := ts.Labels
		j := 0
		needToObfuscate := false
		for ; j < len(labels); j++ {
			label := &labels[j]
			if _, ok := obfuscateLabels[label.Name]; !ok {
				continue
			}
			needToObfuscate = true
			break
		}
		if !needToObfuscate {
			continue
		}
		// Copy the label array to apply obfuscation
		poolLabelsLen := len(poolLabels)
		poolLabels = append(poolLabels, labels...)
		ctx.labels = poolLabels
		ts.Labels = poolLabels[poolLabelsLen:]
		for ; j < len(ts.Labels); j++ {
			label := &ts.Labels[j]
			if _, ok := obfuscateLabels[label.Name]; !ok {
				continue
			}
			if obfuscatedValue, ok := ctx.cacheObfuscatedResult[label.Value]; ok {
				// fast path: the obfuscated result was calculated before
				label.Value = obfuscatedValue
			} else {
				obfuscatedResult := sha256.Sum256([]byte(label.Value))
				ctx.cacheObfuscatedResult[label.Value] = hex.EncodeToString(obfuscatedResult[:])
				label.Value = ctx.cacheObfuscatedResult[label.Value]
			}
		}
	}
	return tss
}

func (rwctx *remoteWriteCtx) initObfuscationConfig() {
	if len(*obfuscationLabels) == 0 {
		return
	}
	idx := rwctx.idx
	rwctx.obfuscationLabels = make(map[string]struct{})
	rwObfuscationLabels := obfuscationLabels.GetOptionalArg(idx)
	rwObfuscationLabelsList := strings.Split(rwObfuscationLabels, "^^")

	for _, label := range rwObfuscationLabelsList {
		rwctx.obfuscationLabels[label] = struct{}{}
	}
}
