package memory

import (
	"flag"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	allowedPercent = flag.Float64("memory.allowedPercent", 60, `Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache which will result in higher disk IO usage`)
	allowedBytes   = flagutil.NewBytes("memory.allowedBytes", 0, `Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache resulting in higher disk IO usage`)
)

var (
	allowedMemory   int
	remainingMemory int
)

var once sync.Once

func initOnce() {
	if !flag.Parsed() {
		// Do not use logger.Panicf here, since logger may be uninitialized yet.
		panic(fmt.Errorf("BUG: memory.Allowed must be called only after flag.Parse call"))
	}
	mem := sysTotalMemory()
	if allowedBytes.N <= 0 {
		if *allowedPercent < 1 || *allowedPercent > 200 {
			logger.Panicf("FATAL: -memory.allowedPercent must be in the range [1...200]; got %g", *allowedPercent)
		}
		percent := *allowedPercent / 100
		allowedMemory = int(float64(mem) * percent)
		remainingMemory = mem - allowedMemory
		logger.Infof("limiting caches to %d bytes, leaving %d bytes to the OS according to -memory.allowedPercent=%g", allowedMemory, remainingMemory, *allowedPercent)
	} else {
		allowedMemory = allowedBytes.N
		remainingMemory = mem - allowedMemory
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
