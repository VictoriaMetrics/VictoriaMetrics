package procutil

import (
	"os"
	"os/signal"
	"syscall"
)

// WaitForSigterm waits for either SIGTERM or SIGINT
//
// Returns the caught signal.
func WaitForSigterm() os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return <-ch
}
