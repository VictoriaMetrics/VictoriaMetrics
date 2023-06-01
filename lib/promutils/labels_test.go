package promutils

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestLabels(t *testing.T) {
	x := NewLabels(2)
	x.Add("job", "bar")
	x.Add("instance", "foo")
	v := x.Get("instance")
	if v != "foo" {
		t.Fatalf("unexpected value obtained; got %q; want %q", v, "foo")
	}
	v = x.Get("non-existing-label")
	if v != "" {
		t.Fatalf("unexpected non-empty value obtained for non-existing label: %q", v)
	}
	n := x.Len()
	if n != 2 {
		t.Fatalf("unexpected labels len; got %d; want 2", n)
	}
	x.Sort()
	x.SortStable()
	s, err := yaml.Marshal(x)
	if err != nil {
		t.Fatalf("unexpected error in yaml.Marshal: %s", err)
	}
	sExpected := "instance: foo\njob: bar\n"
	if string(s) != sExpected {
		t.Fatalf("unexpected marshaled value;\ngot\n%s\nwant\n%q", s, sExpected)
	}
	x1 := GetLabels()
	if err := yaml.Unmarshal(s, &x1); err != nil {
		t.Fatalf("unexpected error in yaml.Unmarshal: %s", err)
	}
	x1.InternStrings()
	s, err = yaml.Marshal(&x1)
	if err != nil {
		t.Fatalf("unexpected error in yaml.Marshal: %s", err)
	}
	if string(s) != sExpected {
		t.Fatalf("unexpected marshaled value;\ngot\n%s\nwant\n%q", s, sExpected)
	}
	PutLabels(x1)
	x1 = nil
	if n = x1.Len(); n != 0 {
		t.Fatalf("unexpected len for empty labels: %d", n)
	}
	x1 = GetLabels()
	x2 := x.Clone()
	s, err = yaml.Marshal(x2)
	if err != nil {
		t.Fatalf("cannot marshal cloned labels")
	}
	if string(s) != sExpected {
		t.Fatalf("unexpected marshaled value;\ngot\n%s\nwant\n%q", s, sExpected)
	}
	s2 := x2.String()
	s2Expected := `{instance="foo",job="bar"}`
	if s2 != s2Expected {
		t.Fatalf("unexpected string representation for labels;\ngot\n%s\nwant\n%s", s2, s2Expected)
	}
}

func TestLabelsUnmarshalYAMLFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var x Labels
		if err := yaml.Unmarshal([]byte(s), &x); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}
	f("foobar")
	f("[foo,bar]")
	f("{foo:[bar]}")
	f("[aa")
}

func TestLabelsUnmarshalJSONSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var x Labels
		if err := json.Unmarshal([]byte(s), &x); err != nil {
			t.Fatalf("unexpected error in json.Unmarshal: %s", err)
		}
		data, err := json.Marshal(&x)
		if err != nil {
			t.Fatalf("json.Marshal error: %s", err)
		}
		if string(data) != s {
			t.Fatalf("unexpected marshaled JSON;\ngot\n%s\nwant\n%s", data, s)
		}
	}
	f(`{}`)
	f(`{"foo":"bar"}`)
	f(`{"a":"y","x":"b"}`)
}

func TestLabelsUnmarshalJSONFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var x Labels
		if err := json.Unmarshal([]byte(s), &x); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}
	f("foobar")
	f("[1,2]")
	f(`{"foo":123}`)
	f(`{"foo"`)
	f(`"ff`)
}

func TestLabelsRemoveDuplicates(t *testing.T) {
	var x Labels
	x.Add("foo", "bar")
	x.Add("foo", "baz")
	x.Add("foo", "123")
	x.Add("bar", "a")
	x.RemoveDuplicates()
	s := x.String()
	sExpected := `{bar="a",foo="123"}`
	if s != sExpected {
		t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", s, sExpected)
	}
}

func TestLabelsAddFrom(t *testing.T) {
	var a, b Labels
	a.Add("z", "x")
	a.Add("foo", "bar")
	b.Add("foo", "baz")
	b.Add("x", "y")
	a.AddFrom(&b)
	a.RemoveDuplicates()
	s := a.String()
	sExpected := `{foo="baz",x="y",z="x"}`
	if s != sExpected {
		t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", s, sExpected)
	}
}

