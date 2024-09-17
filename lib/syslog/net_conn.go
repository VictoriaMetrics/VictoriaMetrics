// defines the netConn and the applicable methods
package syslog

import (
	"net"
)

type netConn struct {
	conn net.Conn
}

func (n *netConn) writeString(framer framer, formatter formatter, priority int64, hostname, msg string) error {
	formattedMessage := framer(formatter(priority, hostname, msg))
	_, err := n.conn.Write([]byte(formattedMessage+"\n"))
	return err
}

func (n *netConn) close() error {
	return n.conn.Close()
}