package cgroup

// GetMemoryLimit returns cgroup memory limit
func GetMemoryLimit() int64 {
	// Try determining the amount of memory inside docker container.
	// See https://stackoverflow.com/questions/42187085/check-mem-limit-within-a-docker-container
	//
	// Read memory limit according to https://unix.stackexchange.com/questions/242718/how-to-find-out-how-much-memory-lxc-container-is-allowed-to-consume
	// This should properly determine the limit inside lxc container.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/84
	n, err := readInt64("/sys/fs/cgroup/memory/memory.limit_in_bytes", "cat /sys/fs/cgroup/memory$(cat /proc/self/cgroup | grep memory | cut -d: -f3)/memory.limit_in_bytes")
	if err != nil {
		return 0
	}
	return n
}

// GetHierarchicalMemoryLimit returns hierarchical memory limit
func GetHierarchicalMemoryLimit() int64 {
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/699
	n, err := readInt64FromCommand("cat /sys/fs/cgroup/memory/memory.stat | grep hierarchical_memory_limit | cut -d' ' -f 2")
	if err == nil {
		return n
	}
	n, err = readInt64FromCommand(
		"cat /sys/fs/cgroup/memory$(cat /proc/self/cgroup | grep memory | cut -d: -f3)/memory.stat | grep hierarchical_memory_limit | cut -d' ' -f 2")
	if err != nil {
		return 0
	}
	return n
}
