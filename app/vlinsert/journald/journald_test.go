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
			MsgFields: []string{"MESSAGE"},
		}
		if err := parseJournaldRequest([]byte(src), tlp, cp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}
	// Single event
	f("__REALTIME_TIMESTAMP=91723819283\nMESSAGE=Test message\n",
		[]int64{91723819283000},
		"{\"_msg\":\"Test message\"}",
	)

	// Multiple events
	f("__REALTIME_TIMESTAMP=91723819283\nMESSAGE=Test message\n\n__REALTIME_TIMESTAMP=91723819284\nMESSAGE=Test message2\n",
		[]int64{91723819283000, 91723819284000},
		"{\"_msg\":\"Test message\"}\n{\"_msg\":\"Test message2\"}",
	)

	// Parse binary data
	f("__CURSOR=s=e0afe8412a6a49d2bfcf66aa7927b588;i=1f06;b=f778b6e2f7584a77b991a2366612a7b5;m=300bdfd420;t=62526e1182354;x=930dc44b370963b7\n__REALTIME_TIMESTAMP=1729698775704404\n__MONOTONIC_TIMESTAMP=206357648416\n__SEQNUM=7942\n__SEQNUM_ID=e0afe8412a6a49d2bfcf66aa7927b588\n_BOOT_ID=f778b6e2f7584a77b991a2366612a7b5\n_UID=0\n_GID=0\n_MACHINE_ID=a4a970370c30a925df02a13c67167847\n_HOSTNAME=ecd5e4555787\n_RUNTIME_SCOPE=system\n_TRANSPORT=journal\n_CAP_EFFECTIVE=1ffffffffff\n_SYSTEMD_CGROUP=/init.scope\n_SYSTEMD_UNIT=init.scope\n_SYSTEMD_SLICE=-.slice\nCODE_FILE=<stdin>\nCODE_LINE=1\nCODE_FUNC=<module>\nSYSLOG_IDENTIFIER=python3\n_COMM=python3\n_EXE=/usr/bin/python3.12\n_CMDLINE=python3\nMESSAGE\n\x13\x00\x00\x00\x00\x00\x00\x00foo\nbar\n\n\nasda\nasda\n_PID=2763\n_SOURCE_REALTIME_TIMESTAMP=1729698775704375\n\n",
		[]int64{1729698775704404000},
		"{\"_BOOT_ID\":\"f778b6e2f7584a77b991a2366612a7b5\",\"_UID\":\"0\",\"_GID\":\"0\",\"_MACHINE_ID\":\"a4a970370c30a925df02a13c67167847\",\"_HOSTNAME\":\"ecd5e4555787\",\"_RUNTIME_SCOPE\":\"system\",\"_TRANSPORT\":\"journal\",\"_CAP_EFFECTIVE\":\"1ffffffffff\",\"_SYSTEMD_CGROUP\":\"/init.scope\",\"_SYSTEMD_UNIT\":\"init.scope\",\"_SYSTEMD_SLICE\":\"-.slice\",\"CODE_FILE\":\"\\u003cstdin>\",\"CODE_LINE\":\"1\",\"CODE_FUNC\":\"\\u003cmodule>\",\"SYSLOG_IDENTIFIER\":\"python3\",\"_COMM\":\"python3\",\"_EXE\":\"/usr/bin/python3.12\",\"_CMDLINE\":\"python3\",\"_msg\":\"foo\\nbar\\n\\n\\nasda\\nasda\",\"_PID\":\"2763\",\"_SOURCE_REALTIME_TIMESTAMP\":\"1729698775704375\"}",
	)
}

func TestPushJournald_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()
		tlp := &insertutils.TestLogMessageProcessor{}
		cp := &insertutils.CommonParams{
			TimeField: "__REALTIME_TIMESTAMP",
			MsgFields: []string{"MESSAGE"},
		}
		if err := parseJournaldRequest([]byte(data), tlp, cp); err == nil {
			t.Fatalf("expected non nil error")
		}
	}
	// missing new line terminator for binary encoded message
	f("__CURSOR=s=e0afe8412a6a49d2bfcf66aa7927b588;i=1f06;b=f778b6e2f7584a77b991a2366612a7b5;m=300bdfd420;t=62526e1182354;x=930dc44b370963b7\n__REALTIME_TIMESTAMP=1729698775704404\nMESSAGE\n\x13\x00\x00\x00\x00\x00\x00\x00foo\nbar\n\n\nasdaasda2")
	// missing new line terminator
	f("__REALTIME_TIMESTAMP=91723819283\n=Test message")
	// empty field name
	f("__REALTIME_TIMESTAMP=91723819283\n=Test message\n")
	// field name starting with number
	f("__REALTIME_TIMESTAMP=91723819283\n1incorrect=Test message\n")
	// field name exceeds 64 limit
	f("__REALTIME_TIMESTAMP=91723819283\ntoolooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooongcorrecooooooooooooong=Test message\n")
	// Only allow A-Z0-9 and '_'
	f("__REALTIME_TIMESTAMP=91723819283\nbadC!@$!@$as=Test message\n")
}
