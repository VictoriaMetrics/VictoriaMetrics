package logstorage

import (
	"testing"
	"time"
)

func TestSyslogParser(t *testing.T) {
	f := func(s string, timezone *time.Location, resultExpected string) {
		t.Helper()

		const currentYear = 2024
		p := GetSyslogParser(currentYear, timezone)
		defer PutSyslogParser(p)

		p.Parse(s)
		result := MarshalFieldsToJSON(nil, p.Fields)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result when parsing [%s]; got\n%s\nwant\n%s\n", s, result, resultExpected)
		}
	}

	// RFC 3164
	f("Jun  3 12:08:33 abcd systemd[1]: Starting Update the local ESM caches...", time.UTC,
		`{"format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"systemd","proc_id":"1","message":"Starting Update the local ESM caches..."}`)
	f("<165>Jun  3 12:08:33 abcd systemd[1]: Starting Update the local ESM caches...", time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"systemd","proc_id":"1","message":"Starting Update the local ESM caches..."}`)
	f("Mar 13 12:08:33 abcd systemd: Starting Update the local ESM caches...", time.UTC,
		`{"format":"rfc3164","timestamp":"2024-03-13T12:08:33.000Z","hostname":"abcd","app_name":"systemd","message":"Starting Update the local ESM caches..."}`)
	f("Jun  3 12:08:33 abcd - Starting Update the local ESM caches...", time.UTC,
		`{"format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"-","message":"Starting Update the local ESM caches..."}`)
	f("Jun  3 12:08:33 - - Starting Update the local ESM caches...", time.UTC,
		`{"format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z","hostname":"-","app_name":"-","message":"Starting Update the local ESM caches..."}`)

	// RFC 5424
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data.`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","message":"This is a test message with structured data."}`)
	f(`1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data.`, time.UTC,
		`{"format":"rfc5424","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","message":"This is a test message with structured data."}`)
	f(`<165>1 2023-06-03T17:42:00.000Z mymachine.example.com appname 12345 ID47 [exampleSDID@32473 iut="3" eventSource="Application 123 = ] 56" eventID="11211"] This is a test message with structured data.`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:00.000Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","exampleSDID@32473.iut":"3","exampleSDID@32473.eventSource":"Application 123 = ] 56","exampleSDID@32473.eventID":"11211","message":"This is a test message with structured data."}`)
	f(`<165>1 2023-06-03T17:42:00.000Z mymachine.example.com appname 12345 ID47 [foo@123 iut="3"][bar@456 eventID="11211"] This is a test message with structured data.`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:00.000Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","foo@123.iut":"3","bar@456.eventID":"11211","message":"This is a test message with structured data."}`)

	// Incomplete RFC 3164
	f("", time.UTC, `{}`)
	f("Jun  3 12:08:33", time.UTC, `{"format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z"}`)
	f("Foo  3 12:08:33", time.UTC, `{"format":"rfc3164","message":"Foo  3 12:08:33"}`)
	f("Foo  3 12:08:33bar", time.UTC, `{"format":"rfc3164","message":"Foo  3 12:08:33bar"}`)
	f("Jun  3 12:08:33 abcd", time.UTC, `{"format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd"}`)
	f("Jun  3 12:08:33 abcd sudo", time.UTC, `{"format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"sudo"}`)
	f("Jun  3 12:08:33 abcd sudo[123]", time.UTC, `{"format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"sudo","proc_id":"123"}`)
	f("Jun  3 12:08:33 abcd sudo foobar", time.UTC, `{"format":"rfc3164","timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"sudo","message":"foobar"}`)
	f(`foo bar baz`, time.UTC, `{"format":"rfc3164","message":"foo bar baz"}`)

	// Incomplete RFC 5424
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 [foo@123]`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","foo@123":""}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47"}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345"}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname"}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com"}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z`, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424","timestamp":"2023-06-03T17:42:32.123456789Z"}`)
	f(`<165>1 `, time.UTC,
		`{"priority":"165","facility":"20","severity":"5","format":"rfc5424"}`)
}
