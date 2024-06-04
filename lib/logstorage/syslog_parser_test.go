package logstorage

import (
	"testing"
)

func TestSyslogParser(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()

		const currentYear = 2024
		p := getSyslogParser(currentYear)
		defer putSyslogParser(p)

		p.parse(s)
		result := MarshalFieldsToJSON(nil, p.fields)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result when parsing [%s]; got\n%s\nwant\n%s\n", s, result, resultExpected)
		}
	}

	// RFC 3164
	f("Jun  3 12:08:33 abcd systemd[1]: Starting Update the local ESM caches...",
		`{"timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"systemd","proc_id":"1","message":"Starting Update the local ESM caches..."}`)
	f("<165>Jun  3 12:08:33 abcd systemd[1]: Starting Update the local ESM caches...",
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"systemd","proc_id":"1","message":"Starting Update the local ESM caches..."}`)
	f("Mar 13 12:08:33 abcd systemd: Starting Update the local ESM caches...",
		`{"timestamp":"2024-03-13T12:08:33.000Z","hostname":"abcd","app_name":"systemd","message":"Starting Update the local ESM caches..."}`)
	f("Jun  3 12:08:33 abcd - Starting Update the local ESM caches...",
		`{"timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"-","message":"Starting Update the local ESM caches..."}`)
	f("Jun  3 12:08:33 - - Starting Update the local ESM caches...",
		`{"timestamp":"2024-06-03T12:08:33.000Z","hostname":"-","app_name":"-","message":"Starting Update the local ESM caches..."}`)

	// RFC 5424
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data.`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","message":"This is a test message with structured data."}`)
	f(`1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data.`,
		`{"timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","message":"This is a test message with structured data."}`)
	f(`<165>1 2023-06-03T17:42:00.000Z mymachine.example.com appname 12345 ID47 [exampleSDID@32473 iut="3" eventSource="Application 123 = ] 56" eventID="11211"] This is a test message with structured data.`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:00.000Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","exampleSDID@32473":"iut=\"3\" eventSource=\"Application 123 = ] 56\" eventID=\"11211\"","message":"This is a test message with structured data."}`)
	f(`<165>1 2023-06-03T17:42:00.000Z mymachine.example.com appname 12345 ID47 [foo@123 iut="3"][bar@456 eventID="11211"] This is a test message with structured data.`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:00.000Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","foo@123":"iut=\"3\"","bar@456":"eventID=\"11211\"","message":"This is a test message with structured data."}`)

	// Incomplete RFC 3164
	f("", `{}`)
	f("Jun  3 12:08:33", `{"timestamp":"2024-06-03T12:08:33.000Z"}`)
	f("Jun  3 12:08:33 abcd", `{"timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd"}`)
	f("Jun  3 12:08:33 abcd sudo", `{"timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"sudo"}`)
	f("Jun  3 12:08:33 abcd sudo[123]", `{"timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"sudo","proc_id":"123"}`)
	f("Jun  3 12:08:33 abcd sudo foobar", `{"timestamp":"2024-06-03T12:08:33.000Z","hostname":"abcd","app_name":"sudo","message":"foobar"}`)

	// Incomplete RFC 5424
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 [foo@123]`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47","foo@123":""}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345","msg_id":"ID47"}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname","proc_id":"12345"}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com","app_name":"appname"}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:32.123456789Z","hostname":"mymachine.example.com"}`)
	f(`<165>1 2023-06-03T17:42:32.123456789Z`,
		`{"priority":"165","facility":"20","severity":"5","timestamp":"2023-06-03T17:42:32.123456789Z"}`)
	f(`<165>1 `,
		`{"priority":"165","facility":"20","severity":"5"}`)
}
