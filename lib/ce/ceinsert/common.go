package ceinsert

import "github.com/VictoriaMetrics/metrics"

var timeseriesInsertedTotal = metrics.NewCounter("vm_ce_timeseries_inserted_total")
