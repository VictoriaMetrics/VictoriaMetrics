package pushmetrics

import (
	"context"
	"flag"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/appmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var (
	pushURL = flagutil.NewArrayString("pushmetrics.url", "Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#push-metrics . "+
		"By default, metrics exposed at /metrics page aren't pushed to any remote storage")
	pushInterval   = flag.Duration("pushmetrics.interval", 10*time.Second, "Interval for pushing metrics to every -pushmetrics.url")
	pushExtraLabel = flagutil.NewArrayString("pushmetrics.extraLabel", "Optional labels to add to metrics pushed to every -pushmetrics.url . "+
		`For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url`)
	pushHeader = flagutil.NewArrayString("pushmetrics.header", "Optional HTTP request header to send to every -pushmetrics.url . "+
		"For example, -pushmetrics.header='Authorization: Basic foobar' adds 'Authorization: Basic foobar' header to every request to every -pushmetrics.url")
	disableCompression = flag.Bool("pushmetrics.disableCompression", false, "Whether to disable request body compression when pushing metrics to every -pushmetrics.url")
)

func init() {
	// The -pushmetrics.url flag can contain basic auth creds, so it mustn't be visible when exposing the flags.
	flagutil.RegisterSecretFlag("pushmetrics.url")
}

var (
	pushCtx, cancelPushCtx = context.WithCancel(context.Background())
	wgDone                 sync.WaitGroup
)

// Init must be called after logger.Init
func Init() {
	extraLabels := strings.Join(*pushExtraLabel, ",")
	for _, pu := range *pushURL {
		opts := &metrics.PushOptions{
			ExtraLabels:        extraLabels,
			Headers:            *pushHeader,
			DisableCompression: *disableCompression,
			WaitGroup:          &wgDone,
		}
		if err := metrics.InitPushExtWithOptions(pushCtx, pu, *pushInterval, appmetrics.WritePrometheusMetrics, opts); err != nil {
			logger.Fatalf("cannot initialize pushmetrics: %s", err)
		}
	}
}

// Stop stops the periodic push of metrics.
// It is important to stop the push of metrics before disposing resources
// these metrics attached to. See related https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5548
//
// Stop must be called after Init.
func Stop() {
	cancelPushCtx()
	wgDone.Wait()
}
