package logger

import (
	"flag"
	"log"
	"strings"
)

var loggerJSONFields = flag.String("loggerJSONFields", "", `Allows renaming fields in JSON formatted logs. `+
	`Example: "ts:timestamp,msg:message" renames "ts" to "timestamp" and "msg" to "message". `+
	"Supported fields: ts, level, caller, msg")

func setLoggerJSONFields() {
	if *loggerJSONFields == "" {
		return
	}
	fields := strings.Split(*loggerJSONFields, ",")
	for _, f := range fields {
		f = strings.TrimSpace(f)
		v := strings.Split(f, ":")
		if len(v) != 2 {
			log.Fatalf("missing ':' delimiter in -loggerJSONFields=%q for %q item", *loggerJSONFields, f)
		}

		name, value := v[0], v[1]
		switch name {
		case "ts":
			fieldTs = value
		case "level":
			fieldLevel = value
		case "caller":
			fieldCaller = value
		case "msg":
			fieldMsg = value
		default:
			log.Fatalf("unexpected json field name in -loggerJSONFields=%q: %q; supported values: ts, level, caller, msg", *loggerJSONFields, name)
		}
	}
}

var (
	fieldTs     = "ts"
	fieldLevel  = "level"
	fieldCaller = "caller"
	fieldMsg    = "msg"
)
