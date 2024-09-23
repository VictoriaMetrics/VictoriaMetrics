// defines the Framers support by syslog
package syslog

import (
	"fmt"
)

type framer func(in string) string

// defaultFramer prepends the msg length to the front of the provided msg(RFC 5425)
func defaultFramer(in string) string {
	return fmt.Sprintf("%d %s", len(in), in)
}