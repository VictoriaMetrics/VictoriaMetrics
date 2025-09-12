package configwatcher

import (
	"flag"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

// TBD: migrate /-/reload handler to the package
// TBD: print registered configs in 10s after the start
// TBD: print what reload methods enabled

type handler struct {
	flag    string
	handler func()
}

var configCheckInterval = flag.Duration("configCheckInterval", 0, "TBD")

var signalHandlers []handler

var checkIntervalHandlers []handler

var mux = sync.Mutex{}

func RegisterHandler(flag string, handlerFn func()) {
	RegisterSignalHandler(flag, handlerFn)
	RegisterCheckIntervalHandler(flag, handlerFn)
}

func RegisterSignalHandler(flag string, handlerFn func()) {
	mux.Lock()
	defer mux.Unlock()

	signalHandlers = append(signalHandlers, handler{
		flag:    flag,
		handler: handlerFn,
	})
}

func RegisterCheckIntervalHandler(flag string, handlerFn func()) {
	mux.Lock()
	defer mux.Unlock()

	checkIntervalHandlers = append(checkIntervalHandlers, handler{
		flag:    flag,
		handler: handlerFn,
	})
}

func UnregisterHandler(flag string) {
	mux.Lock()
	defer mux.Unlock()

	newCheckIntervalHandlers := make([]handler, 0, len(checkIntervalHandlers))
	for _, h := range checkIntervalHandlers {
		if h.flag != flag {
			newCheckIntervalHandlers = append(newCheckIntervalHandlers, h)
		}
	}
	newSignalHandlers := make([]handler, 0, len(signalHandlers))
	for _, h := range signalHandlers {
		if h.flag != flag {
			newSignalHandlers = append(newSignalHandlers, h)
		}
	}

	checkIntervalHandlers = newCheckIntervalHandlers
}

var stopChan chan struct{}

func Init() {
	stopChan = make(chan struct{})
	go func() {
		sighupCh := procutil.NewSighupChan()

		var tickerCh <-chan time.Time
		if *configCheckInterval > 0 {
			ticker := time.NewTicker(*configCheckInterval)
			tickerCh = ticker.C
			defer ticker.Stop()
		}
		for {

			select {
			case <-sighupCh:
				mux.Lock()
				for _, h := range signalHandlers {
					h.handler()
				}
				mux.Unlock()
			case <-tickerCh:
				mux.Lock()
				for _, h := range checkIntervalHandlers {
					h.handler()
				}
				mux.Unlock()
			case <-stopChan:
				return
			}
		}
	}()
}

// Method for BC
func EnableCheckInterval(dur time.Duration) {
	mux.Lock()
	defer mux.Unlock()

	if dur > *configCheckInterval {
		*configCheckInterval = dur
	}
}

// Stop stops Prometheus scraper.
func Stop() {
	close(stopChan)
}
