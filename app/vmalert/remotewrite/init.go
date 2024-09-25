package remotewrite

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

var (
	addr = flag.String("remoteWrite.url", "", "Optional URL to VictoriaMetrics or vminsert where to persist alerts state "+
		"and recording rules results in form of timeseries. "+
		"Supports address in the form of IP address with a port (e.g., http://127.0.0.1:8428) or DNS SRV record. "+
		"For example, if -remoteWrite.url=http://127.0.0.1:8428 is specified, "+
		"then the alerts state will be written to http://127.0.0.1:8428/api/v1/write . See also -remoteWrite.disablePathAppend, '-remoteWrite.showURL'.")
	showRemoteWriteURL = flag.Bool("remoteWrite.showURL", false, "Whether to show -remoteWrite.url in the exported metrics. "+
		"It is hidden by default, since it can contain sensitive info such as auth key")

	headers = flag.String("remoteWrite.headers", "", "Optional HTTP headers to send with each request to the corresponding -remoteWrite.url. "+
		"For example, -remoteWrite.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -remoteWrite.url. "+
		"Multiple headers must be delimited by '^^': -remoteWrite.headers='header1:value1^^header2:value2'")

	basicAuthUsername     = flag.String("remoteWrite.basicAuth.username", "", "Optional basic auth username for -remoteWrite.url")
	basicAuthPassword     = flag.String("remoteWrite.basicAuth.password", "", "Optional basic auth password for -remoteWrite.url")
	basicAuthPasswordFile = flag.String("remoteWrite.basicAuth.passwordFile", "", "Optional path to basic auth password to use for -remoteWrite.url")

	bearerToken     = flag.String("remoteWrite.bearerToken", "", "Optional bearer auth token to use for -remoteWrite.url.")
	bearerTokenFile = flag.String("remoteWrite.bearerTokenFile", "", "Optional path to bearer token file to use for -remoteWrite.url.")

	idleConnectionTimeout = flag.Duration("remoteWrite.idleConnTimeout", 50*time.Second, `Defines a duration for idle (keep-alive connections) to exist. Consider settings this value less to the value of "-http.idleConnTimeout". It must prevent possible "write: broken pipe" and "read: connection reset by peer" errors.`)

	maxQueueSize  = flag.Int("remoteWrite.maxQueueSize", 1e6, "Defines the max number of pending datapoints to remote write endpoint")
	maxBatchSize  = flag.Int("remoteWrite.maxBatchSize", 1e4, "Defines max number of timeseries to be flushed at once")
	concurrency   = flag.Int("remoteWrite.concurrency", 4, "Defines number of writers for concurrent writing into remote write endpoint")
	flushInterval = flag.Duration("remoteWrite.flushInterval", 5*time.Second, "Defines interval of flushes to remote write endpoint")

	tlsInsecureSkipVerify = flag.Bool("remoteWrite.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -remoteWrite.url")
	tlsCertFile           = flag.String("remoteWrite.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -remoteWrite.url")
	tlsKeyFile            = flag.String("remoteWrite.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -remoteWrite.url")
	tlsCAFile             = flag.String("remoteWrite.tlsCAFile", "", "Optional path to TLS CA file to use for verifying connections to -remoteWrite.url. "+
		"By default, system CA is used")
	tlsServerName = flag.String("remoteWrite.tlsServerName", "", "Optional TLS server name to use for connections to -remoteWrite.url. "+
		"By default, the server name from -remoteWrite.url is used")

	oauth2ClientID         = flag.String("remoteWrite.oauth2.clientID", "", "Optional OAuth2 clientID to use for -remoteWrite.url")
	oauth2ClientSecret     = flag.String("remoteWrite.oauth2.clientSecret", "", "Optional OAuth2 clientSecret to use for -remoteWrite.url")
	oauth2ClientSecretFile = flag.String("remoteWrite.oauth2.clientSecretFile", "", "Optional OAuth2 clientSecretFile to use for -remoteWrite.url")
	oauth2EndpointParams   = flag.String("remoteWrite.oauth2.endpointParams", "", "Optional OAuth2 endpoint parameters to use for -remoteWrite.url . "+
		`The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}`)
	oauth2TokenURL = flag.String("remoteWrite.oauth2.tokenUrl", "", "Optional OAuth2 tokenURL to use for -notifier.url.")
	oauth2Scopes   = flag.String("remoteWrite.oauth2.scopes", "", "Optional OAuth2 scopes to use for -notifier.url. Scopes must be delimited by ';'.")
)

// InitSecretFlags must be called after flag.Parse and before any logging
func InitSecretFlags() {
	if !*showRemoteWriteURL {
		flagutil.RegisterSecretFlag("remoteWrite.url")
	}
}

// Init creates Client object from given flags.
// Returns nil if addr flag wasn't set.
func Init(ctx context.Context) (*Client, error) {
	if *addr == "" {
		return nil, nil
	}

	t, err := httputils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport for -remoteWrite.url=%q: %w", *addr, err)
	}
	t.IdleConnTimeout = *idleConnectionTimeout
	t.DialContext = netutil.NewStatDialFunc("vmalert_remotewrite")

	endpointParams, err := flagutil.ParseJSONMap(*oauth2EndpointParams)
	if err != nil {
		return nil, fmt.Errorf("cannot parse JSON for -remoteWrite.oauth2.endpointParams=%s: %w", *oauth2EndpointParams, err)
	}
	authCfg, err := utils.AuthConfig(
		utils.WithBasicAuth(*basicAuthUsername, *basicAuthPassword, *basicAuthPasswordFile),
		utils.WithBearer(*bearerToken, *bearerTokenFile),
		utils.WithOAuth(*oauth2ClientID, *oauth2ClientSecret, *oauth2ClientSecretFile, *oauth2TokenURL, *oauth2Scopes, endpointParams),
		utils.WithHeaders(*headers))
	if err != nil {
		return nil, fmt.Errorf("failed to configure auth: %w", err)
	}

	return NewClient(ctx, Config{
		Addr:          *addr,
		AuthCfg:       authCfg,
		Concurrency:   *concurrency,
		MaxQueueSize:  *maxQueueSize,
		MaxBatchSize:  *maxBatchSize,
		FlushInterval: *flushInterval,
		Transport:     t,
	})
}
