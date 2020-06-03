package memory

import (
	"flag"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var allowedMemPercent = flag.Float64("memory.allowedPercent", 60, "Allowed percent of system memory VictoriaMetrics caches may occupy. "+
	"Too low value may increase cache miss rate, which usually results in higher CPU and disk IO usage. "+
	"Too high value may evict too much data from OS page cache, which will result in higher disk IO usage")

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
	if *allowedMemPercent < 1 || *allowedMemPercent > 200 {
		logger.Panicf("FATAL: -memory.allowedPercent must be in the range [1...200]; got %f", *allowedMemPercent)
	}
	percent := *allowedMemPercent / 100

	mem := sysTotalMemory()
	allowedMemory = int(float64(mem) * percent)
	remainingMemory = mem - allowedMemory
	logger.Infof("limiting caches to %d bytes, leaving %d bytes to the OS according to -memory.allowedPercent=%g", allowedMemory, remainingMemory, *allowedMemPercent)
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
