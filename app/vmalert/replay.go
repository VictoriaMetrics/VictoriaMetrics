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
		"The time filter in RFC3339 format to select time series with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'")
	replayTo = flag.String("replay.timeTo", "",
		"The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'")
	replayRulesDelay = flag.Duration("replay.rulesDelay", time.Second,
		"Delay between rules evaluation within the group. Could be important if there are chained rules inside the group "+
			"and processing need to wait for previous rule results to be persisted by remote storage before evaluating the next rule."+
			"Keep it equal or bigger than -remoteWrite.flushInterval.")
	replayMaxDatapoints = flag.Int("replay.maxDatapointsPerQuery", 1e3,
		"Max number of data points expected in one request. It affects the max time range for every `/query_range` request during the replay. The higher the value, the less requests will be made during replay.")
	replayRuleRetryAttempts = flag.Int("replay.ruleRetryAttempts", 5,
		"Defines how many retries to make before giving up on rule if request for it returns an error.")
	disableProgressBar = flag.Bool("replay.disableProgressBar", false, "Whether to disable rendering progress bars during the replay. "+
		"Progress bar rendering might be verbose or break the logs parsing, so it is recommended to be disabled when not used in interactive mode.")
)

func replay(groupsCfg []config.Group, qb datasource.QuerierBuilder, rw remotewrite.RWClient) error {
	if *replayMaxDatapoints < 1 {
		return fmt.Errorf("replay.maxDatapointsPerQuery can't be lower than 1")
	}
	tFrom, err := time.Parse(time.RFC3339, *replayFrom)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %w", *replayFrom, err)
	}
	tTo, err := time.Parse(time.RFC3339, *replayTo)
	if err != nil {
		return fmt.Errorf("failed to parse %q: %w", *replayTo, err)
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
		ng := rule.NewGroup(cfg, qb, *evaluationInterval, labels)
		total += ng.Replay(tFrom, tTo, rw, *replayMaxDatapoints, *replayRuleRetryAttempts, *replayRulesDelay, *disableProgressBar)
	}
	logger.Infof("replay finished! Imported %d samples", total)
	if rw != nil {
		return rw.Close()
	}
	return nil
}
