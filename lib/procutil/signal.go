package procutil

import (
	"os"
	"os/signal"
	"syscall"
)

// WaitForSigterm waits for either SIGTERM or SIGINT
//
// Returns the caught signal.
//
// It also prevent from program termination on SIGHUP signal,
// since this signal is frequently used for config reloading.
func WaitForSigterm() os.Signal {
	ch := make(chan os.Signal, 1)
	for {
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		sig := <-ch
		if sig == syscall.SIGHUP {
			// Prevent from the program stop on SIGHUP
			continue
		}
		return sig
	}
}
