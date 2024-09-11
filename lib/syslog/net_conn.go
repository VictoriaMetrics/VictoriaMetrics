// defines the netConn and the applicable methods
package syslog

import (
	"net"
)

type netConn struct {
	conn net.Conn
}

func (n *netConn) writeString(framer Framer, formatter Formatter, priority int64, hostname, msg string) error {
	if framer == nil {
		framer = DefaultFramer
	}
	if formatter == nil {
		formatter = DefaultFormatter
	}
	formattedMessage := framer(formatter(priority, sysCfg.Syslog.Host, msg))
	_, err := n.conn.Write([]byte(formattedMessage+"\n"))
	return err
}

func (n *netConn) close() error {
	return n.conn.Close()
}