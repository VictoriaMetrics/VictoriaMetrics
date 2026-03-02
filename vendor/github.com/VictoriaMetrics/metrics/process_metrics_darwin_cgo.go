//go:build darwin && !ios && cgo

package metrics

/*
int vm_get_memory_info(unsigned long long *rss, unsigned long long *vs);
*/
import "C"

import (
	"fmt"
)

func getMemory() (*memoryInfo, error) {
	var rss, vsize C.ulonglong

	if err := C.vm_get_memory_info(&rss, &vsize); err != 0 {
		return nil, fmt.Errorf("task_info() failed with 0x%x", int(err))
	}

	return &memoryInfo{vsize: uint64(vsize), rss: uint64(rss)}, nil
}
