package journald

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
)

func TestIsValidFieldName(t *testing.T) {
	f := func(name string, resultExpected bool) {
		t.Helper()

		result := isValidFieldName(name)
		if result != resultExpected {
			t.Fatalf("unexpected result for isValidJournaldFieldName(%q); got %v; want %v", name, result, resultExpected)
		}
	}

	f("", false)
	f("a", false)
	f("1", false)
	f("_", true)
	f("X", true)
	f("Xa", false)
	f("X_343", true)
	f("X_0123456789_AZ", true)
	f("SDDFD sdf", false)
}

func TestGetCommonParams_TimeField(t *testing.T) {
	f := func(timeFieldHeader, expectedTimeField string) {
		t.Helper()

		req, err := http.NewRequest("POST", "/insert/journald/upload", nil)
		if err != nil {
			t.Fatalf("unexpected error creating request: %s", err)
		}

		if timeFieldHeader != "" {
			req.Header.Set("VL-Time-Field", timeFieldHeader)
		}

		cp, err := getCommonParams(req)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if len(cp.TimeFields) != 1 || cp.TimeFields[0] != expectedTimeField {
			t.Fatalf("unexpected TimeFields; got %v; want [%s]", cp.TimeFields, expectedTimeField)
		}
	}

	// Test default behavior - when no custom time field is specified, journald uses __REALTIME_TIMESTAMP
	f("", "__REALTIME_TIMESTAMP")

	// Test custom time field - when a custom time field is specified via HTTP header, it's respected
	f("custom_time", "custom_time")
}

