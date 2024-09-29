package journald

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestPushJournaldOk(t *testing.T) {
	f := func(src string, timestampsExpected []int64, resultExpected string) {
		t.Helper()
		tlp := &insertutils.TestLogMessageProcessor{}
		cp := &insertutils.CommonParams{
			TimeField: "__REALTIME_TIMESTAMP",
			MsgField:  "MESSAGE",
		}
		n, err := parseJournaldRequest([]byte(src), tlp, cp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if err := tlp.Verify(n, timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}
	// Single event
	f("__REALTIME_TIMESTAMP=91723819283\nMESSAGE=Test message",
		[]int64{91723819283000},
		"{\"_msg\":\"Test message\"}",
	)

	// Multiple events
	f("__REALTIME_TIMESTAMP=91723819283\nMESSAGE=Test message\n\n__REALTIME_TIMESTAMP=91723819284\nMESSAGE=Test message2",
		[]int64{91723819283000, 91723819284000},
		"{\"_msg\":\"Test message\"}\n{\"_msg\":\"Test message2\"}",
	)
}
