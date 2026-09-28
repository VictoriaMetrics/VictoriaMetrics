package apptest

import (
	"testing"
	"time"
)

// Vmcluster represents a typical cluster setup: several vmstorage replicas, one
// vminsert, and one vmselect.
//
// Both Vmsingle and Vmcluster implement the PrometheusWriteQuerier used in
// business logic tests to abstract out the infrastructure.
//
// This type is not suitable for infrastructure tests where custom cluster
// setups are often required.
type Vmcluster struct {
	*Vminsert
	*Vmselect
	Vmstorages []*Vmstorage
}

// ForceFlush forces the ingested data to become visible for searching
// immediately.
func (c *Vmcluster) ForceFlush(t *testing.T) {
	for _, s := range c.Vmstorages {
		s.ForceFlush(t)
	}
}

// ForceMerge is a test helper function that forces the merging of parts.
func (c *Vmcluster) ForceMerge(t *testing.T) {
	for _, s := range c.Vmstorages {
		s.ForceMerge(t)
	}
}

// EnsureBlockingIngestion configures vmisertClient to block until data ingested
// via vminsert is received and processed by all vmstorages. Callers still need
// to call ForceFlush() to ensure that the ingested data is seacheable.
func EnsureBlockingIngestion(t *testing.T, vminsert *Vminsert, vmstorages []*Vmstorage) {
	vminsert.sendBlocking = func(t *testing.T, numRows int, send func()) {
		t.Helper()
		sendAndRecvBlocking(t, vminsert, vmstorages, numRows, send)
	}
}

func sendAndRecvBlocking(t *testing.T, vminsert *Vminsert, vmstorages []*Vmstorage, numRows int, send func()) {
	t.Helper()

	rowsRecvTotal := func() int {
		t.Helper()
		var s int
		for _, vmstorage := range vmstorages {
			s += int(vmstorage.GetMetric(t, "vm_rows_received_by_storage_total"))
		}
		return s
	}

	wantRowsSentCount := vminsert.rpcRowsSentTotal(t) + numRows
	wantRowsRecvCount := rowsRecvTotal() + numRows

	send()

	const (
		retries = 20
		period  = 100 * time.Millisecond
	)
	sent := false
	for range retries {
		d := vminsert.rpcRowsSentTotal(t)
		if d >= wantRowsSentCount {
			sent = true
			break
		}
		time.Sleep(period)
	}
	if !sent {
		t.Fatalf("timed out while waiting for inserted rows to be sent to vmstorage")
	}

	for range retries {
		d := rowsRecvTotal()
		if d >= wantRowsRecvCount {
			return
		}
		time.Sleep(period)
	}
	t.Fatalf("timed out while waiting for vmstorage to insert the received rows")
}