func TestPushJournald_Success(t *testing.T) {
	f := func(src string, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		tlp := &insertutil.TestLogMessageProcessor{}

		r, err := http.NewRequest("GET", "https://foo.bar/baz", nil)
		if err != nil {
			t.Fatalf("cannot create request: %s", err)
		}
		cp, err := getCommonParams(r)
		if err != nil {
			t.Fatalf("cannot create commonParams: %s", err)
		}

		buf := bytes.NewBufferString(src)
		if err := processStreamInternal("test", buf, tlp, cp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}

	// Single event
	f("__REALTIME_TIMESTAMP=91723819283\nMESSAGE=Test message\n\n",
		[]int64{91723819283000},
		"{\"_msg\":\"Test message\"}",
	)

	// Multiple events
	f("__REALTIME_TIMESTAMP=91723819283\nPRIORITY=3\nMESSAGE=Test message\n\n__REALTIME_TIMESTAMP=91723819284\nMESSAGE=Test message2\n",
		[]int64{91723819283000, 91723819284000},
		"{\"level\":\"error\",\"PRIORITY\":\"3\",\"_msg\":\"Test message\"}\n{\"_msg\":\"Test message2\"}",
	)

	// Parse binary data
	f("__CURSOR=s=e0afe8412a6a49d2bfcf66aa7927b588;i=1f06;b=f778b6e2f7584a77b991a2366612a7b5;m=300bdfd420;t=62526e1182354;x=930dc44b370963b7\nE=JobStateChanged\n__REALTIME_TIMESTAMP=1729698775704404\n__MONOTONIC_TIMESTAMP=206357648416\n__SEQNUM=7942\n__SEQNUM_ID=e0afe8412a6a49d2bfcf66aa7927b588\n_BOOT_ID=f778b6e2f7584a77b991a2366612a7b5\n_UID=0\n_GID=0\n_MACHINE_ID=a4a970370c30a925df02a13c67167847\n_HOSTNAME=ecd5e4555787\n_RUNTIME_SCOPE=system\n_TRANSPORT=journal\n_CAP_EFFECTIVE=1ffffffffff\n_SYSTEMD_CGROUP=/init.scope\n_SYSTEMD_UNIT=init.scope\n_SYSTEMD_SLICE=-.slice\nCODE_FILE=<stdin>\nCODE_LINE=1\nCODE_FUNC=<module>\nSYSLOG_IDENTIFIER=python3\n_COMM=python3\n_EXE=/usr/bin/python3.12\n_CMDLINE=python3\nMESSAGE\n\x13\x00\x00\x00\x00\x00\x00\x00foo\nbar\n\n\nasda\nasda\n_PID=2763\n_SOURCE_REALTIME_TIMESTAMP=1729698775704375\n\n",
		[]int64{1729698775704404000},
		"{\"E\":\"JobStateChanged\",\"_BOOT_ID\":\"f778b6e2f7584a77b991a2366612a7b5\",\"_UID\":\"0\",\"_GID\":\"0\",\"_MACHINE_ID\":\"a4a970370c30a925df02a13c67167847\",\"_HOSTNAME\":\"ecd5e4555787\",\"_RUNTIME_SCOPE\":\"system\",\"_TRANSPORT\":\"journal\",\"_CAP_EFFECTIVE\":\"1ffffffffff\",\"_SYSTEMD_CGROUP\":\"/init.scope\",\"_SYSTEMD_UNIT\":\"init.scope\",\"_SYSTEMD_SLICE\":\"-.slice\",\"CODE_FILE\":\"\\u003cstdin>\",\"CODE_LINE\":\"1\",\"CODE_FUNC\":\"\\u003cmodule>\",\"SYSLOG_IDENTIFIER\":\"python3\",\"_COMM\":\"python3\",\"_EXE\":\"/usr/bin/python3.12\",\"_CMDLINE\":\"python3\",\"_msg\":\"foo\\nbar\\n\\n\\nasda\\nasda\",\"_PID\":\"2763\",\"_SOURCE_REALTIME_TIMESTAMP\":\"1729698775704375\"}",
	)

	// Parse binary data with trailing newline
	f("__REALTIME_TIMESTAMP=1729698775704404\n_CMDLINE=python3\nMESSAGE\n\x14\x00\x00\x00\x00\x00\x00\x00foo\nbar\n\n\nasda\nasda\n\n_PID=2763\n\n",
		[]int64{1729698775704404000},
		`{"_CMDLINE":"python3","_msg":"foo\nbar\n\n\nasda\nasda\n","_PID":"2763"}`,
	)
	f("__REALTIME_TIMESTAMP=1729698775704404\n_CMDLINE=python3\nMESSAGE\n\x00\x00\x00\x00\x00\x00\x00\x00\n_PID=2763\n\n",
		[]int64{1729698775704404000},
		`{"_CMDLINE":"python3","_PID":"2763"}`,
	)
	f("__REALTIME_TIMESTAMP=1729698775704404\n_CMDLINE=python3\nMESSAGE\n\x0A\x00\x00\x00\x00\x00\x00\x00123456789\n\n_PID=2763\n\n",
		[]int64{1729698775704404000},
		`{"_CMDLINE":"python3","_msg":"123456789\n","_PID":"2763"}`,
	)
	f("__REALTIME_TIMESTAMP=1729698775704404\n_CMDLINE=python3\nMESSAGE\n\x0A\x00\x00\x00\x00\x00\x00\x001234567890\n_PID=2763\n\n",
		[]int64{1729698775704404000},
		`{"_CMDLINE":"python3","_msg":"1234567890","_PID":"2763"}`,
	)

	// Empty field name must be ignored
	f("__REALTIME_TIMESTAMP=91723819283\na=b\n=Test message", nil, "")
	f("__REALTIME_TIMESTAMP=91723819284\nMESSAGE=Test message2\n\n__REALTIME_TIMESTAMP=91723819283\n=Test message\n", []int64{91723819284000}, `{"_msg":"Test message2"}`)

	// field name starting with number must be ignored
	f("__REALTIME_TIMESTAMP=91723819283\n1incorrect=Test message\n\n__REALTIME_TIMESTAMP=91723819284\nMESSAGE=Test message2\n\n", []int64{91723819284000}, `{"_msg":"Test message2"}`)

	// field name exceeding 64 bytes limit must be ignored
	f("__REALTIME_TIMESTAMP=91723819283\ntoolooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooongcorrecooooooooooooong=Test message\n", nil, "")

	// field name with invalid chars must be ignored
	f("__REALTIME_TIMESTAMP=91723819283\nbadC!@$!@$as=Test message\n", nil, "")
}

func TestPushJournald_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		tlp := &insertutil.TestLogMessageProcessor{}

		r, err := http.NewRequest("GET", "https://foo.bar/baz", nil)
		if err != nil {
			t.Fatalf("cannot create request: %s", err)
		}
		cp, err := getCommonParams(r)
		if err != nil {
			t.Fatalf("cannot create commonParams: %s", err)
		}

		buf := bytes.NewBufferString(data)
		if err := processStreamInternal("test", buf, tlp, cp); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// too short binary encoded message
	f("__CURSOR=s=e0afe8412a6a49d2bfcf66aa7927b588;i=1f06;b=f778b6e2f7584a77b991a2366612a7b5;m=300bdfd420;t=62526e1182354;x=930dc44b370963b7\n__REALTIME_TIMESTAMP=1729698775704404\nMESSAGE\n\x13\x00\x00\x00\x00\x00\x00\x00foo\nbar\n\n\nasdaasd")
	f("__REALTIME_TIMESTAMP=1729698775704404\n_CMDLINE=python3\nMESSAGE\n\x00\x00\x00\x00\x00\x00\x00\x00_PID=2763\n\n")
	f("__REALTIME_TIMESTAMP=1729698775704404\n_CMDLINE=python3\nMESSAGE\n\x0A\x00\x00\x00\x00\x00\x00\x001234567890_PID=2763\n\n")
	f("__REALTIME_TIMESTAMP=1729698775704404\n_CMDLINE=python3\nMESSAGE\n\x0A\x00\x00\x00\x00\x00\x00\x00123456789\n_PID=2763\n\n")

	// too long binary encoded message
	f("__CURSOR=s=e0afe8412a6a49d2bfcf66aa7927b588;i=1f06;b=f778b6e2f7584a77b991a2366612a7b5;m=300bdfd420;t=62526e1182354;x=930dc44b370963b7\n__REALTIME_TIMESTAMP=1729698775704404\nMESSAGE\n\x13\x00\x00\x00\x00\x00\x00\x00foo\nbar\n\n\nasdaasdakljlsfd")
}
