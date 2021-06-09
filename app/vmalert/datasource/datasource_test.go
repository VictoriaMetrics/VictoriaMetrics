package datasource

import "testing"

func TestMetric_Label(t *testing.T) {
	m := &Metric{}

	m.AddLabel("foo", "bar")
	checkEqualString(t, "bar", m.Label("foo"))

	m.SetLabel("foo", "baz")
	checkEqualString(t, "baz", m.Label("foo"))

	m.SetLabel("qux", "quux")
	checkEqualString(t, "quux", m.Label("qux"))

	checkEqualString(t, "", m.Label("non-existing"))
}
