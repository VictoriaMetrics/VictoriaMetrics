// Current File defines the syslogWriter and the applicable methods
package syslog

import (
	"fmt"
	"net"
	"os"
)

type syslogWriter struct {
	conn serverConn
	formatter formatter
	framer framer
	sysCfg *config
}

type serverConn interface {
	writeString(framer framer, formatter formatter, priority int64, hostname string, s string) error
	close() error
}


func (w *syslogWriter) basicDialer() (serverConn, error) {
	c, err := net.Dial(w.sysCfg.syslogConfig.protocol, fmt.Sprintf("%s:%d", w.sysCfg.syslogConfig.remoteHost, w.sysCfg.syslogConfig.port))
	var sc serverConn
	if err == nil {
		sc = &netConn{conn: c}
	}
	return sc, err
}

func (w *syslogWriter) connect() (serverConn, error) {
	conn, err := w.basicDialer()
	if err == nil {
		w.conn = conn
		return conn, nil
	} else {
		return nil, err
	}
}


//Connects to the syslog server and sends the log message
func (w *syslogWriter) send(logLevel, msg string) (int, error) {
	priority := (w.sysCfg.syslogConfig.facility << 3) | logLevelMap[logLevel]

	var err error
	if w.conn != nil {
		err = w.conn.writeString(w.framer, w.formatter,  priority, w.getHostname(), msg)
		if err == nil {
			return len(msg), nil
		}
	}
	//Establishes a new connection with the syslog server
	_,err = w.connect()
	err = w.conn.writeString(w.framer, w.formatter,  priority, w.getHostname(), msg)
	if err != nil {
		return 0, err
	}
	return len(msg), nil
}

func (w *syslogWriter) getHostname() string {
	hostname := w.sysCfg.syslogConfig.hostname
	if hostname == "" {
		hostname,_ = os.Hostname()
	}
	return hostname
}

//Observes the buffered channel for log data to be written to the syslog server
func (w *syslogWriter) logSender() {
	for logEntry := range logChan {
		_,err := w.send(logEntry.LogLevel, logEntry.Msg)
		for err != nil {
			_,err = w.send(logEntry.LogLevel, logEntry.Msg)
		}
	}
}