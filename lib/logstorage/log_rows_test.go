package logstorage

import (
	"reflect"
	"testing"
)

func TestLogRows_StreamFieldsOverride(t *testing.T) {
	f := func(rows []string, streamFields []Field, ignoreFields []string, resultExpected []string) {
		t.Helper()

		lr := GetLogRows(nil, ignoreFields, nil, "foobar")
		defer PutLogRows(lr)

		tid := TenantID{
			AccountID: 123,
			ProjectID: 456,
		}

		p := GetJSONParser()
		defer PutJSONParser(p)
		for i, r := range rows {
			if err := p.ParseLogMessage([]byte(r)); err != nil {
				t.Fatalf("unexpected error when parsing %q: %s", r, err)
			}
			timestamp := int64(i)*1_000 + 1
			lr.MustAdd(tid, timestamp, p.Fields, streamFields)
		}

		var result []string
		for i := range rows {
			s := lr.GetRowString(i)
			result = append(result, s)
		}
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	f([]string{
		`{"foo":"bar","_msg":"abc"}`,
		`{"xyz":"bar","_msg":"abc"}`,
		`{"xyz":"123","_msg":"abc"}`,
	}, []Field{
		{
			Name:  "xyz",
			Value: "123",
		},
	}, nil, []string{
		`{"_msg":"abc","_stream":"{xyz=\"123\"}","_time":"1970-01-01T00:00:00.000000001Z","foo":"bar"}`,
		`{"_msg":"abc","_stream":"{xyz=\"123\"}","_time":"1970-01-01T00:00:00.000001001Z","xyz":"bar"}`,
		`{"_msg":"abc","_stream":"{xyz=\"123\"}","_time":"1970-01-01T00:00:00.000002001Z","xyz":"123"}`,
	})
	f([]string{
		`{"foo":"bar","_msg":"abc"}`,
		`{"xyz":"bar","_msg":"abc"}`,
		`{"xyz":"123","_msg":"abc"}`,
	}, []Field{
		{
			Name:  "xyz",
			Value: "123",
		},
		{
			Name:  "f1",
			Value: "v1",
		},
	}, []string{"xyz", "qwert"}, []string{
		`{"_msg":"abc","_stream":"{f1=\"v1\"}","_time":"1970-01-01T00:00:00.000000001Z","foo":"bar"}`,
		`{"_msg":"abc","_stream":"{f1=\"v1\"}","_time":"1970-01-01T00:00:00.000001001Z"}`,
		`{"_msg":"abc","_stream":"{f1=\"v1\"}","_time":"1970-01-01T00:00:00.000002001Z"}`,
	})
}

func TestLogRows_DefaultMsgValue(t *testing.T) {
	type opts struct {
		rows []string

		streamFields    []string
		ignoreFields    []string
		extraFields     []Field
		defaultMsgValue string

		resultExpected []string
	}

	f := func(o opts) {
		t.Helper()

		lr := GetLogRows(o.streamFields, o.ignoreFields, o.extraFields, o.defaultMsgValue)
		defer PutLogRows(lr)

		tid := TenantID{
			AccountID: 123,
			ProjectID: 456,
		}

		p := GetJSONParser()
		defer PutJSONParser(p)
		for i, r := range o.rows {
			if err := p.ParseLogMessage([]byte(r)); err != nil {
				t.Fatalf("unexpected error when parsing %q: %s", r, err)
			}
			timestamp := int64(i)*1_000 + 1
			lr.MustAdd(tid, timestamp, p.Fields, nil)
		}

		var result []string
		for i := range o.rows {
			s := lr.GetRowString(i)
			result = append(result, s)
		}
		if !reflect.DeepEqual(result, o.resultExpected) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, o.resultExpected)
		}
	}

	var o opts

	f(o)

	// default options
	o = opts{
		rows: []string{
			`{"foo":"bar"}`,
			`{}`,
			`{"foo":"bar","a":"b"}`,
		},
		resultExpected: []string{
			`{"_stream":"{}","_time":"1970-01-01T00:00:00.000000001Z","foo":"bar"}`,
			`{"_stream":"{}","_time":"1970-01-01T00:00:00.000001001Z"}`,
			`{"_stream":"{}","_time":"1970-01-01T00:00:00.000002001Z","a":"b","foo":"bar"}`,
		},
	}
	f(o)

	// stream fields
	o = opts{
		rows: []string{
			`{"x":"y","foo":"bar"}`,
			`{"x":"y","foo":"bar","abc":"de"}`,
			`{}`,
		},
		streamFields: []string{"foo", "abc"},
		resultExpected: []string{
			`{"_stream":"{foo=\"bar\"}","_time":"1970-01-01T00:00:00.000000001Z","foo":"bar","x":"y"}`,
			`{"_stream":"{abc=\"de\",foo=\"bar\"}","_time":"1970-01-01T00:00:00.000001001Z","abc":"de","foo":"bar","x":"y"}`,
			`{"_stream":"{}","_time":"1970-01-01T00:00:00.000002001Z"}`,
		},
	}
	f(o)

	// ignore fields
	o = opts{
		rows: []string{
			`{"x":"y","foo":"bar"}`,
			`{"x":"y"}`,
			`{}`,
		},
		streamFields: []string{"foo", "abc", "x"},
		ignoreFields: []string{"foo"},
		resultExpected: []string{
			`{"_stream":"{x=\"y\"}","_time":"1970-01-01T00:00:00.000000001Z","x":"y"}`,
			`{"_stream":"{x=\"y\"}","_time":"1970-01-01T00:00:00.000001001Z","x":"y"}`,
			`{"_stream":"{}","_time":"1970-01-01T00:00:00.000002001Z"}`,
		},
	}
	f(o)

	// extra fields
	o = opts{
		rows: []string{
			`{"x":"y","foo":"bar"}`,
			`{}`,
		},
		streamFields: []string{"foo", "abc", "x"},
		ignoreFields: []string{"foo"},
		extraFields: []Field{
			{
				Name:  "foo",
				Value: "test",
			},
			{
				Name:  "abc",
				Value: "1234",
			},
		},
		resultExpected: []string{
			`{"_stream":"{abc=\"1234\",foo=\"test\",x=\"y\"}","_time":"1970-01-01T00:00:00.000000001Z","abc":"1234","foo":"test","x":"y"}`,
			`{"_stream":"{abc=\"1234\",foo=\"test\"}","_time":"1970-01-01T00:00:00.000001001Z","abc":"1234","foo":"test"}`,
		},
	}
	f(o)

	// default _msg value
	o = opts{
		rows: []string{
			`{"x":"y","foo":"bar"}`,
			`{"_msg":"ppp"}`,
			`{"abc":"ppp"}`,
		},
		streamFields:    []string{"abc", "x"},
		defaultMsgValue: "qwert",
		resultExpected: []string{
			`{"_msg":"qwert","_stream":"{x=\"y\"}","_time":"1970-01-01T00:00:00.000000001Z","foo":"bar","x":"y"}`,
			`{"_msg":"ppp","_stream":"{}","_time":"1970-01-01T00:00:00.000001001Z"}`,
			`{"_msg":"qwert","_stream":"{abc=\"ppp\"}","_time":"1970-01-01T00:00:00.000002001Z","abc":"ppp"}`,
		},
	}
	f(o)

}
