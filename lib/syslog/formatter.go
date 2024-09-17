// defines the Formatters support by syslog
package syslog

import (
	"fmt"
	"os"
	"time"
)

//
const appNameMaxLength = 48

type formatter func(priority int64, hostname, content string) string

func truncateStartStr(s string, max int) string {
	if (len(s) > max) {
		return s[len(s) - max:]
	}
	return s
}

// RFC5424Formatter provides an RFC 5424 compliant message.
func defaultFormatter(priority int64, hostname, content string) string {
	version := 1 
	timestamp := time.Now().Format(time.RFC3339)
	appName := truncateStartStr(os.Args[0], appNameMaxLength)
	// Construct the syslog message.
	msg := fmt.Sprintf("<%d>%d %s %s %s %d %s %s %s",
		priority,
		version,
		timestamp,
		hostname,
		appName,
		os.Getpid(),
		"NIL",
		"",
		content)

	return msg
}