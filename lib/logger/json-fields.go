package logger

import "strings"

var (
	fieldTs     = "ts"
	fieldLevel  = "level"
	fieldCaller = "caller"
	fieldMsg    = "msg"
)

func setLoggerJSONFields() {
	fields := strings.Split(*loggerJSONFields, ",")
	for _, f := range fields {
		v := strings.Split(strings.TrimSpace(f), ":")
		if len(v) != 2 {
			continue
		}

		old, new := v[0], v[1]
		switch old {
		case "ts":
			fieldTs = new
		case "level":
			fieldLevel = new
		case "caller":
			fieldCaller = new
		case "msg":
			fieldMsg = new
		}
	}
}
