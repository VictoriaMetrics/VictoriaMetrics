package memory

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
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
	allowedMemory           int
	remainingMemory         int
	memoryLimit             int
	currentMemory           atomic.Int64
	currentMemoryPercentage atomic.Int32
)
var (
	once        sync.Once
	watcherOnce sync.Once
)

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

	currentUsedBytes := sysCurrentMemory()
	currentMemory.Store(int64(currentUsedBytes))
	currentMemoryPercentage.Store(int32(currentUsedBytes * 100 / memoryLimit))

	go func() {
		// Register SIGHUP handler for config reload before loadRelabelConfigs.
		// This guarantees that the config will be re-read if the signal arrives just after loadRelabelConfig.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240
		sighupCh := procutil.NewSighupChan()
		t := time.NewTicker(time.Second * 5)
		defer t.Stop()
		for {
			select {
			case <-sighupCh:
				return
			case <-t.C:
				currentUsedBytes = sysCurrentMemory()
				currentMemory.Store(int64(currentUsedBytes))
				currentMemoryPercentage.Store(int32(currentUsedBytes * 100 / memoryLimit))
			}
		}
	}()
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

// Current return memory usage in byte. The value is updated every 5 seconds.
func Current() int {
	once.Do(initOnce)
	return int(currentMemory.Load())
}

// CurrentPercentage return memory usage percentage in [0-100] int. The value is updated every 5 seconds.
func CurrentPercentage() int {
	once.Do(initOnce)
	return int(currentMemoryPercentage.Load())
}
