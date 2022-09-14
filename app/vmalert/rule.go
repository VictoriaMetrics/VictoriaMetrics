package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// Rule represents alerting or recording rule
// that has unique ID, can be Executed and
// updated with other Rule.
type Rule interface {
	// ID returns unique ID that may be used for
	// identifying this Rule among others.
	ID() uint64
	// Exec executes the rule with given context at the given timestamp and limit.
	// returns an err if number of resulting time series exceeds the limit.
	Exec(ctx context.Context, ts time.Time, limit int) ([]prompbmarshal.TimeSeries, error)
	// ExecRange executes the rule on the given time range.
	ExecRange(ctx context.Context, start, end time.Time) ([]prompbmarshal.TimeSeries, error)
	// UpdateWith performs modification of current Rule
	// with fields of the given Rule.
	UpdateWith(Rule) error
	// ToAPI converts Rule into APIRule
	ToAPI() APIRule
	// Close performs the shutdown procedures for rule
	// such as metrics unregister
	Close()
}

var errDuplicate = errors.New("result contains metrics with the same labelset after applying rule labels")

type ruleState struct {
	sync.RWMutex
	entries []ruleStateEntry
	cur     int
}

type ruleStateEntry struct {
	// stores last moment of time rule.Exec was called
	time time.Time
	// stores the timesteamp with which rule.Exec was called
	at time.Time
	// stores the duration of the last rule.Exec call
	duration time.Duration
	// stores last error that happened in Exec func
	// resets on every successful Exec
	// may be used as Health ruleState
	err error
	// stores the number of samples returned during
	// the last evaluation
	samples int
}

const defaultStateEntriesLimit = 20

func newRuleState() *ruleState {
	return &ruleState{
		entries: make([]ruleStateEntry, defaultStateEntriesLimit),
	}
}

func (s *ruleState) getLast() ruleStateEntry {
	s.RLock()
	defer s.RUnlock()
	return s.entries[s.cur]
}

func (s *ruleState) getAll() []ruleStateEntry {
	entries := make([]ruleStateEntry, 0)

	s.RLock()
	defer s.RUnlock()

	cur := s.cur
	for {
		e := s.entries[cur]
		if !e.time.IsZero() || !e.at.IsZero() {
			entries = append(entries, e)
		}
		cur--
		if cur < 0 {
			cur = cap(s.entries) - 1
		}
		if cur == s.cur {
			return entries
		}
	}
}

func (s *ruleState) add(e ruleStateEntry) {
	s.Lock()
	defer s.Unlock()

	s.cur++
	if s.cur > cap(s.entries)-1 {
		s.cur = 0
	}
	s.entries[s.cur] = e
}
