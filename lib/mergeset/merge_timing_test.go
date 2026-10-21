package mergeset

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkMergeBlockStreamsCallbacks(b *testing.B) {
	b.Run("no-callbacks", func(b *testing.B) {
		benchmarkMergeBlockStreamsCallbacks(b, nil, nil)
	})
	b.Run("with-filterItem", func(b *testing.B) {
		var processedItems int
		filterItem := func(_ []byte) bool {
			processedItems++
			return true
		}
		benchmarkMergeBlockStreamsCallbacks(b, filterItem, nil)
	})
	b.Run("with-filterItem-drop-half", func(b *testing.B) {
		filterItem := func(item []byte) bool {
			return item[len(item)-1]%2 == 0
		}
		benchmarkMergeBlockStreamsCallbacks(b, filterItem, nil)
	})
	b.Run("with-prepareBlock", func(b *testing.B) {
		var totalItems, totalData int
		prepareBlock := func(data []byte, items []Item) ([]byte, []Item) {
			totalItems += len(items)
			totalData += len(data)
			return data, items
		}
		benchmarkMergeBlockStreamsCallbacks(b, nil, prepareBlock)
	})
	b.Run("with-all-callbacks", func(b *testing.B) {
		var processedItems int
		filterItem := func(_ []byte) bool {
			processedItems++
			return true
		}
		var totalItems, totalData int
		prepareBlock := func(data []byte, items []Item) ([]byte, []Item) {
			totalItems += len(items)
			totalData += len(data)
			return data, items
		}
		benchmarkMergeBlockStreamsCallbacks(b, filterItem, prepareBlock)
	})
}

func benchmarkMergeBlockStreamsCallbacks(b *testing.B, filterItem FilterItemCallback, prepareBlock PrepareBlockCallback) {
	const (
		itemsPerSource = 1000
		partsCount     = 100
	)

	ips := make([]*inmemoryPart, partsCount)
	for i := range ips {
		var ib inmemoryBlock
		for j := range itemsPerSource {
			item := fmt.Sprintf("metric_name_%08d", j*partsCount+i)
			if !ib.Add([]byte(item)) {
				break
			}
		}
		ips[i] = &inmemoryPart{}
		ips[i].Init(&ib)
	}

	bsrs := make([]*blockStreamReader, partsCount)
	for i := range bsrs {
		bsrs[i] = &blockStreamReader{}
	}
	var dstIP inmemoryPart
	var bsw blockStreamWriter
	var itemsMerged atomic.Uint64

	b.ReportAllocs()
	for b.Loop() {
		for i, ip := range ips {
			bsrs[i].MustInitFromInmemoryPart(ip)
		}
		dstIP.Reset()
		bsw.MustInitFromInmemoryPart(&dstIP, 0)
		if err := mergeBlockStreams(&dstIP.ph, &bsw, bsrs, filterItem, prepareBlock, b.Context().Done(), &itemsMerged); err != nil {
			b.Fatalf("cannot merge block streams: %s", err)
		}
	}
}
