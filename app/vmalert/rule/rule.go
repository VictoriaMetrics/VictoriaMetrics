package rule

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// Rule represents alerting or recording rule
// that has unique ID, can be Executed and
// updated with other Rule.
type Rule interface {
	// ID returns unique ID that may be used for
	// identifying this Rule among others.
	ID() uint64
	// exec executes the rule with given context at the given timestamp and limit.
	// returns an err if number of resulting time series exceeds the limit.
	exec(ctx context.Context, ts time.Time, limit int) ([]prompbmarshal.TimeSeries, error)
	// execRange executes the rule on the given time range.
	execRange(ctx context.Context, start, end time.Time) ([]prompbmarshal.TimeSeries, error)
	// updateWith performs modification of current Rule
	// with fields of the given Rule.
	updateWith(Rule) error
	// close performs the shutdown procedures for rule
	// such as metrics unregister
	close()
}

var errDuplicate = errors.New("result contains metrics with the same labelset during evaluation. See https://docs.victoriametrics.com/vmalert/#series-with-the-same-labelset for details")

type ruleState struct {
	sync.RWMutex
	entries []StateEntry
	cur     int
}

// StateEntry stores rule's execution states
type StateEntry struct {
	// stores last moment of time rule.Exec was called
	Time time.Time `json:"time"`
	// stores the timesteamp with which rule.Exec was called
	At time.Time `json:"at"`
	// stores the duration of the last rule.Exec call
	Duration time.Duration `json:"duration"`
	// stores last error that happened in Exec func
	// resets on every successful Exec
	// may be used as Health ruleState
	Err error `json:"error"`
	// stores the number of samples returned during
	// the last evaluation
	Samples int `json:"samples"`
	// stores the number of time series fetched during
	// the last evaluation.
	// Is supported by VictoriaMetrics only, starting from v1.90.0
	// If seriesFetched == nil, then this attribute was missing in
	// datasource response (unsupported).
	SeriesFetched *int `json:"series_fetched"`
	// stores the curl command reflecting the HTTP request used during rule.Exec
	Curl string `json:"curl"`
}

// GetLastEntry returns latest stateEntry of rule
func GetLastEntry(r Rule) StateEntry {
	if rule, ok := r.(*AlertingRule); ok {
		return rule.state.getLast()
	}
	if rule, ok := r.(*RecordingRule); ok {
		return rule.state.getLast()
	}
	return StateEntry{}
}

// GetRuleStateSize returns size of rule stateEntry
func GetRuleStateSize(r Rule) int {
	if rule, ok := r.(*AlertingRule); ok {
		return rule.state.size()
	}
	if rule, ok := r.(*RecordingRule); ok {
		return rule.state.size()
	}
	return 0
}

// GetAllRuleState returns rule entire stateEntries
func GetAllRuleState(r Rule) []StateEntry {
	if rule, ok := r.(*AlertingRule); ok {
		return rule.state.getAll()
	}
	if rule, ok := r.(*RecordingRule); ok {
		return rule.state.getAll()
	}
	return []StateEntry{}
}

func (s *ruleState) size() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.entries)
}

func (s *ruleState) getLast() StateEntry {
	s.RLock()
	defer s.RUnlock()
	if len(s.entries) == 0 {
		return StateEntry{}
	}
	return s.entries[s.cur]
}

func (s *ruleState) getAll() []StateEntry {
	entries := make([]StateEntry, 0)

	s.RLock()
	defer s.RUnlock()

	cur := s.cur
	for {
		e := s.entries[cur]
		if !e.Time.IsZero() || !e.At.IsZero() {
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

func (s *ruleState) add(e StateEntry) {
	s.Lock()
	defer s.Unlock()

	s.cur++
	if s.cur > cap(s.entries)-1 {
		s.cur = 0
	}
	s.entries[s.cur] = e
}

func replayRule(r Rule, start, end time.Time, rw remotewrite.RWClient, replayRuleRetryAttempts int) (int, error) {
	var err error
	var tss []prompbmarshal.TimeSeries
	for i := 0; i < replayRuleRetryAttempts; i++ {
		tss, err = r.execRange(context.Background(), start, end)
		if err == nil {
			break
		}
		logger.Errorf("attempt %d to execute rule %q failed: %s", i+1, r, err)
		time.Sleep(time.Second)
	}
	if err != nil { // means all attempts failed
		return 0, err
	}
	if len(tss) < 1 {
		return 0, nil
	}
	var n int
	for _, ts := range tss {
		if err := rw.Push(ts); err != nil {
			return n, fmt.Errorf("remote write failure: %w", err)
		}
		n += len(ts.Samples)
	}
	return n, nil
}
