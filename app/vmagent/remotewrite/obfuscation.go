package remotewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func (rwctx *remoteWriteCtx) initObfuscationConfig() {
	idx := rwctx.idx
	if len(*obfuscatedLabels) != 0 {
		rwctx.obfuscatedLabels = make(map[string]struct{})
		rwObfuscatedLabels := obfuscatedLabels.GetOptionalArg(idx)
		rwObfuscatedLabelsList := strings.Split(rwObfuscatedLabels, "^^")
		for _, label := range rwObfuscatedLabelsList {
			rwctx.obfuscatedLabels[label] = struct{}{}
		}
	}
}

var obfuscationBufPool bytesutil.ByteBufferPool

func (rwctx *remoteWriteCtx) applyObfuscation(tss []prompb.TimeSeries) []prompb.TimeSeries {
	if len(rwctx.obfuscatedLabels) == 0 {
		return tss
	}
	if len(rwctx.obfuscatedLabels) == 0 {
		return tss
	}
	cacheObfuscatedResult := make(map[string]string)
	for i := range tss {
		ts := &tss[i]
		labels := ts.Labels
		for j := range labels {
			label := &labels[j]
			if _, ok := rwctx.obfuscatedLabels[label.Name]; !ok {
				continue
			}
			if obfuscatedValue, ok := cacheObfuscatedResult[label.Name]; ok {
				label.Value = obfuscatedValue
			} else {
				bb := obfuscationBufPool.Get()
				bb.Write([]byte(label.Value))
				obfuscatedResult := sha256.Sum256(bb.B)
				cacheObfuscatedResult[label.Name] = hex.EncodeToString(obfuscatedResult[:])
				label.Value = cacheObfuscatedResult[label.Name]
				obfuscationBufPool.Put(bb)
			}
		}
	}
	return tss
}
