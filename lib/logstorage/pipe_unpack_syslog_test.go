package logstorage

import (
	"testing"
)

func TestParsePipeUnpackSyslogSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`unpack_syslog`)
	f(`unpack_syslog offset 6h30m`)
	f(`unpack_syslog offset -6h30m`)
	f(`unpack_syslog keep_original_fields`)
	f(`unpack_syslog offset -6h30m keep_original_fields`)
	f(`unpack_syslog if (a:x)`)
	f(`unpack_syslog if (a:x) keep_original_fields`)
	f(`unpack_syslog if (a:x) offset 2h keep_original_fields`)
	f(`unpack_syslog from x`)
	f(`unpack_syslog from x keep_original_fields`)
	f(`unpack_syslog if (a:x) from x`)
	f(`unpack_syslog from x result_prefix abc`)
	f(`unpack_syslog from x offset 2h30m result_prefix abc`)
	f(`unpack_syslog if (a:x) from x result_prefix abc`)
	f(`unpack_syslog result_prefix abc`)
	f(`unpack_syslog if (a:x) result_prefix abc`)
	f(`unpack_syslog if (a:x) offset -1h result_prefix abc`)
}

func TestParsePipeUnpackSyslogFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`unpack_syslog foo`)
	f(`unpack_syslog if`)
	f(`unpack_syslog offset`)
	f(`unpack_syslog if (x:y) foobar`)
	f(`unpack_syslog from`)
	f(`unpack_syslog from x y`)
	f(`unpack_syslog from x if`)
	f(`unpack_syslog from x result_prefix`)
	f(`unpack_syslog from x result_prefix a b`)
	f(`unpack_syslog from x result_prefix a if`)
	f(`unpack_syslog result_prefix`)
	f(`unpack_syslog result_prefix a b`)
	f(`unpack_syslog result_prefix a if`)
}

