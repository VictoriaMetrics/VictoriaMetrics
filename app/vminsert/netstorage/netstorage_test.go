package netstorage

import (
	"flag"
	"runtime"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
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

func TestAllowRerouting(t *testing.T) {
	originDisableRerouting := *disableRerouting
	t.Cleanup(func() {
		*disableRerouting = originDisableRerouting
	})
	// Enable rerouting for the test
	*disableRerouting = false

	newStorage := func(avgSendDuration float64, ready bool) *storageNode {
		sn := &storageNode{
			avgSendDuration: newMovingAverage(movingAvgAge),
			avgIdleDuration: newMovingAverage(movingAvgAge),
			dialer:          netutil.NewTCPDialer(metrics.NewSet(), "aName", "anAddr", time.Second, time.Second),
		}
		sn.isBroken.Store(!ready)
		sn.avgSendDuration.Set(avgSendDuration)
		sn.avgIdleDuration.Set(1 - avgSendDuration)

		return sn
	}

	f := func(sns []*storageNode, snSourceIdx int, expected bool) {
		t.Helper()

		snSource := sns[snSourceIdx]

		actual := allowRerouting(snSource, sns)

		if actual != expected {
			t.Errorf("unexpected allowRerouting result for snSourceIdx=%d from %d storages; got %v; want %v", snSourceIdx, len(sns), actual, expected)
		}
	}

	// rerouting triggered on the slowest node if cluster avg saturation less than or equal 0.5
	f([]*storageNode{
		newStorage(0.801, true),
		newStorage(0.5, true),
		newStorage(0.5, true),
	}, 0, true)

	// rerouting not triggered because rerouting disabled by flag
	*disableRerouting = true
	f([]*storageNode{
		newStorage(0.801, true),
		newStorage(0.5, true),
		newStorage(0.5, true),
	}, 0, false)
	*disableRerouting = false

	// rerouting not triggered because cluster avg saturation more than 0.5
	f([]*storageNode{
		newStorage(0.801, true),
		newStorage(0.501, true),
		newStorage(0.501, true),
	}, 0, false)

	// rerouting not triggered because the snSource node not slow enough
	f([]*storageNode{
		newStorage(0.79, true),
		newStorage(0.5, true),
		newStorage(0.5, true),
	}, 0, false)

	// rerouting not triggered because snSource not the slowest
	f([]*storageNode{
		newStorage(0.81, true),
		newStorage(0.805, true),
		newStorage(0.01, true),
		newStorage(0.01, true),
	}, 1, false)

	// rerouting not triggered if not enough nodes
	f([]*storageNode{
		newStorage(0.81, true),
		newStorage(0.01, true),
	}, 0, false)

	// rerouting not triggered if not enough ready nodes
	f([]*storageNode{
		newStorage(0.81, true),
		newStorage(0.01, true),
		newStorage(0.01, false),
		newStorage(0.01, false),
	}, 0, false)

	// rerouting triggered if enough ready nodes
	f([]*storageNode{
		newStorage(0.81, true),
		newStorage(0.01, true),
		newStorage(0.01, true),
		newStorage(0.01, false),
	}, 0, true)
}
