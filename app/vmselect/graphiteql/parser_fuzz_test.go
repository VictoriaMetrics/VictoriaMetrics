package graphiteql

import "testing"

func FuzzParse(f *testing.F) {
	f.Add("foo.bar")
	f.Add("sumSeries(foo.bar,baz.*)")
	f.Add(`foo|alias('bar')`)
	f.Add(`foo\,bar`)
	f.Add("sumSeries(foo")
	f.Add(`fOO.*.b[a-z]R{aa*,bb}s_s.$aaa`)
	f.Add(`f\x1B\x3a`)
	f.Add("True")
	f.Add("0xab")
	f.Add("-1.23e5")
	f.Add(`'fo\'o"b\\ar'`)
	f.Add("foo(bar,123,'baz')")
	f.Add(`f(a,xx=b|c|aa(124,'456'),aa=bb)`)
	f.Add("f(a=')")

	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			t.Skip()
		}

		expr, err := Parse(s)
		if err != nil {
			return
		}
		if expr == nil {
			t.Fatal("Parse returned nil expression without error")
		}
		_ = expr.AppendString(nil)
	})
}
