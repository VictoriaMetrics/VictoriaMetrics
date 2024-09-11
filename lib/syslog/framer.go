// defines the Framers support by syslog
package syslog

import (
	"fmt"
)

type Framer func(in string) string

func DefaultFramer(in string) string {
	return fmt.Sprintf("%d %s", len(in), in)
}

func RFC5425MessageLengthFramer(in string) string {
	return fmt.Sprintf("%d %s", len(in), in)
}
