package memory

import (
	"flag"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var (
	allowedPercent = flag.Float64("memory.allowedPercent", 60, `Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage`)
	allowedBytes   = flagutil.NewBytes("memory.allowedBytes", 0, `Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage`)
)

var _ = metrics.NewGauge("process_memory_limit_bytes", func() float64 {
	return float64(memoryLimit)
})

var (
	allowedMemory   int
	remainingMemory int
	memoryLimit     int
)
var once sync.Once

func initOnce() {
	if !flag.Parsed() {
		// Do not use logger.Panicf here, since logger may be uninitialized yet.
		panic(fmt.Errorf("BUG: memory.Allowed must be called only after flag.Parse call"))
	}
	memoryLimit = sysTotalMemory()
	if allowedBytes.N <= 0 {
		if *allowedPercent < 1 || *allowedPercent > 100 {
			logger.Fatalf("FATAL: -memory.allowedPercent must be in the range [1...100]; got %g", *allowedPercent)
		}
		percent := *allowedPercent / 100
		allowedMemory = int(float64(memoryLimit) * percent)
		remainingMemory = memoryLimit - allowedMemory
		logger.Infof("limiting caches to %d bytes, leaving %d bytes to the OS according to -memory.allowedPercent=%g", allowedMemory, remainingMemory, *allowedPercent)
	} else {
		allowedMemory = allowedBytes.IntN()
		remainingMemory = memoryLimit - allowedMemory
		logger.Infof("limiting caches to %d bytes, leaving %d bytes to the OS according to -memory.allowedBytes=%s", allowedMemory, remainingMemory, allowedBytes.String())
	}
}

// Allowed returns the amount of system memory allowed to use by the app.
//
// The function must be called only after flag.Parse is called.
func Allowed() int {
	once.Do(initOnce)
	return allowedMemory
}

// Remaining returns the amount of memory remaining to the OS.
//
// This function must be called only after flag.Parse is called.
func Remaining() int {
	once.Do(initOnce)
	return remainingMemory
}
