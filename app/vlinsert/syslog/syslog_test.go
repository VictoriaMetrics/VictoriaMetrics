package syslog

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func TestReadLine_Success(t *testing.T) {
	f := func(data string, currentYear, rowsExpected int, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		var timestamps []int64
		var result string
		processLogMessage := func(timestamp int64, fields []logstorage.Field) {
			timestamps = append(timestamps, timestamp)
			result += string(logstorage.MarshalFieldsToJSON(nil, fields)) + "\n"
		}

		r := bytes.NewBufferString(data)
		sc := bufio.NewScanner(r)
		rows := 0
		for {
			ok, err := readLine(sc, currentYear, processLogMessage)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !ok {
				break
			}
			rows++
		}
		if rows != rowsExpected {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, rowsExpected)
		}

		if !reflect.DeepEqual(timestamps, timestampsExpected) {
			t.Fatalf("unexpected timestamps;\ngot\n%d\nwant\n%d", timestamps, timestampsExpected)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	data := `Jun  3 12:08:33 abcd systemd: Starting Update the local ESM caches...
<165>Jun  4 12:08:33 abcd systemd[345]: abc defg
<123>1 2023-06-03T17:42:12.345Z mymachine.example.com appname 12345 ID47 [exampleSDID@32473 iut="3" eventSource="Application 123 = ] 56" eventID="11211"] This is a test message with structured data.
`
	currentYear := 2023
	rowsExpected := 3
	timestampsExpected := []int64{1685794113000000000, 1685880513000000000, 1685814132345000000}
	resultExpected := `{"format":"rfc3164","timestamp":"","hostname":"abcd","app_name":"systemd","_msg":"Starting Update the local ESM caches..."}
{"priority":"165","facility":"20","severity":"5","format":"rfc3164","timestamp":"","hostname":"abcd","app_name":"systemd","proc_id":"345","_msg":"abc defg"}
{"priority":"123","facility":"15","severity":"3","format":"rfc5424","timestamp":"","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","exampleSDID@32473":"iut=\"3\" eventSource=\"Application 123 = ] 56\" eventID=\"11211\"","_msg":"This is a test message with structured data."}
`
	f(data, currentYear, rowsExpected, timestampsExpected, resultExpected)
}
