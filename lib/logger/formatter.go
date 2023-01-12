package logger

import (
	"flag"
	"fmt"
	"log"
	"time"
)

var (
	loggerFormat      = flag.String("loggerFormat", "default", "Format for logs. Possible values: default, json")
	disableTimestamps = flag.Bool("loggerDisableTimestamps", false, "Whether to disable writing timestamps in logs")
	loggerTimezone    = flag.String("loggerTimezone", "UTC", "Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. "+
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

func (f logFormatter) formatMessage(timestamp time.Time, level logLevel, location, msg string) string {
	switch f {
	case formatterDefault:
		if *disableTimestamps {
			msg = fmt.Sprintf("%s\t%s\t%s\n", level, location, msg)
		} else {
			msg = fmt.Sprintf("%s\t%s\t%s\t%s\n", formatTimestamp(timestamp), level, location, msg)
		}
	case formatterJson:
		if *disableTimestamps {
			msg = fmt.Sprintf(
				"{%q:%q,%q:%q,%q:%q}\n",
				fieldLevel, level,
				fieldCaller, location,
				fieldMsg, msg,
			)
		} else {
			msg = fmt.Sprintf(
				"{%q:%q,%q:%q,%q:%q,%q:%q}\n",
				fieldTs, formatTimestamp(timestamp),
				fieldLevel, level,
				fieldCaller, location,
				fieldMsg, msg,
			)
		}
	}
	return msg
}

func initTimezone() {
	tz, err := time.LoadLocation(*loggerTimezone)
	if err != nil {
		log.Fatalf("cannot load timezone %q: %s", *loggerTimezone, err)
	}
	timezone = tz
}

var timezone = time.UTC

func formatTimestamp(timestamp time.Time) string {
	return timestamp.In(timezone).Format("2006-01-02T15:04:05.000Z0700")
}
