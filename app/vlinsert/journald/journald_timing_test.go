package journald

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
)

func BenchmarkIsValidFieldName(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchmarkFields)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, field := range benchmarkFields {
				if !isValidFieldName(field) {
					panic(fmt.Errorf("cannot validate field %q", field))
				}
			}
		}
	})
}

var benchmarkFields = strings.Split(
	"E,_BOOT_ID,_UID,_GID,_MACHINE_ID,_HOSTNAME,_RUNTIME_SCOPE,_TRANSPORT,_CAP_EFFECTIVE,_SYSTEMD_CGROUP,_SYSTEMD_UNIT,"+
		"_SYSTEMD_SLICE,CODE_FILE,CODE_LINE,CODE_FUNC,SYSLOG_IDENTIFIER,_COMM,_EXE,_CMDLINE,MESSAGE,_PID,_SOURCE_REALTIME_TIMESTAMP,_REALTIME_TIMESTAMP",
	",")

func BenchmarkPushJournaldPerformance(b *testing.B) {
	cp := &insertutil.CommonParams{
		TimeFields: []string{"__REALTIME_TIMESTAMP"},
		MsgFields:  []string{"MESSAGE"},
	}
	const dataChunkSize = 1024 * 1024

	data := generateJournaldData(dataChunkSize)

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.RunParallel(func(pb *testing.PB) {
		r := &bytes.Reader{}
		blp := &insertutil.BenchmarkLogMessageProcessor{}
		for pb.Next() {
			r.Reset(data)
			if err := processStreamInternal("performance_test", r, blp, cp); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
		}
	})
}

func generateJournaldData(size int) []byte {
	var buf []byte
	timestamp := time.Now().UnixMicro()
	binaryMsg := []byte("binary message data for performance test")
	var sizeBuf [8]byte

	for len(buf) < size {
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
		buf = append(buf, entry...)
	}
	return buf
}
