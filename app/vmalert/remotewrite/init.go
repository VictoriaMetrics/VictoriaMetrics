package remotewrite

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

var (
	addr = flag.String("remoteWrite.url", "", "Optional URL to Victoria Metrics or VMInsert where to persist alerts state"+
		" and recording rules results in form of timeseries. E.g. http://127.0.0.1:8428")
	basicAuthUsername = flag.String("remoteWrite.basicAuth.username", "", "Optional basic auth username for -remoteWrite.url")
	basicAuthPassword = flag.String("remoteWrite.basicAuth.password", "", "Optional basic auth password for -remoteWrite.url")

	maxQueueSize  = flag.Int("remoteWrite.maxQueueSize", 1e5, "Defines the max number of pending datapoints to remote write endpoint")
	maxBatchSize  = flag.Int("remoteWrite.maxBatchSize", 1e3, "Defines defines max number of timeseries to be flushed at once")
	concurrency   = flag.Int("remoteWrite.concurrency", 1, "Defines number of writers for concurrent writing into remote querier")
	flushInterval = flag.Duration("remoteWrite.flushInterval", 5*time.Second, "Defines interval of flushes to remote write endpoint")

	tlsInsecureSkipVerify = flag.Bool("remoteWrite.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -remoteWrite.url")
	tlsCertFile           = flag.String("remoteWrite.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -remoteWrite.url")
	tlsKeyFile            = flag.String("remoteWrite.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -remoteWrite.url")
	tlsCAFile             = flag.String("remoteWrite.tlsCAFile", "", "Optional path to TLS CA file to use for verifying connections to -remoteWrite.url. "+
		"By default system CA is used")
	tlsServerName = flag.String("remoteWrite.tlsServerName", "", "Optional TLS server name to use for connections to -remoteWrite.url. "+
		"By default the server name from -remoteWrite.url is used")
)

// Init creates Client object from given flags.
// Returns nil if addr flag wasn't set.
func Init(ctx context.Context) (*Client, error) {
	if *addr == "" {
		return nil, nil
	}

	t, err := utils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	return NewClient(ctx, Config{
		Addr:          *addr,
		Concurrency:   *concurrency,
		MaxQueueSize:  *maxQueueSize,
		MaxBatchSize:  *maxBatchSize,
		FlushInterval: *flushInterval,
		BasicAuthUser: *basicAuthUsername,
		BasicAuthPass: *basicAuthPassword,
		Transport:     t,
	})
}
