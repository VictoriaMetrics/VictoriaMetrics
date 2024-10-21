// Current File defines the syslogWriter and the applicable methods
package syslog

import (
	"fmt"
	"net"
	"os"
	"time"
)

type syslogWriter struct {
	conn      serverConn
	formatter formatter
	framer    framer
	sysCfg    *config
}

type serverConn interface {
	writeString(framer framer, formatter formatter, priority int64, hostname string, s string) error
	close() error
}

// basicDialer connects to the syslog server
func (w *syslogWriter) basicDialer() (serverConn, error) {
	c, err := net.Dial(w.sysCfg.SyslogConfig.Protocol, fmt.Sprintf("%s:%d", w.sysCfg.SyslogConfig.RemoteHost, w.sysCfg.SyslogConfig.Port))
	var sc serverConn
	if err == nil {
		sc = &netConn{conn: c}
	}
	return sc, err
}

// connect updates the syslogWriter with a new serverConn
func (w *syslogWriter) connect() (serverConn, error) {
	conn, err := w.basicDialer()
	if err == nil {
		w.conn = conn
		return conn, nil
	} else {
		return nil, err
	}
}

// send forwards the log message to the syslog server
func (w *syslogWriter) send(logLevel, msg string) (int, error) {
	priority := (w.sysCfg.SyslogConfig.Facility << 3) | logLevelMap[logLevel]

	var err error
	if w.conn != nil {
		err = w.conn.writeString(w.framer, w.formatter, priority, w.getHostname(), msg)
		if err == nil {
			return len(msg), nil
		}
	}
	//Establishes a new connection with the syslog server
	_, err = w.connect()

	if err != nil {
		return 0, err
	}

	err = w.conn.writeString(w.framer, w.formatter, priority, w.getHostname(), msg)
	if err != nil {
		return 0, err
	}
	return len(msg), nil
}

// getHostname returns the hostname field to be used in the log message
func (w *syslogWriter) getHostname() string {
	hostname := w.sysCfg.SyslogConfig.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	return hostname
}

// logSender observes the buffered channel for log data to be written to the syslog server
func (w *syslogWriter) logSender() {
	for {
		select {
		case logEntry, ok := <-logChan:
			fmt.Println("message available in log chan at time ", time.Now())
			if ok {
				fmt.Println("Sender sleeping for 5s before send")
				time.Sleep(5 * time.Second)
				_, err := w.send(logEntry.LogLevel, logEntry.Msg)
				if err != nil {
					if !w.sendWithRetry(logEntry) {
						fmt.Fprintf(os.Stderr, "sendcase: unable to send %s message to syslog server after %d retries", logEntry.Msg, w.sysCfg.QueueConfig.Retries)
					}
				}
				fmt.Println("Sender completed sleeping for 5s")
			}
		case <-syslogSendStopCh:
			fmt.Println("stop chan signal received at ", time.Now())
			close(logChan)
			for logEntry := range logChan {
				_, err := w.send(logEntry.LogLevel, logEntry.Msg)
				if err != nil {
					// TODO: Retry can be added if needed here
					fmt.Fprintf(os.Stderr, "unable to send %s message to syslog server", logEntry.Msg)
				}
			}
			return
		}
	}
}

func (w *syslogWriter) sendWithRetry(logEntry SyslogLogContent) bool {
	duration, err := time.ParseDuration(w.sysCfg.QueueConfig.RetryDuration)
	if err != nil {
		fmt.Println("unable to parse retry duration")
	}
	for _ = range w.sysCfg.QueueConfig.Retries {
		_, err := w.send(logEntry.LogLevel, logEntry.Msg)
		if err != nil {
			time.Sleep(duration)
		} else {
			return true
		}
	}
	return false
}
