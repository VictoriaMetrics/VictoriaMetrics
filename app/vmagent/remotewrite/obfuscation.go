package remotewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

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

func (rwctx *remoteWriteCtx) applyObfuscation(tss []prompb.TimeSeries) []prompb.TimeSeries {
	if len(rwctx.obfuscationLabels) == 0 || len(tss) == 0 {
		return tss
	}
	cacheObfuscatedResult := make(map[string]string)
	for i := range tss {
		ts := &tss[i]
		labels := ts.Labels
		for j := range labels {
			label := &labels[j]
			if _, ok := rwctx.obfuscationLabels[label.Name]; !ok {
				continue
			}
			if obfuscatedValue, ok := cacheObfuscatedResult[label.Value]; ok {
				// fast path: the obfuscated result was calculated before
				label.Value = obfuscatedValue
			} else {
				obfuscatedResult := sha256.Sum256([]byte(label.Value))
				cacheObfuscatedResult[label.Value] = hex.EncodeToString(obfuscatedResult[:])
				label.Value = cacheObfuscatedResult[label.Value]
			}
		}
	}
	return tss
}
