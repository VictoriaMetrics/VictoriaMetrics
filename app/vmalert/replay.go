package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

var (
	replayFrom = flag.String("replay.timeFrom", "",
		"The time filter in RFC3339 format to select time series with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'")
	replayTo = flag.String("replay.timeTo", "",
		"The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'")
	replayRulesDelay = flag.Duration("replay.rulesDelay", time.Second,
		"Delay between rules evaluation within the group. Could be important if there are chained rules inside of the group"+
			"and processing need to wait for previous rule results to be persisted by remote storage before evaluating the next rule."+
			"Keep it equal or bigger than -remoteWrite.flushInterval.")
	replayMaxDatapoints = flag.Int("replay.maxDatapointsPerQuery", 1e3,
		"Max number of data points expected in one request. The higher the value, the less requests will be made during replay.")
	replayRuleRetryAttempts = flag.Int("replay.ruleRetryAttempts", 5,
		"Defines how many retries to make before giving up on rule if request for it returns an error.")
)

func replay(groupsCfg []config.Group, qb datasource.QuerierBuilder, rw *remotewrite.Client) error {
	if *replayMaxDatapoints < 1 {
		return fmt.Errorf("replay.maxDatapointsPerQuery can't be lower than 1")
	}
	tFrom, err := time.Parse(time.RFC3339, *replayFrom)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %s", *replayFrom, err)
	}
	tTo, err := time.Parse(time.RFC3339, *replayTo)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %s", *replayTo, err)
	}
	if !tTo.After(tFrom) {
		return fmt.Errorf("replay.timeTo must be bigger than replay.timeFrom")
	}
	labels := make(map[string]string)
	for _, s := range *externalLabels {
		if len(s) == 0 {
			continue
		}
		n := strings.IndexByte(s, '=')
		if n < 0 {
			return fmt.Errorf("missing '=' in `-label`. It must contain label in the form `name=value`; got %q", s)
		}
		labels[s[:n]] = s[n+1:]
	}

	fmt.Printf("Replay mode:"+
		"\nfrom: \t%v "+
		"\nto: \t%v "+
		"\nmax data points per request: %d\n",
		tFrom, tTo, *replayMaxDatapoints)

	var total int
	for _, cfg := range groupsCfg {
		ng := newGroup(cfg, qb, *evaluationInterval, labels)
		total += ng.replay(tFrom, tTo, rw)
	}
	logger.Infof("replay finished! Imported %d samples", total)
	if rw != nil {
		return rw.Close()
	}
	return nil
}

func (g *Group) replay(start, end time.Time, rw *remotewrite.Client) int {
	var total int
	step := g.Interval * time.Duration(*replayMaxDatapoints)
	ri := rangeIterator{start: start, end: end, step: step}
	iterations := int(end.Sub(start)/step) + 1
	fmt.Printf("\nGroup %q"+
		"\ninterval: \t%v"+
		"\nrequests to make: \t%d"+
		"\nmax range per request: \t%v\n",
		g.Name, g.Interval, iterations, step)
	for _, rule := range g.Rules {
		fmt.Printf("> Rule %q (ID: %d)\n", rule, rule.ID())
		bar := pb.StartNew(iterations)
		ri.reset()
		for ri.next() {
			n, err := replayRule(rule, ri.s, ri.e, rw)
			if err != nil {
				logger.Fatalf("rule %q: %s", rule, err)
			}
			total += n
			bar.Increment()
		}
		bar.Finish()
		// sleep to let remote storage to flush data on-disk
		// so chained rules could be calculated correctly
		time.Sleep(*replayRulesDelay)
	}
	return total
}

func replayRule(rule Rule, start, end time.Time, rw *remotewrite.Client) (int, error) {
	var err error
	var tss []prompbmarshal.TimeSeries
	for i := 0; i < *replayRuleRetryAttempts; i++ {
		tss, err = rule.ExecRange(context.Background(), start, end)
		if err == nil {
			break
		}
		logger.Errorf("attempt %d to execute rule %q failed: %s", i+1, rule, err)
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
			return n, fmt.Errorf("remote write failure: %s", err)
		}
		n += len(ts.Samples)
	}
	return n, nil
}

type rangeIterator struct {
	step       time.Duration
	start, end time.Time

	iter int
	s, e time.Time
}

func (ri *rangeIterator) reset() {
	ri.iter = 0
	ri.s, ri.e = time.Time{}, time.Time{}
}

func (ri *rangeIterator) next() bool {
	ri.s = ri.start.Add(ri.step * time.Duration(ri.iter))
	if !ri.end.After(ri.s) {
		return false
	}
	ri.e = ri.s.Add(ri.step)
	if ri.e.After(ri.end) {
		ri.e = ri.end
	}
	ri.iter++
	return true
}
