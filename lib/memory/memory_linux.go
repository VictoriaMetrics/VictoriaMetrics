package memory

import (
	"io/ioutil"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const maxInt = int(^uint(0) >> 1)

func sysTotalMemory() int {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		logger.Panicf("FATAL: error in syscall.Sysinfo: %s", err)
	}
	totalMem := maxInt
	if uint64(maxInt)/uint64(si.Totalram) > uint64(si.Unit) {
		totalMem = int(uint64(si.Totalram) * uint64(si.Unit))
	}

	// Try determining the amount of memory inside docker container.
	// See https://stackoverflow.com/questions/42187085/check-mem-limit-within-a-docker-container .
	data, err := ioutil.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		// Try determining the amount of memory inside lxc container.
		mem, err := readLXCMemoryLimit(totalMem)
		if err != nil {
			return totalMem
		}
		return mem
	}
	mem, err := readPositiveInt(data, totalMem)
	if err != nil {
		return totalMem
	}
	if mem != totalMem {
		return mem
	}

	// Try reading LXC memory limit, since it looks like the cgroup limit doesn't work
	mem, err = readLXCMemoryLimit(totalMem)
	if err != nil {
		return totalMem
	}
	return mem
}

func readLXCMemoryLimit(totalMem int) (int, error) {
	// Read memory limit according to https://unix.stackexchange.com/questions/242718/how-to-find-out-how-much-memory-lxc-container-is-allowed-to-consume
	// This should properly determine the limit inside lxc container.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/84
	cmd := exec.Command("/bin/sh", "-c",
		`cat /sys/fs/cgroup/memory$(cat /proc/self/cgroup | grep memory | cut -d: -f3)/memory.limit_in_bytes`)
	data, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return readPositiveInt(data, totalMem)
}

func readPositiveInt(data []byte, maxN int) (int, error) {
	for len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	n, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil {
		return 0, err
	}
	if int64(n) < 0 || int64(int(n)) != int64(n) {
		// Int overflow.
		return maxN, nil
	}
	ni := int(n)
	if ni > maxN {
		return maxN, nil
	}
	return ni, nil
}
