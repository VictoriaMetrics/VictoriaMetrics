package memory

import (
	"io/ioutil"
	"strconv"
	"syscall"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func sysTotalMemory() int {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		logger.Panicf("FATAL: error in syscall.Sysinfo: %s", err)
	}
	totalMem := int(si.Totalram) * int(si.Unit)

	// Try determining the amount of memory inside docker container.
	// See https://stackoverflow.com/questions/42187085/check-mem-limit-within-a-docker-container .
	data, err := ioutil.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		return totalMem
	}
	for len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	mem, err := strconv.Atoi(string(data))
	if err != nil {
		return totalMem
	}
	if mem > totalMem {
		mem = totalMem
	}
	return mem
}
