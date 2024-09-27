package streamaggr

import (
	"fmt"
	"sync"
	"time"
	// "github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// maxAggrState calculates output=max, e.g. the maximum value over input samples.
type maxAggrState struct {
	m sync.Map
}

type maxStateValue struct {
	mu      sync.Mutex
	max     float64
	deleted bool
}

func newMaxAggrState() *maxAggrState {
	return &maxAggrState{}
}

func (as *maxAggrState) pushSamples(samples []pushSample) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			fmt.Printf("wang The entry is missing in the map: %d\n", time.Now().UnixMicro())
			// The entry is missing in the map. Try creating it.
			v = &maxStateValue{
				max: s.value,
			}
			// outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if !loaded {
				fmt.Printf("wang new entry has been successfully created: %d\n", time.Now().UnixMicro())
				// The new entry has been successfully created.
				continue
			}
			// Use the entry created by a concurrent goroutine.
			v = vNew
		}
		fmt.Printf("wang the right way %d\n", time.Now().Unix())
		sv := v.(*maxStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if s.value > sv.max {
				sv.max = s.value
			}
		}
		sv.mu.Unlock()
		if deleted {
			// The entry has been deleted by the concurrent call to flushState
			// Try obtaining and updating the entry again.
			goto again
		}
	}
	length := 0
    as.m.Range(func(_, _ interface{}) bool {
        length++
        return true
    })
	fmt.Printf("wang sync map length %d\n", length)
}

func (as *maxAggrState) flushState(ctx *flushCtx) {
	m := &as.m
	// fmt.Println("Im outside %d", time.Now().Unix())

	m.Range(func(k, v any) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)
		// fmt.Println("wang  I have deleted the entry: %d", time.Now().UnixMicro())

		sv := v.(*maxStateValue)
		sv.mu.Lock()
		max := sv.max
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()

		key := k.(string)
		// fmt.Printf("Im here %d, %v", time.Now().UnixMicro(), max)

		ctx.appendSeries(key, "max", max)
		return true
	})
}
