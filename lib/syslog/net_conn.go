// defines the netConn and the applicable methods
package syslog

import (
	"net"
)

type netConn struct {
	conn net.Conn
}

// writeString formats and frames message before writing it over the connection
func (n *netConn) writeString(framer framer, formatter formatter, priority int64, hostname, msg string) error {
	formattedMessage := framer(formatter(priority, hostname, msg))
	_, err := n.conn.Write([]byte(formattedMessage+"\n"))
	return err
}

// close shuts down the connection
func (n *netConn) close() error {
	return n.conn.Close()
}