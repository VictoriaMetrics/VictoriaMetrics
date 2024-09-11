// Current File defines the syslogWriter and the applicable methods
package syslog

import (
	"strings"
	"fmt"
	"net"
)

type syslogWriter struct {
	conn serverConn
	formatter Formatter
	framer Framer
}

type serverConn interface {
	writeString(framer Framer, formatter Formatter, priority int64, hostname string, s string) error
	close() error
}


func (w *syslogWriter) basicDialer() (serverConn, error) {
	c, err := net.Dial(sysCfg.Syslog.Protocol, fmt.Sprintf("%s:%d", sysCfg.Syslog.Host, sysCfg.Syslog.Port))
	var sc serverConn
	if err == nil {
		sc = &netConn{conn: c}
	}
	return sc, err
}

func (w *syslogWriter) connect() (serverConn, error) {
	conn := w.conn

	if conn != nil {
		conn.close()
		w.conn = nil
	}

	var err error
	conn, err = w.basicDialer()
	if err == nil {
		w.conn = conn

		return conn, nil
	} else {
		return nil, err
	}
}

// Close closes a connection to the syslog daemon.
func (w *syslogWriter) Close() error {
	if w.conn != nil {
		err := w.conn.close()
		w.conn = nil
		return err
	}
	return nil
}


/*
Connects to the syslog server and sends the log message
*/
func (w *syslogWriter) send(logLevel, msg string) (int, error) {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}

	_,err := w.connect()
	if err != nil {
		return 0, err
	}

	priority := (sysCfg.Syslog.Facility << 3) | logLevelMap[logLevel]
	err = w.conn.writeString(w.framer, w.formatter,  priority, "nokia.com", msg)
	if err != nil {
		return 0, err
	}
	return len(msg), nil
}

/*
Observes the buffered channel for log data to be written to the syslog server
Input: nil
Output: error (TO_BE_DONE)
*/
func (w *syslogWriter) LogSender() {
	for logEntry := range logChan {
		_,err := w.send(logEntry.LogLevel, logEntry.Msg)
		for err != nil {
			_,err = w.send(logEntry.LogLevel, logEntry.Msg)
		}
	}
}