func TestPipeUnpackSyslog(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// no skip empty results
	f("unpack_syslog", [][]Field{
		{
			{"_msg", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
			{"foo", "321"},
		},
	}, [][]Field{
		{
			{"_msg", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
			{"foo", "321"},
			{"priority", "165"},
			{"facility", "20"},
			{"severity", "5"},
			{"format", "rfc5424"},
			{"timestamp", "2023-06-03T17:42:32.123456789Z"},
			{"hostname", "mymachine.example.com"},
			{"app_name", "appname"},
			{"proc_id", "12345"},
			{"msg_id", "ID47"},
			{"message", "This is a test message with structured data"},
		},
	})

	// keep original fields
	f("unpack_syslog keep_original_fields", [][]Field{
		{
			{"_msg", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
			{"foo", "321"},
			{"app_name", "foobar"},
			{"msg_id", "baz"},
		},
	}, [][]Field{
		{
			{"_msg", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
			{"foo", "321"},
			{"priority", "165"},
			{"facility", "20"},
			{"severity", "5"},
			{"format", "rfc5424"},
			{"timestamp", "2023-06-03T17:42:32.123456789Z"},
			{"hostname", "mymachine.example.com"},
			{"app_name", "foobar"},
			{"proc_id", "12345"},
			{"msg_id", "baz"},
			{"message", "This is a test message with structured data"},
		},
	})

	// unpack from other field
	f("unpack_syslog from x", [][]Field{
		{
			{"x", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
		},
	}, [][]Field{
		{
			{"x", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
			{"priority", "165"},
			{"facility", "20"},
			{"severity", "5"},
			{"format", "rfc5424"},
			{"timestamp", "2023-06-03T17:42:32.123456789Z"},
			{"hostname", "mymachine.example.com"},
			{"app_name", "appname"},
			{"proc_id", "12345"},
			{"msg_id", "ID47"},
			{"message", "This is a test message with structured data"},
		},
	})

	// offset should be ignored when parsing non-rfc3164 messages
	f("unpack_syslog from x offset 2h30m", [][]Field{
		{
			{"x", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
		},
	}, [][]Field{
		{
			{"x", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
			{"priority", "165"},
			{"facility", "20"},
			{"severity", "5"},
			{"format", "rfc5424"},
			{"timestamp", "2023-06-03T17:42:32.123456789Z"},
			{"hostname", "mymachine.example.com"},
			{"app_name", "appname"},
			{"proc_id", "12345"},
			{"msg_id", "ID47"},
			{"message", "This is a test message with structured data"},
		},
	})

	// failed if condition
	f("unpack_syslog if (foo:bar)", [][]Field{
		{
			{"_msg", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
		},
	}, [][]Field{
		{
			{"_msg", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
		},
	})

	// matched if condition
	f("unpack_syslog if (appname)", [][]Field{
		{
			{"_msg", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
		},
	}, [][]Field{
		{
			{"_msg", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
			{"priority", "165"},
			{"facility", "20"},
			{"severity", "5"},
			{"format", "rfc5424"},
			{"timestamp", "2023-06-03T17:42:32.123456789Z"},
			{"hostname", "mymachine.example.com"},
			{"app_name", "appname"},
			{"proc_id", "12345"},
			{"msg_id", "ID47"},
			{"message", "This is a test message with structured data"},
		},
	})

	// single row, unpack from missing field
	f("unpack_syslog from x", [][]Field{
		{
			{"_msg", `foo=bar`},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar`},
		},
	})

	// single row, unpack from non-syslog field
	f("unpack_syslog from x", [][]Field{
		{
			{"x", `foobar`},
		},
	}, [][]Field{
		{
			{"x", `foobar`},
			{"format", "rfc3164"},
			{"message", "foobar"},
		},
	})

	// multiple rows with distinct number of fields
	f("unpack_syslog from x result_prefix qwe_", [][]Field{
		{
			{"x", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
		},
		{
			{"x", `<163>1 2024-12-13T18:21:43Z mymachine.example.com appname2 345 ID7 - foobar`},
			{"y", `z=bar`},
		},
	}, [][]Field{
		{
			{"x", `<165>1 2023-06-03T17:42:32.123456789Z mymachine.example.com appname 12345 ID47 - This is a test message with structured data`},
			{"qwe_priority", "165"},
			{"qwe_facility", "20"},
			{"qwe_severity", "5"},
			{"qwe_format", "rfc5424"},
			{"qwe_timestamp", "2023-06-03T17:42:32.123456789Z"},
			{"qwe_hostname", "mymachine.example.com"},
			{"qwe_app_name", "appname"},
			{"qwe_proc_id", "12345"},
			{"qwe_msg_id", "ID47"},
			{"qwe_message", "This is a test message with structured data"},
		},
		{
			{"x", `<163>1 2024-12-13T18:21:43Z mymachine.example.com appname2 345 ID7 - foobar`},
			{"y", `z=bar`},
			{"qwe_priority", "163"},
			{"qwe_facility", "20"},
			{"qwe_severity", "3"},
			{"qwe_format", "rfc5424"},
			{"qwe_timestamp", "2024-12-13T18:21:43Z"},
			{"qwe_hostname", "mymachine.example.com"},
			{"qwe_app_name", "appname2"},
			{"qwe_proc_id", "345"},
			{"qwe_msg_id", "ID7"},
			{"qwe_message", "foobar"},
		},
	})
}

func TestPipeUnpackSyslogUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unpack_syslog", "*", "", "*", "")
	f("unpack_syslog keep_original_fields", "*", "", "*", "")
	f("unpack_syslog if (y:z) from x", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("unpack_syslog from x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_syslog if (y:z) from x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_syslog if (f1:z) from x", "*", "f1,f2", "*", "f2")

	// all the needed fields, unneeded fields intersect with src
	f("unpack_syslog from x", "*", "f2,x", "*", "f2")
	f("unpack_syslog if (y:z) from x", "*", "f2,x", "*", "f2")
	f("unpack_syslog if (f2:z) from x", "*", "f1,f2,x", "*", "f1")

	// needed fields do not intersect with src
	f("unpack_syslog from x", "f1,f2", "", "f1,f2,x", "")
	f("unpack_syslog if (y:z) from x", "f1,f2", "", "f1,f2,x,y", "")
	f("unpack_syslog if (f1:z) from x", "f1,f2", "", "f1,f2,x", "")

	// needed fields intersect with src
	f("unpack_syslog from x", "f2,x", "", "f2,x", "")
	f("unpack_syslog if (y:z) from x", "f2,x", "", "f2,x,y", "")
	f("unpack_syslog if (f2:z y:qwe) from x", "f2,x", "", "f2,x,y", "")
}
