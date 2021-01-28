// +build windows

package procutil

import (
	"os"
	"os/signal"
	"sync"
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
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	for {
		sig := <-ch
		if sig == syscall.SIGHUP {
			// Prevent from the program stop on SIGHUP
			continue
		}
		return sig
	}
}

var sigPool []chan os.Signal
var sigPoolLock sync.Mutex

// https://golang.org/pkg/os/signal/#hdr-Windows
// https://github.com/golang/go/issues/6948
// SelfSIGHUP sends SIGHUP signal to the subscribed listeners.
func SelfSIGHUP() {
	sigPoolLock.Lock()
	defer sigPoolLock.Unlock()
	for _, c := range sigPool {
		select {
		case c <- syscall.SIGHUP:
		default:
		}
	}
}

// NewSighupChan returns a channel, which is triggered on every SelfSIGHUP.
func NewSighupChan() <-chan os.Signal {
	sigPoolLock.Lock()
	defer sigPoolLock.Unlock()
	ch := make(chan os.Signal, 1)
	sigPool = append(sigPool, ch)
	return ch
}
