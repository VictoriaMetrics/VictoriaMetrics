package netstorage

import (
	"runtime"
	"testing"
)

func TestInitStopNodes(t *testing.T) {
	for i := 0; i < 3; i++ {
		Init([]string{"host1", "host2"}, 0)
		runtime.Gosched()
		MustStop()
	}
}
