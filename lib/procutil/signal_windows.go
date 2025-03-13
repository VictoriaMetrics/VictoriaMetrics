//go:build windows

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
// Windows does not have SIGHUP syscall.
func WaitForSigterm() os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	sig := <-ch
	return sig
}

type sigHUPNotifier struct {
	lock        sync.Mutex
	subscribers []chan<- os.Signal
}

var notifier sigHUPNotifier

// https://golang.org/pkg/os/signal/#hdr-Windows
// https://github.com/golang/go/issues/6948
// SelfSIGHUP sends SIGHUP signal to the subscribed listeners.
func SelfSIGHUP() {
	notifier.notify(syscall.SIGHUP)
}

// NewSighupChan returns a channel, which is triggered on every SelfSIGHUP.
func NewSighupChan() <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	notifier.subscribe(ch)
	return ch
}

func (sn *sigHUPNotifier) subscribe(sub chan<- os.Signal) {
	sn.lock.Lock()
	defer sn.lock.Unlock()
	sn.subscribers = append(sn.subscribers, sub)
}

func (sn *sigHUPNotifier) notify(sig os.Signal) {
	sn.lock.Lock()
	defer sn.lock.Unlock()
	for _, sub := range sn.subscribers {
		select {
		case sub <- sig:
		default:
		}
	}
}
