// defines the Framers support by syslog
package syslog

import (
	"fmt"
)

type framer func(in string) string

func defaultFramer(in string) string {
	return fmt.Sprintf("%d %s", len(in), in)
}