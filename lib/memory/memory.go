package memory

import (
	"flag"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var allowedMemPercent = flag.Float64("memory.allowedPercent", 60, "Allowed percent of system memory VictoriaMetrics caches may occupy")

var allowedMemory int

var once sync.Once

// Allowed returns the amount of system memory allowed to use by the app.
//
// The function must be called only after flag.Parse is called.
func Allowed() int {
	once.Do(func() {
		if !flag.Parsed() {
			// Do not use logger.Panicf here, since logger may be uninitialized yet.
			panic(fmt.Errorf("BUG: memory.Allowed must be called only after flag.Parse call"))
		}
		if *allowedMemPercent < 10 || *allowedMemPercent > 200 {
			logger.Panicf("FATAL: -memory.allowedPercent must be in the range [10...200]; got %f", *allowedMemPercent)
		}
		percent := *allowedMemPercent / 100

		mem := sysTotalMemory()
		allowedMemory = int(float64(mem) * percent)
		logger.Infof("limiting caches to %d bytes of RAM according to -memory.allowedPercent=%g", allowedMemory, *allowedMemPercent)
	})
	return allowedMemory
}
