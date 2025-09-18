package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	replayFrom = flag.String("replay.timeFrom", "",
		"The time filter in RFC3339 format to start the replay from. E.g. '2020-01-01T20:07:00Z'")
	replayTo = flag.String("replay.timeTo", "",
		"The time filter in RFC3339 format to finish the replay by. E.g. '2020-01-01T20:07:00Z'. "+
			"By default, is set to the current time.")
	replayRulesDelay = flag.Duration("replay.rulesDelay", time.Second,
		"Delay before evaluating the next rule within the group. Is important for chained rules. "+
			"Keep it equal or bigger than -remoteWrite.flushInterval. When set to >0, replay ignores group's concurrency setting.")
	replayMaxDatapoints = flag.Int("replay.maxDatapointsPerQuery", 1e3,
		"Max number of data points expected in one request. It affects the max time range for every '/query_range' request during the replay. The higher the value, the less requests will be made during replay.")
	replayRuleRetryAttempts = flag.Int("replay.ruleRetryAttempts", 5,
		"Defines how many retries to make before giving up on rule if request for it returns an error.")
	disableProgressBar = flag.Bool("replay.disableProgressBar", false, "Whether to disable rendering progress bars during the replay. "+
		"Progress bar rendering might be verbose or break the logs parsing, so it is recommended to be disabled when not used in interactive mode.")
	ruleEvaluationConcurrency = flag.Int("replay.ruleEvaluationConcurrency", 1, "The maximum number of concurrent '/query_range' requests when replay recording rule or alerting rule with for=0. "+
		"Increasing this value when replaying for a long time, since each request is limited by -replay.maxDatapointsPerQuery.")
)

func replay(groupsCfg []config.Group, qb datasource.QuerierBuilder, rw remotewrite.RWClient) (totalRows, droppedRows int, err error) {
	if *replayMaxDatapoints < 1 {
		return 0, 0, fmt.Errorf("replay.maxDatapointsPerQuery can't be lower than 1")
	}
	tFrom, err := time.Parse(time.RFC3339, *replayFrom)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse replay.timeFrom=%q: %w", *replayFrom, err)
	}

	// use tFrom location for default value, otherwise filters could have different locations
	tTo := time.Now().In(tFrom.Location())
	if *replayTo != "" {
		tTo, err = time.Parse(time.RFC3339, *replayTo)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse replay.timeTo=%q: %w", *replayTo, err)
		}
	}

	if !tTo.After(tFrom) {
		return 0, 0, fmt.Errorf("replay.timeTo=%v must be bigger than replay.timeFrom=%v", tTo, tFrom)
	}
	labels := make(map[string]string)
	for _, s := range *externalLabels {
		if len(s) == 0 {
			continue
		}
		n := strings.IndexByte(s, '=')
		if n < 0 {
			return 0, 0, fmt.Errorf("missing '=' in `-label`. It must contain label in the form `name=value`; got %q", s)
		}
		labels[s[:n]] = s[n+1:]
	}

	fmt.Printf("Replay mode:"+
		"\nfrom: \t%v "+
		"\nto: \t%v "+
		"\nmax data points per request: %d\n",
		tFrom, tTo, *replayMaxDatapoints)

	for _, cfg := range groupsCfg {
		ng := rule.NewGroup(cfg, qb, *evaluationInterval, labels)
		totalRows += ng.Replay(tFrom, tTo, rw, *replayMaxDatapoints, *replayRuleRetryAttempts, *replayRulesDelay, *disableProgressBar, *ruleEvaluationConcurrency)
	}
	logger.Infof("replay evaluation finished, generated %d samples", totalRows)
	if err := rw.Close(); err != nil {
		return 0, 0, err
	}
	droppedRows = remotewrite.GetDroppedRows()
	return totalRows, droppedRows, nil
}
