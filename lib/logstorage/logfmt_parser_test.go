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
		result := marshalFieldsToJSON(nil, p.fields)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result when parsing [%s]; got\n%s\nwant\n%s\n", s, result, resultExpected)
		}
	}

	f(``, `{}`)
	f(`foo=bar`, `{"foo":"bar"}`)
	f(`foo="bar=baz x=y"`, `{"foo":"bar=baz x=y"}`)
	f(`foo=`, `{"foo":""}`)
	f(`foo=bar baz="x y" a=b`, `{"foo":"bar","baz":"x y","a":"b"}`)

	// errors
	f(`foo`, `{}`)
	f(`foo=bar  baz=x z qwe`, `{"foo":"bar","baz":"x"}`)
}
