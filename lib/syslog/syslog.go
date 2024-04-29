package syslog

import (
	"flag"
	"fmt"
	"log/syslog"
)

var (
	syslogNetwork = flag.String("syslog.network", "", "network connection to establish. Options are tcp, udp. If network is empty, it will connect to the local syslog server")
	syslogAddress = flag.String("syslog.address", "", "Required: Network address of the syslog server.")
	syslogTag     = flag.String("syslog.tag", "", "tag for syslog. Used os.args[0] if empty")
)

// Mapping between Victoria Metrics and syslog log priorities
var defaultPriorityMap = map[string]syslog.Priority{
	"INFO":  syslog.LOG_INFO,
	"WARN":  syslog.LOG_WARNING,
	"ERROR": syslog.LOG_ERR,
	"FATAL": syslog.LOG_CRIT,
	"PANIC": syslog.LOG_EMERG,
}

// GetSyslogWriter initializes and returns an instence of *syslog.Writer based on the
// syslog network, address, tag for the given priority.
func GetSyslogWriter(priority string) *syslog.Writer {
	syslogPriority := parsePriority(priority)
	switch *syslogNetwork {
	case "", "tcp", "udp":
		// syslogAddress cannot be "" when syslogNetwork is specified (tcp/udp)
		if *syslogNetwork != "" && *syslogAddress == "" {
			panic(`flag 'syslog.address' cannot be "" when 'syslog.network' is specified.`)
		}
		op, err := syslog.Dial(*syslogNetwork, *syslogAddress, syslogPriority, *syslogTag)
		if err != nil {
			panic(fmt.Errorf("error dialing syslog: %w", err))
		}
		return op
	default:
		panic(fmt.Errorf("FATAL: unsupported `syslogNetwork` value: %q; supported values are: 'tcp' or 'udp'", *syslogNetwork))
	}
}

func parsePriority(priority string) syslog.Priority {
	v, found := defaultPriorityMap[priority]
	if !found {
		// default the facility level to LOG_INFO
		return syslog.LOG_INFO
	}
	return v
}
