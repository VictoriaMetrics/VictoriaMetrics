package journald

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
)

func generateJournaldData(size int) []byte {
	var buf bytes.Buffer
	timestamp := time.Now().UnixMicro()
	binaryMsg := []byte("binary message data for performance test")
	var sizeBuf [8]byte

	for buf.Len() < size {
		timestamp++

		var entry string
		// Generate a mix of simple and binary messages
		if timestamp%10 == 0 {
			// Generate binary message
			binary.LittleEndian.PutUint64(sizeBuf[:], uint64(len(binaryMsg)))
			entry = fmt.Sprintf("__REALTIME_TIMESTAMP=%d\nMESSAGE\n%s%s\n\n",
				timestamp,
				sizeBuf[:],
				binaryMsg,
			)
		} else {
			// Generate simple message
			entry = fmt.Sprintf("__REALTIME_TIMESTAMP=%d\nMESSAGE=Performance test message %d\n\n", timestamp, timestamp)
		}
		if _, err := buf.WriteString(entry); err != nil {
			panic(fmt.Errorf("failed to write string to buffer: %w", err))
		}
	}
	return buf.Bytes()
}

func BenchmarkPushJournaldPerformance(b *testing.B) {
	cp := &insertutil.CommonParams{
		TimeFields: []string{"__REALTIME_TIMESTAMP"},
		MsgFields:  []string{"MESSAGE"},
	}
	blp := &insertutil.BenchmarkLogMessageProcessor{}
	const dataChunkSize = 100 * 1024 * 1024 // 100MB

	data := generateJournaldData(dataChunkSize)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.RunParallel(func(pb *testing.PB) {
		r := &bytes.Reader{}
		for pb.Next() {
			r.Reset(data)
			if err := processStreamInternal("performance_test", r, blp, cp); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
		}
	})
}
