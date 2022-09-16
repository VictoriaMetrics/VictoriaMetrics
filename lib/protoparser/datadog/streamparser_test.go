package datadog

import (
	"testing"
)

func TestSanitizeNameSuccess(t *testing.T) {
	f := func(metricName, expected string) {
		t.Helper()
		result, _ := sanitizeName(metricName)
		if result != expected {
			t.Fatalf("unexpected metric name; got %s; want %s", result, expected)
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

func TestSanitizeNameVerifyReplacements(t *testing.T) {
	f := func(metricName, error string) {
		t.Helper()
		result, _ := sanitizeName(metricName)
		if result == metricName {
			t.Fatalf(error)
		}
	}
	f("some.metric!.name", "special character not replaced")
	f("some.metric_.name", "underscore before dot not allowed")
	f("some.metr__ic.name", "2 or more consecutive underscore are not allowed")
}
