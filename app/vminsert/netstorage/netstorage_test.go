package netstorage

import (
	"flag"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consistenthash"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

func TestInitStopNodes(t *testing.T) {
	if err := flag.Set("vmstorageDialTimeout", "1ms"); err != nil {
		t.Fatalf("cannot set vmstorageDialTimeout flag: %s", err)
	}
	for range 3 {
		Init([]string{"host1", "host2"}, 0)
		runtime.Gosched()
		MustStop()
	}

	// Try initializing the netstorage with bigger number of nodes
	for range 3 {
		Init([]string{"host1", "host2", "host3"}, 0)
		runtime.Gosched()
		MustStop()
	}

	// Try initializing the netstorage with smaller number of nodes
	for range 3 {
		Init([]string{"host1"}, 0)
		runtime.Gosched()
		MustStop()
	}

	// Mixed alias=addr and bare addr entries.
	for range 3 {
		Init([]string{"node-a=host1", "node-b=host2", "host3"}, 0)
		runtime.Gosched()
		MustStop()
	}
}

func TestParseStorageNodes(t *testing.T) {
	f := func(in []string, want []StorageNode, wantErr bool) {
		t.Helper()
		got, err := ParseStorageNodes(in)
		if (err != nil) != wantErr {
			t.Fatalf("ParseStorageNodes(%v) error=%v, wantErr=%v", in, err, wantErr)
		}
		if wantErr {
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ParseStorageNodes(%v) = %#v, want %#v", in, got, want)
		}
	}

	// Bare addresses: alias defaults to address.
	f([]string{"host1", "host2:8400"}, []StorageNode{
		{Alias: "host1", Addr: "host1"},
		{Alias: "host2:8400", Addr: "host2:8400"},
	}, false)

	// Aliased addresses.
	f([]string{"node-a=10.0.0.1:8400", "node-b=10.0.0.2:8400"}, []StorageNode{
		{Alias: "node-a", Addr: "10.0.0.1:8400"},
		{Alias: "node-b", Addr: "10.0.0.2:8400"},
	}, false)

	// Errors are surfaced (used by Enterprise discovery providers to validate
	// user-supplied file content).
	f([]string{"=10.0.0.1"}, nil, true)
	f([]string{"node-a="}, nil, true)
}

func TestInitWithStorageNodes(t *testing.T) {
	if err := flag.Set("vmstorageDialTimeout", "1ms"); err != nil {
		t.Fatalf("cannot set vmstorageDialTimeout flag: %s", err)
	}
	// Init via the typed API used by Enterprise discovery providers.
	for range 3 {
		InitWithStorageNodes([]StorageNode{
			{Alias: "node-a", Addr: "host1"},
			{Alias: "node-b", Addr: "host2"},
		}, 0)
		runtime.Gosched()
		MustStop()
	}
	// Empty Alias should default to Addr (preserves legacy hashing).
	for range 2 {
		InitWithStorageNodes([]StorageNode{
			{Addr: "host1"},
			{Addr: "host2"},
		}, 0)
		runtime.Gosched()
		MustStop()
	}
}

func TestParseStorageNodeSpecs(t *testing.T) {
	f := func(in []string, want []storageNodeSpec, wantErr bool) {
		t.Helper()
		got, err := parseStorageNodeSpecs(in)
		if (err != nil) != wantErr {
			t.Fatalf("parseStorageNodeSpecs(%v) error=%v, wantErr=%v", in, err, wantErr)
		}
		if wantErr {
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parseStorageNodeSpecs(%v) = %#v, want %#v", in, got, want)
		}
	}

	// Bare addresses: alias defaults to address (legacy behavior).
	f([]string{"host1", "host2:8400"}, []storageNodeSpec{
		{alias: "host1", addr: "host1"},
		{alias: "host2:8400", addr: "host2:8400"},
	}, false)

	// Mixed alias=addr and bare entries.
	f([]string{"node-a=10.0.0.1:8400", "host2"}, []storageNodeSpec{
		{alias: "node-a", addr: "10.0.0.1:8400"},
		{alias: "host2", addr: "host2"},
	}, false)

	// Address may itself contain '='; only the first '=' separates alias.
	f([]string{"node-a=k=v"}, []storageNodeSpec{
		{alias: "node-a", addr: "k=v"},
	}, false)

	// Empty alias before '='.
	f([]string{"=10.0.0.1"}, nil, true)

	// Empty address after '='.
	f([]string{"node-a="}, nil, true)
}

// TestAliasBasedShardingIsStable verifies that the consistent-hash ring keys off
// the alias, so renaming the underlying address keeps the routing decisions
// identical as long as the alias stays the same.
func TestAliasBasedShardingIsStable(t *testing.T) {
	specsOld, err := parseStorageNodeSpecs([]string{"node-a=10.0.0.1:8400", "node-b=10.0.0.2:8400", "node-c=10.0.0.3:8400"})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	specsNew, err := parseStorageNodeSpecs([]string{"node-a=192.168.1.1:8400", "node-b=192.168.1.2:8400", "node-c=192.168.1.3:8400"})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	aliasesOf := func(specs []storageNodeSpec) []string {
		out := make([]string, len(specs))
		for i, s := range specs {
			out[i] = s.alias
		}
		return out
	}

	hOld := consistenthash.NewConsistentHash(aliasesOf(specsOld), 0)
	hNew := consistenthash.NewConsistentHash(aliasesOf(specsNew), 0)

	// Sample a range of synthetic series hashes and ensure both rings pick the
	// same node index, proving that changing the address (while keeping the
	// alias) preserves shard placement.
	for h := uint64(0); h < 1024; h++ {
		if got, want := hNew.GetNodeIdx(h, nil), hOld.GetNodeIdx(h, nil); got != want {
			t.Fatalf("alias-based ring not stable for h=%d: got %d, want %d", h, got, want)
		}
	}
}

func TestAllowRerouting(t *testing.T) {
	originDisableRerouting := *disableRerouting
	t.Cleanup(func() {
		*disableRerouting = originDisableRerouting
	})
	// Enable rerouting for the test
	*disableRerouting = false

	newStorage := func(avgSaturation float64, ready bool) *storageNode {
		sn := &storageNode{
			avgSaturation: newMovingAverage(180),
			dialer:        netutil.NewTCPDialer(metrics.NewSet(), "aName", "anAddr", time.Second, time.Second),
		}
		sn.isBroken.Store(!ready)
		sn.avgSaturation.Set(avgSaturation)

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

	// rerouting is triggered on the slowest node if cluster median saturation less than or equal 0.8
	f([]*storageNode{
		newStorage(0.81, true),
		newStorage(0.79, true),
		newStorage(0.1, true),
	}, 0, true)

	// four nodes median test
	f([]*storageNode{
		newStorage(0.82, true),
		newStorage(0.81, true),
		newStorage(0.79, true),
		newStorage(0.1, true),
	}, 0, true)

	// rerouting not triggered because rerouting disabled by flag
	*disableRerouting = true
	f([]*storageNode{
		newStorage(0.81, true),
		newStorage(0.79, true),
		newStorage(0.79, true),
	}, 0, false)
	*disableRerouting = false

	// rerouting not triggered because cluster median saturation more than 0.8
	f([]*storageNode{
		newStorage(0.81, true),
		newStorage(0.801, true),
		newStorage(0.1, true),
	}, 0, false)

	// four nodes median test
	f([]*storageNode{
		newStorage(0.82, true),
		newStorage(0.82, true),
		newStorage(0.79, true),
		newStorage(0.1, true),
	}, 0, false)

	// rerouting not triggered because snSource not the slowest
	f([]*storageNode{
		newStorage(0.81, true),
		newStorage(0.801, true),
		newStorage(0.1, true),
	}, 2, false)

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
		newStorage(0, false),
	}, 0, true)
}

func TestGetMaxBufSizePerStorageNode(t *testing.T) {
	f := func(mem, concurrency, netstorageCount, threshold int) {
		t.Helper()
		result := getMaxBufSizePerInsertCtxStorageNode(mem, concurrency, netstorageCount)
		if result > threshold {
			t.Fatalf("getMaxBufSizePerInsertCtxStorageNode() returned %d MiB results, expected not exceeding %d MiB", result/1024/1024, threshold/1024/1024)
		}
	}

	// 26MiB * 2 * 5 = 260 MiB, roughly 1/4 of 1 GiB.
	f(1*1024*1024*1024, 2, 5, 26*1024*1024)

	// not enough memory, high concurrency and storages
	f(1*1024*1024*1024, 96, 50, 1*1024*1024)

	// a lot of memory, a few storages & concurrency
	f(1*1024*1024*1024, 1, 5, 30*1024*1024)
}
