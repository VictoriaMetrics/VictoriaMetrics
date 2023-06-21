package logjson

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func TestParserFailure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		p := GetParser()
		err := p.ParseLogMessage([]byte(data))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		PutParser(p)
	}
	f("")
	f("{foo")
	f("[1,2,3]")
	f(`{"foo",}`)
}

func TestParserSuccess(t *testing.T) {
	f := func(data string, fieldsExpected []logstorage.Field) {
		t.Helper()

		p := GetParser()
		err := p.ParseLogMessage([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(p.Fields, fieldsExpected) {
			t.Fatalf("unexpected fields;\ngot\n%s\nwant\n%s", p.Fields, fieldsExpected)
		}
		PutParser(p)
	}

	f("{}", nil)
	f(`{"foo":"bar"}`, []logstorage.Field{
		{
			Name:  "foo",
			Value: "bar",
		},
	})
	f(`{"foo":{"bar":"baz"},"a":1,"b":true,"c":[1,2],"d":false}`, []logstorage.Field{
		{
			Name:  "foo.bar",
			Value: "baz",
		},
		{
			Name:  "a",
			Value: "1",
		},
		{
			Name:  "b",
			Value: "true",
		},
		{
			Name:  "c",
			Value: "[1,2]",
		},
		{
			Name:  "d",
			Value: "false",
		},
	})
}
