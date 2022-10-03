package main

import (
	"sync"
	"testing"
	"time"
)

func TestRule_state(t *testing.T) {
	state := newRuleState()
	e := state.getLast()
	if !e.at.IsZero() {
		t.Fatalf("expected entry to be zero")
	}

	now := time.Now()
	state.add(ruleStateEntry{at: now})

	e = state.getLast()
	if e.at != now {
		t.Fatalf("expected entry at %v to be equal to %v",
			e.at, now)
	}

	time.Sleep(time.Millisecond)
	now2 := time.Now()
	state.add(ruleStateEntry{at: now2})

	e = state.getLast()
	if e.at != now2 {
		t.Fatalf("expected entry at %v to be equal to %v",
			e.at, now2)
	}

	if len(state.getAll()) != 2 {
		t.Fatalf("expected for state to have 2 entries only; got %d",
			len(state.getAll()),
		)
	}

	var last time.Time
	for i := 0; i < defaultStateEntriesLimit*2; i++ {
		last = time.Now()
		state.add(ruleStateEntry{at: last})
	}

	e = state.getLast()
	if e.at != last {
		t.Fatalf("expected entry at %v to be equal to %v",
			e.at, last)
	}

	if len(state.getAll()) != defaultStateEntriesLimit {
		t.Fatalf("expected for state to have %d entries only; got %d",
			defaultStateEntriesLimit, len(state.getAll()),
		)
	}
}

// TestRule_stateConcurrent supposed to test concurrent
// execution of state updates.
// Should be executed with -race flag
func TestRule_stateConcurrent(t *testing.T) {
	state := newRuleState()

	const workers = 50
	const iterations = 100
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				state.add(ruleStateEntry{at: time.Now()})
				state.getAll()
				state.getLast()
			}
		}()
	}
	wg.Wait()
}
