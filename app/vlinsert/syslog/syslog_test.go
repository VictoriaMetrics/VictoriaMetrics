package syslog

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestSyslogLineReader_Success(t *testing.T) {
	f := func(data string, linesExpected []string) {
		t.Helper()

		r := bytes.NewBufferString(data)
		slr := getSyslogLineReader(r)
		defer putSyslogLineReader(slr)

		var lines []string
		for slr.nextLine() {
			lines = append(lines, string(slr.line))
		}
		if err := slr.Error(); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(lines, linesExpected) {
			t.Fatalf("unexpected lines read;\ngot\n%q\nwant\n%q", lines, linesExpected)
		}
	}

	f("", nil)
	f("\n", nil)
	f("\n\n\n", nil)

	f("foobar", []string{"foobar"})
	f("foobar\n", []string{"foobar\n"})
	f("\n\nfoo\n\nbar\n\n", []string{"foo\n\nbar\n\n"})

	f(`Jun  3 12:08:33 abcd systemd: Starting Update the local ESM caches...`, []string{"Jun  3 12:08:33 abcd systemd: Starting Update the local ESM caches..."})

	f(`Jun  3 12:08:33 abcd systemd: Starting Update the local ESM caches...

48 <165>Jun  4 12:08:33 abcd systemd[345]: abc defg<123>1 2023-06-03T17:42:12.345Z mymachine.example.com appname 12345 ID47 [exampleSDID@32473 iut="3" eventSource="Application 123 = ] 56" eventID="11211"] This is a test message with structured data.

`, []string{
		"Jun  3 12:08:33 abcd systemd: Starting Update the local ESM caches...",
		"<165>Jun  4 12:08:33 abcd systemd[345]: abc defg",
		`<123>1 2023-06-03T17:42:12.345Z mymachine.example.com appname 12345 ID47 [exampleSDID@32473 iut="3" eventSource="Application 123 = ] 56" eventID="11211"] This is a test message with structured data.`,
	})
}

func TestSyslogLineReader_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		r := bytes.NewBufferString(data)
		slr := getSyslogLineReader(r)
		defer putSyslogLineReader(slr)

		if slr.nextLine() {
			t.Fatalf("expecting failure to read the first line")
		}
		if err := slr.Error(); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// invalid format for message size
	f("12foo bar")

	// too big message size
	f("123 aa")
	f("1233423432 abc")
}

func TestProcessStreamInternal_Success(t *testing.T) {
	f := func(data string, currentYear, rowsExpected int, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		MustInit()
		defer MustStop()

		globalTimezone = time.UTC
		globalCurrentYear.Store(int64(currentYear))

		tlp := &insertutils.TestLogMessageProcessor{}
		r := bytes.NewBufferString(data)
		if err := processStreamInternal(r, "", false, tlp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := tlp.Verify(rowsExpected, timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}

	data := `Jun  3 12:08:33 abcd systemd: Starting Update the local ESM caches...

48 <165>Jun  4 12:08:33 abcd systemd[345]: abc defg<123>1 2023-06-03T17:42:12.345Z mymachine.example.com appname 12345 ID47 [exampleSDID@32473 iut="3" eventSource="Application 123 = ] 56" eventID="11211"] This is a test message with structured data.
`
	currentYear := 2023
	rowsExpected := 3
	timestampsExpected := []int64{1685794113000000000, 1685880513000000000, 1685814132345000000}
	resultExpected := `{"format":"rfc3164","timestamp":"","hostname":"abcd","app_name":"systemd","_msg":"Starting Update the local ESM caches..."}
{"priority":"165","facility":"20","severity":"5","format":"rfc3164","timestamp":"","hostname":"abcd","app_name":"systemd","proc_id":"345","_msg":"abc defg"}
{"priority":"123","facility":"15","severity":"3","format":"rfc5424","timestamp":"","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","exampleSDID@32473.iut":"3","exampleSDID@32473.eventSource":"Application 123 = ] 56","exampleSDID@32473.eventID":"11211","_msg":"This is a test message with structured data."}`
	f(data, currentYear, rowsExpected, timestampsExpected, resultExpected)
}

func TestProcessStreamInternal_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		MustInit()
		defer MustStop()

		tlp := &insertutils.TestLogMessageProcessor{}
		r := bytes.NewBufferString(data)
		if err := processStreamInternal(r, "", false, tlp); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// invalid format for message size
	f("12foo bar")

	// too big message size
	f("123 foo")
	f("123456789 bar")
}
