package memory

import (
	"flag"
	"fmt"
)

var (
	memoryUsageLimit = flag.Int("insert.circuitBreakPercentage", 95, ``)
)

func GetMemoryUsagePercentage() int {
	fmt.Println(sysTotalMemory())
	fmt.Println(sysCurrentMemory())
	return 0
}
