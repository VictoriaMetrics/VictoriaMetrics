package pushmetrics

import (
	"flag"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/appmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	pushURL = flagutil.NewArray("pushmetrics.url", "Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/#push-metrics . "+
		"By default metrics exposed at /metrics page aren't pushed to any remote storage")
	pushInterval    = flag.Duration("pushmetrics.interval", 10*time.Second, "Interval for pushing metrics to -pushmetrics.url")
	pushExtraLabels = flagutil.NewArray("pushmetrics.extraLabels", "Optional labels to add to metrics pushed to -pushmetrics.url . "+
		`For example, -pushmetrics.extraLabels='instance="foo"' adds instance="foo" label to all the metrics pushed to -pushmetrics.url`)
)

func init() {
	// The -pushmetrics.url flag can contain basic auth creds, so it mustn't be visible when exposing the flags.
	flagutil.RegisterSecretFlag("pushmetrics.url")
}

// Init must be called after flag.Parse.
func Init() {
	extraLabels := strings.Join(*pushExtraLabels, ",")
	for _, pu := range *pushURL {
		metrics.InitPushExt(pu, *pushInterval, extraLabels, appmetrics.WritePrometheusMetrics)
	}
}
