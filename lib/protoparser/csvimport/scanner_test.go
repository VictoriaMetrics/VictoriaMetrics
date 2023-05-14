package csvimport

import (
	"reflect"
	"testing"
)

func TestReadQuotedFieldSuccess(t *testing.T) {
	f := func(s, resultExpected, tailExpected string) {
		t.Helper()
		result, tail, err := readQuotedField(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
		if tail != tailExpected {
			t.Fatalf("unexpected tail; got %q; want %q", tail, tailExpected)
		}
	}

	// double quotes
	f(`""`, ``, ``)
	f(`"",`, ``, `,`)
	f(`"",foobar`, ``, `,foobar`)
	f(`"","bc"`, ``, `,"bc"`)
	f(`"a"`, `a`, ``)
	f(`"a"bc`, `a`, `bc`)
	f(`"foo`+"`',\n\t\r"+`bar"baz`, "foo`',\n\t\rbar", "baz")

	// single quotes
	f(`''`, ``, ``)
	f(`'',`, ``, `,`)
	f(`'',foobar`, ``, `,foobar`)
	f(`'','bc'`, ``, `,'bc'`)
	f(`'a'`, `a`, ``)
	f(`'a'bc`, `a`, `bc`)
	f(`'foo"`+"`,\n\t\r"+`bar'baz`, "foo\"`,\n\t\rbar", "baz")

	// escaped double quotes
	f(`" foo""bar"baz`, ` foo"bar`, `baz`)
	f(`""""bar"baz`, `"`, `bar"baz`)
	f(`"a,""b""'c",d,"e"`, `a,"b"'c`, `,d,"e"`)

	// escaped single quotes
	f(`' foo''bar'baz`, ` foo'bar`, `baz`)
	f(`''''bar'baz`, `'`, `bar'baz`)
	f(`'''bar'''baz`, `'bar'`, `baz`)
	f(`'a,''b''"c',d,'e'`, `a,'b'"c`, `,d,'e'`)
}

func TestReadQuotedFieldFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		field, tail, err := readQuotedField(s)
		if field != "" {
			t.Fatalf("unexpected non-empty field returned: %q", field)
		}
		if tail != s {
			t.Fatalf("unexpected tail returned; got %q; want %q", tail, s)
		}
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}
	f(`"`)
	f(`'`)
	f(`"foo""`)
	f(`'foo''`)
	f(`'foo''`)
}

func TestScannerSuccess(t *testing.T) {
	f := func(s string, rowsExpected [][]string) {
		t.Helper()
		var sc scanner
		sc.Init(s)
		var rows [][]string
		for sc.NextLine() {
			var row []string
			for sc.NextColumn() {
				row = append(row, sc.Column)
			}
			rows = append(rows, row)
		}
		if sc.Error != nil {
			t.Fatalf("unexpected error: %s", sc.Error)
		}
		if !reflect.DeepEqual(rows, rowsExpected) {
			t.Fatalf("unexpected rows;\ngot\n%q\nwant\n%q", rows, rowsExpected)
		}
	}

	f("", nil)
	f("\n", nil)
	f("\r\n\n\r", nil)
	f("foo,bar\n\"aa,\"\"bb\",\"\"", [][]string{
		{"foo", "bar"},
		{`aa,"bb`, ``},
	})
	f(`fo"bar,baz'a,"bc""de",'g''e'`, [][]string{
		{`fo"bar`, `baz'a`, `bc"de`, `g'e`},
	})
	f(`,`, [][]string{
		{``, ``},
	})
	f(`foo`, [][]string{
		{`foo`},
	})
	f(`foo,,`+"\r\n"+`,bar,`+"\n", [][]string{
		{`foo`, ``, ``},
		{``, `bar`, ``},
	})
}

func TestScannerFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var sc scanner
		sc.Init(s)
		for sc.NextLine() {
			for sc.NextColumn() {
			}
			if sc.Error != nil {
				if sc.NextColumn() {
					t.Fatalf("unexpected NextColumn success after the error %v", sc.Error)
				}
				return
			}
		}
		t.Fatalf("expecting at least a single error")
	}
	// Unclosed quote
	f("foo\r\n\"bar,")
	f(`"foo,"bar`)
	f(`foo,"bar",""a`)
	f(`foo,"bar","a""`)
}
