package logger

import (
	"flag"
	"fmt"
	"log"
	"time"
)

var (
	loggerFormat   = flag.String("loggerFormat", "default", "Format for logs. Possible values: default, json")
	loggerTimezone = flag.String("loggerTimezone", "UTC", "Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. "+
		"For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local")
)

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

func initTimezone() {
	tz, err := time.LoadLocation(*loggerTimezone)
	if err != nil {
		log.Fatalf("cannot load timezone %q: %s", *loggerTimezone, err)
	}
	timezone = tz
}

var timezone = time.UTC
