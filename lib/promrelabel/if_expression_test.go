package promrelabel

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"gopkg.in/yaml.v2"
)

func TestIfExpressionParseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var ie IfExpression
		if err := ie.Parse(s); err == nil {
			t.Fatalf("expecting non-nil error when parsing %q", s)
		}
	}
	f(`{`)
	f(`{foo`)
	f(`foo{`)
	f(`foo{bar="a" or}`)
}

func TestIfExpressionParseSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var ie IfExpression
		if err := ie.Parse(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}
	f(`foo`)
	f(`{foo="bar"}`)
	f(`foo{bar=~"baz", x!="y"}`)
	f(`{a="b" or c="d",e="x"}`)
	f(`foo{
  bar="a",x="y" or
  x="a",a="b" or
  a="x"
}`)
}

func TestIfExpressionMarshalUnmarshalJSON(t *testing.T) {
	f := func(s, jsonExpected string) {
		t.Helper()
		var ie IfExpression
		if err := ie.Parse(s); err != nil {
			t.Fatalf("cannot parse ifExpression %q: %s", s, err)
		}
		data, err := json.Marshal(&ie)
		if err != nil {
			t.Fatalf("cannot marshal ifExpression %q: %s", s, err)
		}
		if string(data) != jsonExpected {
			t.Fatalf("unexpected value after json marshaling;\ngot\n%s\nwant\n%s", data, jsonExpected)
		}
		var ie2 IfExpression
		if err := json.Unmarshal(data, &ie2); err != nil {
			t.Fatalf("cannot unmarshal ifExpression from json %q: %s", data, err)
		}
		data2, err := json.Marshal(&ie2)
		if err != nil {
			t.Fatalf("cannot marshal ifExpression2: %s", err)
		}
		if string(data2) != jsonExpected {
			t.Fatalf("unexpected data after unmarshal/marshal cycle;\ngot\n%s\nwant\n%s", data2, jsonExpected)
		}
	}
	f("foo", `"foo"`)
	f(`{foo="bar",baz=~"x.*"}`, `"{foo=\"bar\",baz=~\"x.*\"}"`)
	f(`{a="b" or c="d",x="z"}`, `"{a=\"b\" or c=\"d\",x=\"z\"}"`)
}

func TestIfExpressionUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var ie IfExpression
		err := yaml.UnmarshalStrict([]byte(s), &ie)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}
	f(`{`)
	f(`{x:y}`)
	f(`[1]`)
	f(`"{"`)
	f(`'{'`)
	f(`foo{bar`)
	f(`foo{bar}`)
	f(`foo{bar=`)
	f(`foo{bar="`)
	f(`foo{bar='`)
	f(`foo{bar=~"("}`)
	f(`foo{bar!~"("}`)
	f(`foo{bar==aaa}`)
	f(`foo{bar=="b"}`)
	f(`'foo+bar'`)
	f(`'foo{bar=~"a[b"}'`)
}

func TestIfExpressionUnmarshalSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var ie IfExpression
		if err := yaml.UnmarshalStrict([]byte(s), &ie); err != nil {
			t.Fatalf("unexpected error during unmarshal: %s", err)
		}
		b, err := yaml.Marshal(&ie)
		if err != nil {
			t.Fatalf("unexpected error during marshal: %s", err)
		}
		b = bytes.TrimSpace(b)
		if string(b) != s {
			t.Fatalf("unexpected marshaled data;\ngot\n%s\nwant\n%s", b, s)
		}
	}
	f(`'{}'`)
	f(`foo`)
	f(`foo{bar="baz"}`)
	f(`'{a="b", c!="d", e=~"g", h!~"d"}'`)
	f(`foo{bar="zs",a=~"b|c"}`)
	f(`foo{z="y" or bar="zs",a=~"b|c"}`)
	f(`- foo
- bar{baz="abc"}`)
}

