package logger

import (
	"flag"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
)

func logAllFlags() {
	Infof("build version: %s", buildinfo.Version)
	Infof("command line flags")
	flag.VisitAll(func(f *flag.Flag) {
		lname := strings.ToLower(f.Name)
		value := f.Value.String()
		if strings.Contains(lname, "pass") || strings.Contains(lname, "key") || strings.Contains(lname, "secret") {
			// Do not expose passwords and keys to prometheus.
			value = "secret"
		}
		Infof("flag %q = %q", f.Name, value)
	})
}
