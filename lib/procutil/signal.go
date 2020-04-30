package procutil

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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

// SelfSIGHUP sends SIGHUP signal to the current process.
func SelfSIGHUP() {
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
		logger.Panicf("FATAL: cannot send SIGHUP to itself: %s", err)
	}
}
