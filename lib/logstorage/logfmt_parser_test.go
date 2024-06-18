package logstorage

import (
	"testing"
)

func TestLogfmtParser(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()

		p := getLogfmtParser()
		defer putLogfmtParser(p)

		p.parse(s)
		result := MarshalFieldsToJSON(nil, p.fields)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result when parsing [%s]; got\n%s\nwant\n%s\n", s, result, resultExpected)
		}
	}

	f(``, `{}`)
	f(`foo=bar`, `{"foo":"bar"}`)
	f(`foo="bar=baz x=y"`, `{"foo":"bar=baz x=y"}`)
	f(`foo=`, `{"foo":""}`)
	f(`foo`, `{"foo":""}`)
	f(`foo bar`, `{"foo":"","bar":""}`)
	f(`foo bar=baz`, `{"foo":"","bar":"baz"}`)
	f(`foo=bar baz="x y" a=b`, `{"foo":"bar","baz":"x y","a":"b"}`)
	f(`  foo=bar  baz=x =z qwe`, `{"foo":"bar","baz":"x","_msg":"z","qwe":""}`)
}