func TestIfExpressionMatch(t *testing.T) {
	f := func(ifExpr, metricWithLabels string) {
		t.Helper()
		var ie IfExpression
		if err := yaml.UnmarshalStrict([]byte(ifExpr), &ie); err != nil {
			t.Fatalf("unexpected error during unmarshal: %s", err)
		}
		labels := promutils.MustNewLabelsFromString(metricWithLabels)
		if !ie.Match(labels.GetLabels()) {
			t.Fatalf("unexpected mismatch of ifExpr=%s for %s", ifExpr, metricWithLabels)
		}
	}
	f(`foo`, `foo`)
	f(`foo`, `foo{bar="baz",a="b"}`)
	f(`foo{bar="a"}`, `foo{bar="a"}`)
	f(`foo{bar="a" or baz="x"}`, `foo{bar="a"}`)
	f(`foo{baz="x" or bar="a"}`, `foo{bar="a"}`)
	f(`foo{bar="a"}`, `foo{x="y",bar="a",baz="b"}`)
	f(`'{a=~"x|abc",y!="z"}'`, `m{x="aa",a="abc"}`)
	f(`'{a=~"x|abc",y!="z"}'`, `m{x="aa",a="abc",y="qwe"}`)
	f(`'{__name__="foo"}'`, `foo{bar="baz"}`)
	f(`'{__name__=~"foo|bar"}'`, `bar`)
	f(`'{__name__!=""}'`, `foo`)
	f(`'{__name__!=""}'`, `bar{baz="aa",b="c"}`)
	f(`'{__name__!~"a.+"}'`, `bar{baz="aa",b="c"}`)
	f(`foo{a!~"a.+"}`, `foo{a="baa"}`)
	f(`'{foo=""}'`, `bar`)
	f(`'{foo!=""}'`, `aa{foo="b"}`)
	f(`'{foo=~".*"}'`, `abc`)
	f(`'{foo=~".*"}'`, `abc{foo="bar"}`)
	f(`'{foo!~".+"}'`, `abc`)
	f(`'{foo=~"bar|"}'`, `abc`)
	f(`'{foo=~"bar|"}'`, `abc{foo="bar"}`)
	f(`'{foo!~"bar|"}'`, `abc{foo="baz"}`)
}

func TestIfExpressionMismatch(t *testing.T) {
	f := func(ifExpr, metricWithLabels string) {
		t.Helper()
		var ie IfExpression
		if err := yaml.UnmarshalStrict([]byte(ifExpr), &ie); err != nil {
			t.Fatalf("unexpected error during unmarshal: %s", err)
		}
		labels := promutils.MustNewLabelsFromString(metricWithLabels)
		if ie.Match(labels.GetLabels()) {
			t.Fatalf("unexpected match of ifExpr=%s for %s", ifExpr, metricWithLabels)
		}
	}
	f(`foo`, `bar`)
	f(`foo`, `a{foo="bar"}`)
	f(`foo{bar="a"}`, `foo`)
	f(`foo{bar="a" or baz="a"}`, `foo`)
	f(`foo{bar="a"}`, `foo{bar="b"}`)
	f(`foo{bar="a"}`, `foo{baz="b",a="b"}`)
	f(`'{a=~"x|abc",y!="z"}'`, `m{x="aa",a="xabc"}`)
	f(`'{a=~"x|abc",y!="z"}'`, `m{x="aa",a="abc",y="z"}`)
	f(`'{__name__!~".+"}'`, `foo`)
	f(`'{a!~"a.+"}'`, `foo{a="abc"}`)
	f(`'{foo=""}'`, `bar{foo="aa"}`)
	f(`'{foo!=""}'`, `aa`)
	f(`'{foo=~".+"}'`, `abc`)
	f(`'{foo!~".+"}'`, `abc{foo="x"}`)
	f(`'{foo=~"bar|"}'`, `abc{foo="baz"}`)
	f(`'{foo!~"bar|"}'`, `abc`)
	f(`'{foo!~"bar|"}'`, `abc{foo="bar"}`)
}
