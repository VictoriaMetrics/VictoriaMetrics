package netstorage

import (
	"flag"
	"runtime"
	"testing"
)

func TestInitStopNodes(t *testing.T) {
	if err := flag.Set("vmstorageDialTimeout", "1ms"); err != nil {
		t.Fatalf("cannot set vmstorageDialTimeout flag: %s", err)
	}
	for i := 0; i < 3; i++ {
		Init([]string{"host1", "host2"}, 0)
		runtime.Gosched()
		MustStop()
	}

	// Try initializing the netstorage with bigger number of nodes
	for i := 0; i < 3; i++ {
		Init([]string{"host1", "host2", "host3"}, 0)
		runtime.Gosched()
		MustStop()
	}

	// Try initializing the netstorage with smaller number of nodes
	for i := 0; i < 3; i++ {
		Init([]string{"host1"}, 0)
		runtime.Gosched()
		MustStop()
	}
}
