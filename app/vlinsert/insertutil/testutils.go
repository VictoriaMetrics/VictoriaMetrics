package insertutil

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// TestLogMessageProcessor implements LogMessageProcessor for testing.
type TestLogMessageProcessor struct {
	timestamps []int64
	rows       []string
}

// AddRow adds row with the given timestamp and fields to tlp
func (tlp *TestLogMessageProcessor) AddRow(timestamp int64, fields, streamFields []logstorage.Field) {
	if streamFields != nil {
		panic(fmt.Errorf("BUG: streamFields must be nil; got %v", streamFields))
	}
	tlp.timestamps = append(tlp.timestamps, timestamp)
	tlp.rows = append(tlp.rows, string(logstorage.MarshalFieldsToJSON(nil, fields)))
}

// MustClose closes tlp.
func (tlp *TestLogMessageProcessor) MustClose() {
}

// Verify verifies the number of rows, timestamps and results after AddRow calls.
func (tlp *TestLogMessageProcessor) Verify(timestampsExpected []int64, resultExpected string) error {
	result := strings.Join(tlp.rows, "\n")
	if len(tlp.rows) != len(timestampsExpected) {
		return fmt.Errorf("unexpected rows read; got %d; want %d;\nrows read:\n%s\nrows wanted\n%s", len(tlp.rows), len(timestampsExpected), result, resultExpected)
	}

	if !reflect.DeepEqual(tlp.timestamps, timestampsExpected) {
		return fmt.Errorf("unexpected timestamps;\ngot\n%d\nwant\n%d", tlp.timestamps, timestampsExpected)
	}
	if result != resultExpected {
		return fmt.Errorf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
	}

	return nil
}

// BenchmarkLogMessageProcessor implements LogMessageProcessor for benchmarks.
type BenchmarkLogMessageProcessor struct{}

// AddRow implements LogMessageProcessor interface.
func (blp *BenchmarkLogMessageProcessor) AddRow(_ int64, _, _ []logstorage.Field) {
}

// MustClose implements LogMessageProcessor interface.
func (blp *BenchmarkLogMessageProcessor) MustClose() {
}
