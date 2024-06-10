package datadogutils

import (
	"testing"
)

func TestSplitTag(t *testing.T) {
	f := func(s, nameExpected, valueExpected string) {
		t.Helper()
		name, value := SplitTag(s)
		if name != nameExpected {
			t.Fatalf("unexpected name obtained from %q; got %q; want %q", s, name, nameExpected)
		}
		if value != valueExpected {
			t.Fatalf("unexpected value obtained from %q; got %q; want %q", s, value, valueExpected)
		}
	}
	f("", "", "no_label_value")
	f("foo", "foo", "no_label_value")
	f("foo:bar", "foo", "bar")
	f(":bar", "", "bar")
}

func TestSanitizeName(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result := SanitizeName(s)
		if result != resultExpected {
			t.Fatalf("unexpected result for sanitizeName(%q); got\n%q\nwant\n%q", s, result, resultExpected)
		}
	}
	f("before.dot.metric!.name", "before.dot.metric.name")
	f("after.dot.metric.!name", "after.dot.metric.name")
	f("in.the.middle.met!ric.name", "in.the.middle.met_ric.name")
	f("before.and.after.and.middle.met!ric!.!name", "before.and.after.and.middle.met_ric.name")
	f("many.consecutive.met!!!!ric!!.!!name", "many.consecutive.met_ric.name")
	f("many.non.consecutive.m!e!t!r!i!c!.!name", "many.non.consecutive.m_e_t_r_i_c.name")
	f("how.about.underscores_.!_metric!_!.__!!name", "how.about.underscores.metric.name")
	f("how.about.underscores.middle.met!_!_ric.name", "how.about.underscores.middle.met_ric.name")
}
