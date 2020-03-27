package csvimport

import (
	"testing"
)

func TestScannerSuccess(t *testing.T) {
	var sc scanner
	sc.Init("foo,bar\n\"aa,\"\"bb\",\"\"")
	if !sc.NextLine() {
		t.Fatalf("expecting the first line")
	}
	if sc.Line != "foo,bar" {
		t.Fatalf("unexpected line; got %q; want %q", sc.Line, "foo,bar")
	}
	if !sc.NextColumn() {
		t.Fatalf("expecting the first column")
	}
	if sc.Column != "foo" {
		t.Fatalf("unexpected first column; got %q; want %q", sc.Column, "foo")
	}
	if !sc.NextColumn() {
		t.Fatalf("expecting the second column")
	}
	if sc.Column != "bar" {
		t.Fatalf("unexpected second column; got %q; want %q", sc.Column, "bar")
	}
	if sc.NextColumn() {
		t.Fatalf("unexpected next column: %q", sc.Column)
	}
	if sc.Error != nil {
		t.Fatalf("unexpected error: %s", sc.Error)
	}
	if !sc.NextLine() {
		t.Fatalf("expecting the second line")
	}
	if sc.Line != "\"aa,\"\"bb\",\"\"" {
		t.Fatalf("unexpected the second line; got %q; want %q", sc.Line, "\"aa,\"\"bb\",\"\"")
	}
	if !sc.NextColumn() {
		t.Fatalf("expecting the first column on the second line")
	}
	if sc.Column != "aa,\"bb" {
		t.Fatalf("unexpected column on the second line; got %q; want %q", sc.Column, "aa,\"bb")
	}
	if !sc.NextColumn() {
		t.Fatalf("expecting the second column on the second line")
	}
	if sc.Column != "" {
		t.Fatalf("unexpected column on the second line; got %q; want %q", sc.Column, "")
	}
	if sc.NextColumn() {
		t.Fatalf("unexpected next column on the second line: %q", sc.Column)
	}
	if sc.Error != nil {
		t.Fatalf("unexpected error: %s", sc.Error)
	}
	if sc.NextLine() {
		t.Fatalf("unexpected next line: %q", sc.Line)
	}
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
