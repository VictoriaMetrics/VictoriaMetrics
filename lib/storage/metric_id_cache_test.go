package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMetricIDCache_SetHas(t *testing.T) {
	c := newMetricIDCache()
	defer c.MustStop()

	metricIDMin := uint64(time.Now().UnixNano())

	for i := range uint64(1_000_000) {
		c.Set(metricIDMin + i)
	}

	for i := range uint64(1_000_000) {
		metricID := metricIDMin + i
		if !c.Has(metricID) {
			t.Fatalf("metricID not found: %d", metricID)
		}
	}
}

func TestMetricIDCache_SetHas_Concurrent(t *testing.T) {
	c := newMetricIDCache()
	defer c.MustStop()

	const (
		numMetricIDs = 1_000_000
		concurrency  = 1000
	)

	var writeWG, readWG sync.WaitGroup
	writeCh := make(chan uint64, concurrency)
	readCh := make(chan uint64, concurrency)
	for range concurrency {
		writeWG.Add(1)
		go func() {
			for metricID := range writeCh {
				c.Set(metricID)
				readCh <- metricID
			}
			writeWG.Done()
		}()

		readWG.Add(1)
		go func() {
			for metricID := range readCh {
				if !c.Has(metricID) {
					panic(fmt.Sprintf("metricID not found: %d", metricID))
				}
			}
			readWG.Done()
		}()
	}

	metricIDMin := uint64(time.Now().UnixNano())
	for i := range uint64(numMetricIDs) {
		writeCh <- metricIDMin + i
	}
	close(writeCh)
	writeWG.Wait()
	close(readCh)
	readWG.Wait()
}
