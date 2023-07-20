package remotewrite

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestDebugClient_Push(t *testing.T) {
	testSrv := newRWServer()
	oldAddr := *addr
	*addr = testSrv.URL
	defer func() {
		*addr = oldAddr
	}()

	client, err := NewDebugClient()
	if err != nil {
		t.Fatalf("failed to create debug client: %s", err)
	}

	const rowsN = 100
	var sent int
	for i := 0; i < rowsN; i++ {
		s := prompbmarshal.TimeSeries{
			Samples: []prompbmarshal.Sample{{
				Value:     float64(i),
				Timestamp: time.Now().Unix(),
			}},
		}
		err := client.Push(s)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		if err == nil {
			sent++
		}
	}
	if sent == 0 {
		t.Fatalf("0 series sent")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close client: %s", err)
	}
	got := testSrv.accepted()
	if got != sent {
		t.Fatalf("expected to have %d series; got %d", sent, got)
	}
}
