package syslog

// defines the syslog severity
const (
	EMERG int64 = iota
	ALERT
	CRIT
	ERROR
	WARN
	NOTICE
	INFO
	DEBUG
)

// Mapping vm log levels to syslog severity
var logLevelMap = map[string]int64{
	"info":    INFO,
	"warn":    WARN,
	"error":   ERROR,
	"fatal":   CRIT,
	"panic":   EMERG,
}
