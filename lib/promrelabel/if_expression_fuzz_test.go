package promrelabel

import (
	"testing"
)

func FuzzIfExpressionParse(f *testing.F) {
	f.Add("foo")
	f.Add(`{foo="bar"}`)
	f.Add(`foo{bar=~"baz", x!="y"}`)
	f.Add(`{a="b" or c="d",e="x"}`)
	f.Add(`foo{`)

	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			t.Skip()
		}

		var ie IfExpression
		_ = ie.Parse(s)
	})
}
