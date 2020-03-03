package persistentqueue

import (
	"fmt"
	"runtime"
	"testing"
)

func BenchmarkFastQueueThroughputSerial(b *testing.B) {
	const iterationsCount = 10
	for _, blockSize := range []int{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
		block := make([]byte, blockSize)
		b.Run(fmt.Sprintf("block-size-%d", blockSize), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(blockSize) * iterationsCount)
			path := fmt.Sprintf("bench-fast-queue-throughput-serial-%d", blockSize)
			mustDeleteDir(path)
			fq := MustOpenFastQueue(path, "foobar", iterationsCount*2, 0)
			defer func() {
				fq.MustClose()
				mustDeleteDir(path)
			}()
			for i := 0; i < b.N; i++ {
				writeReadIterationFastQueue(fq, block, iterationsCount)
			}
		})
	}
}

func BenchmarkFastQueueThroughputConcurrent(b *testing.B) {
	const iterationsCount = 10
	for _, blockSize := range []int{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
		block := make([]byte, blockSize)
		b.Run(fmt.Sprintf("block-size-%d", blockSize), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(blockSize) * iterationsCount)
			path := fmt.Sprintf("bench-fast-queue-throughput-concurrent-%d", blockSize)
			mustDeleteDir(path)
			fq := MustOpenFastQueue(path, "foobar", iterationsCount*runtime.GOMAXPROCS(-1)*2, 0)
			defer func() {
				fq.MustClose()
				mustDeleteDir(path)
			}()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					writeReadIterationFastQueue(fq, block, iterationsCount)
				}
			})
		})
	}
}

func writeReadIterationFastQueue(fq *FastQueue, block []byte, iterationsCount int) {
	for i := 0; i < iterationsCount; i++ {
		fq.MustWriteBlock(block)
	}
	var ok bool
	bb := bbPool.Get()
	for i := 0; i < iterationsCount; i++ {
		bb.B, ok = fq.MustReadBlock(bb.B[:0])
		if !ok {
			panic(fmt.Errorf("unexpected ok=false"))
		}
	}
	bbPool.Put(bb)
}
