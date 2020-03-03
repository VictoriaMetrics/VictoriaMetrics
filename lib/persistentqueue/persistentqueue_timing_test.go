package persistentqueue

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func BenchmarkQueueThroughputSerial(b *testing.B) {
	const iterationsCount = 10
	for _, blockSize := range []int{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
		block := make([]byte, blockSize)
		b.Run(fmt.Sprintf("block-size-%d", blockSize), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(blockSize) * iterationsCount)
			path := fmt.Sprintf("bench-queue-throughput-serial-%d", blockSize)
			mustDeleteDir(path)
			q := MustOpen(path, "foobar", 0)
			defer func() {
				q.MustClose()
				mustDeleteDir(path)
			}()
			for i := 0; i < b.N; i++ {
				writeReadIteration(q, block, iterationsCount)
			}
		})
	}
}

func BenchmarkQueueThroughputConcurrent(b *testing.B) {
	const iterationsCount = 10
	for _, blockSize := range []int{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
		block := make([]byte, blockSize)
		b.Run(fmt.Sprintf("block-size-%d", blockSize), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(blockSize) * iterationsCount)
			path := fmt.Sprintf("bench-queue-throughput-concurrent-%d", blockSize)
			mustDeleteDir(path)
			q := MustOpen(path, "foobar", 0)
			defer func() {
				q.MustClose()
				mustDeleteDir(path)
			}()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					writeReadIteration(q, block, iterationsCount)
				}
			})
		})
	}
}

func writeReadIteration(q *Queue, block []byte, iterationsCount int) {
	for i := 0; i < iterationsCount; i++ {
		q.MustWriteBlock(block)
	}
	var ok bool
	bb := bbPool.Get()
	for i := 0; i < iterationsCount; i++ {
		bb.B, ok = q.MustReadBlock(bb.B[:0])
		if !ok {
			panic(fmt.Errorf("unexpected ok=false"))
		}
	}
	bbPool.Put(bb)
}

var bbPool bytesutil.ByteBufferPool