func TestLabelsRemoveMetaLabels(t *testing.T) {
	f := func(metric, resultExpected string) {
		t.Helper()
		labels := MustNewLabelsFromString(metric)
		labels.RemoveMetaLabels()
		result := labels.String()
		if result != resultExpected {
			t.Fatalf("unexpected result of RemoveMetaLabels;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}
	f(`{}`, `{}`)
	f(`{foo="bar"}`, `{foo="bar"}`)
	f(`{__meta_foo="bar"}`, `{}`)
	f(`{__meta_foo="bdffr",foo="bar",__meta_xxx="basd"}`, `{foo="bar"}`)
}

func TestLabelsRemoveLabelsWithDoubleUnderscorePrefix(t *testing.T) {
	f := func(metric, resultExpected string) {
		t.Helper()
		labels := MustNewLabelsFromString(metric)
		labels.RemoveLabelsWithDoubleUnderscorePrefix()
		result := labels.String()
		if result != resultExpected {
			t.Fatalf("unexpected result of RemoveLabelsWithDoubleUnderscorePrefix;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}
	f(`{}`, `{}`)
	f(`{foo="bar"}`, `{foo="bar"}`)
	f(`{__meta_foo="bar",a="b",__name__="foo",__vm_filepath="aa"}`, `{a="b"}`)
	f(`{__meta_foo="bdffr",foo="bar",__meta_xxx="basd"}`, `{foo="bar"}`)
}

func TestLabels_Set(t *testing.T) {
	f := func(metric, name, value, resultExpected string) {
		t.Helper()
		labels := MustNewLabelsFromString(metric)
		labels.Set(name, value)
		result := labels.String()
		if result != resultExpected {
			t.Fatalf("unexpected result of RemoveLabelsWithDoubleUnderscorePrefix;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}
	f(`{}`, ``, ``, `{}`)
	f(`{foo="bar"}`, `bar`, `baz`, `{foo="bar",bar="baz"}`)
	f(`{__meta_foo="bar",a="b",__name__="foo",__vm_filepath="aa"}`, `__name__`, `bar`, `{__meta_foo="bar",a="b",__name__="bar",__vm_filepath="aa"}`)
	f(`{__meta_foo="bdffr",foo="bar",__meta_xxx="basd"}`, `__name__`, `baz`, `{__meta_foo="bdffr",foo="bar",__meta_xxx="basd",__name__="baz"}`)
	f(`http_request_total{a="b"}`, `__name__`, `metric`, `{__name__="metric",a="b"}`)
	f(`http_request_total{a="b"}`, `a`, `c`, `{__name__="http_request_total",a="c"}`)
	f(`http_request_total{a="b"}`, `ip`, `127.0.0.1`, `{__name__="http_request_total",a="b",ip="127.0.0.1"}`)
}

func TestNewLabelsFromStringSuccess(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		labels, err := NewLabelsFromString(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := labels.String()
		if result != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	f("{}", "{}")
	f("foo", `{__name__="foo"}`)
	f(`foo{bar="baz"}`, `{__name__="foo",bar="baz"}`)
	f(`foo {bar="baz", a="b"}`, `{__name__="foo",bar="baz",a="b"}`)
	f(`{foo="bar", baz="a"}`, `{foo="bar",baz="a"}`)
	f(`{__name__="aaa"}`, `{__name__="aaa"}`)
	f(`{__name__="abc",de="fg"}`, `{__name__="abc",de="fg"}`)
}

func TestNewLabelsFromStringFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		labels, err := NewLabelsFromString(s)
		if labels != nil {
			t.Fatalf("unexpected non-nil labels: %s", labels)
		}
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	f("")
	f("foo bar")
	f(`foo{`)
	f(`foo{bar`)
	f(`foo{bar=`)
	f(`foo{bar="`)
	f(`foo{bar="baz`)
	f(`foo{bar="baz"`)
	f(`foo{bar="baz",`)
	f(`foo{"bar"="baz"}`)
	f(`{"bar":"baz"}`)
	f(`{bar:"baz"}`)
	f(`{bar=~"baz"}`)
}
