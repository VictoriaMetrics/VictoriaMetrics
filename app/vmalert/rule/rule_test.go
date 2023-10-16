package rule

import (
	"sync"
	"testing"
	"time"
)

func TestRule_state(t *testing.T) {
	stateEntriesN := 20
	r := &AlertingRule{state: &ruleState{entries: make([]StateEntry, stateEntriesN)}}
	e := r.state.getLast()
	if !e.At.IsZero() {
		t.Fatalf("expected entry to be zero")
	}

	now := time.Now()
	r.state.add(StateEntry{At: now})

	e = r.state.getLast()
	if e.At != now {
		t.Fatalf("expected entry at %v to be equal to %v",
			e.At, now)
	}

	time.Sleep(time.Millisecond)
	now2 := time.Now()
	r.state.add(StateEntry{At: now2})

	e = r.state.getLast()
	if e.At != now2 {
		t.Fatalf("expected entry at %v to be equal to %v",
			e.At, now2)
	}

	if len(r.state.getAll()) != 2 {
		t.Fatalf("expected for state to have 2 entries only; got %d",
			len(r.state.getAll()),
		)
	}

	var last time.Time
	for i := 0; i < stateEntriesN*2; i++ {
		last = time.Now()
		r.state.add(StateEntry{At: last})
	}

	e = r.state.getLast()
	if e.At != last {
		t.Fatalf("expected entry at %v to be equal to %v",
			e.At, last)
	}

	if len(r.state.getAll()) != stateEntriesN {
		t.Fatalf("expected for state to have %d entries only; got %d",
			stateEntriesN, len(r.state.getAll()),
		)
	}
}

// TestRule_stateConcurrent supposed to test concurrent
// execution of state updates.
// Should be executed with -race flag
func TestRule_stateConcurrent(_ *testing.T) {
	r := &AlertingRule{state: &ruleState{entries: make([]StateEntry, 20)}}
	const workers = 50
	const iterations = 100
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				r.state.add(StateEntry{At: time.Now()})
				r.state.getAll()
				r.state.getLast()
			}
		}()
	}
	wg.Wait()
}
