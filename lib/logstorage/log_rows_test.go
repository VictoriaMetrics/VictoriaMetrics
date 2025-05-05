package logstorage

import (
	"reflect"
	"testing"
)

func TestLogRows_WildcardIgnoreFields(t *testing.T) {
	type opts struct {
		rows []string

		streamFields []string
		ignoreFields []string
		extraFields  []Field

		resultExpected []string
	}

	f := func(o opts) {
		t.Helper()

		lr := GetLogRows(o.streamFields, o.ignoreFields, o.extraFields, "foobar")
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

	o := opts{
		rows: []string{
			`{"foo.a":"bar","foo.b":"abc","z":"abc","x":"y","_msg":"aaa","foobar":"b"}`,
			`{"_msg":"x"}`,
		},
		streamFields: []string{"foo.a", "foo.b", "foobar"},
		ignoreFields: []string{"foo.*", "x"},
		extraFields: []Field{
			{
				Name:  "foo.a",
				Value: "1234",
			},
		},
		resultExpected: []string{
			`{"_msg":"aaa","_stream":"{foo.a=\"1234\",foobar=\"b\"}","_time":"1970-01-01T00:00:00.000000001Z","foo.a":"1234","foobar":"b","z":"abc"}`,
			`{"_msg":"x","_stream":"{foo.a=\"1234\"}","_time":"1970-01-01T00:00:00.000001001Z","foo.a":"1234"}`,
		},
	}
	f(o)
}

func TestLogRows_StreamFieldsOverride(t *testing.T) {
	type opts struct {
		rows []string

		streamFields []Field
		ignoreFields []string

		resultExpected []string
	}

	f := func(o opts) {
		t.Helper()

		lr := GetLogRows(nil, o.ignoreFields, nil, "foobar")
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
			lr.MustAdd(tid, timestamp, p.Fields, o.streamFields)
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

	o = opts{
		rows: []string{
			`{"foo":"bar","_msg":"abc"}`,
			`{"xyz":"bar","_msg":"abc"}`,
			`{"xyz":"123","_msg":"abc"}`,
		},
		streamFields: []Field{
			{
				Name:  "xyz",
				Value: "123",
			},
		},
		resultExpected: []string{
			`{"_msg":"abc","_stream":"{xyz=\"123\"}","_time":"1970-01-01T00:00:00.000000001Z","foo":"bar"}`,
			`{"_msg":"abc","_stream":"{xyz=\"123\"}","_time":"1970-01-01T00:00:00.000001001Z","xyz":"bar"}`,
			`{"_msg":"abc","_stream":"{xyz=\"123\"}","_time":"1970-01-01T00:00:00.000002001Z","xyz":"123"}`,
		},
	}
	f(o)

	o = opts{
		rows: []string{
			`{"foo":"bar","_msg":"abc"}`,
			`{"xyz":"bar","_msg":"abc"}`,
			`{"xyz":"123","_msg":"abc"}`,
		},
		streamFields: []Field{
			{
				Name:  "xyz",
				Value: "123",
			},
			{
				Name:  "f1",
				Value: "v1",
			},
		},
		ignoreFields: []string{"xyz", "qwert"},
		resultExpected: []string{
			`{"_msg":"abc","_stream":"{f1=\"v1\"}","_time":"1970-01-01T00:00:00.000000001Z","foo":"bar"}`,
			`{"_msg":"abc","_stream":"{f1=\"v1\"}","_time":"1970-01-01T00:00:00.000001001Z"}`,
			`{"_msg":"abc","_stream":"{f1=\"v1\"}","_time":"1970-01-01T00:00:00.000002001Z"}`,
		},
	}
	f(o)
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

func TestInsertRow_MarshalUnmarshal(t *testing.T) {
	r := &InsertRow{
		TenantID: TenantID{
			AccountID: 123,
			ProjectID: 456,
		},
		StreamTagsCanonical: "foobar",
		Timestamp:           789,
		Fields: []Field{
			{
				Name:  "x",
				Value: "y",
			},
			{
				Name:  "qwe",
				Value: "rty",
			},
		},
	}
	data := r.Marshal(nil)

	var r2 InsertRow
	tail, err := r2.UnmarshalInplace(data)
	if err != nil {
		t.Fatalf("unexpected error when unmarshaling InsertRow: %s", err)
	}
	if len(tail) > 0 {
		t.Fatalf("unexpected tail left after unmarshaling InsertRow; len(tail)=%d; tail=%X", len(tail), tail)
	}
}
