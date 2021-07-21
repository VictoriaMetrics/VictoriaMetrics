package logger

import (
	"flag"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

func logAllFlags() {
	Infof("build version: %s", buildinfo.Version)
	Infof("command line flags")
	isSetMap := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		isSetMap[f.Name] = true
	})
	flag.VisitAll(func(f *flag.Flag) {
		lname := strings.ToLower(f.Name)
		value := f.Value.String()
		if flagutil.IsSecretFlag(lname) {
			value = "secret"
		}
		isSet := "false"
		if isSetMap[f.Name] {
			isSet = "true"
		}
		Infof("flag %q=%q (is_set=%s)", f.Name, value, isSet)
	})
}
