package logger

import (
	"flag"
	"fmt"
)

var loggerFormat = flag.String("loggerFormat", "default", "Format for logs. Possible values: default, json")

func setLoggerFormat() {
	switch *loggerFormat {
	case "default":
		formatter = formatterDefault
	case "json":
		formatter = formatterJson
	default:
		// We cannot use logger.Panicf here, since the logger isn't initialized yet.
		panic(fmt.Errorf("FATAL: unsupported `-loggerFormat` value: %q; supported values are: default, json", *loggerFormat))
	}
}

var formatter logFormatter = formatterDefault

type logFormatter int

const (
	formatterDefault logFormatter = iota
	formatterJson
)
