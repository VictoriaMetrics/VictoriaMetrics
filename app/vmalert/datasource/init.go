package datasource

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

var (
	addr = flag.String("datasource.url", "", "VictoriaMetrics or vmselect url. Required parameter. "+
		"E.g. http://127.0.0.1:8428 . See also -remoteRead.disablePathAppend")
	appendTypePrefix = flag.Bool("datasource.appendTypePrefix", false, "Whether to add type prefix to -datasource.url based on the query type. Set to true if sending different query types to the vmselect URL.")

	headers = flag.String("datasource.headers", "", "Optional HTTP extraHeaders to send with each request to the corresponding -datasource.url. "+
		"For example, -datasource.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -datasource.url. "+
		"Multiple headers must be delimited by '^^': -datasource.headers='header1:value1^^header2:value2'")

	basicAuthUsername     = flag.String("datasource.basicAuth.username", "", "Optional basic auth username for -datasource.url")
	basicAuthPassword     = flag.String("datasource.basicAuth.password", "", "Optional basic auth password for -datasource.url")
	basicAuthPasswordFile = flag.String("datasource.basicAuth.passwordFile", "", "Optional path to basic auth password to use for -datasource.url")

	bearerToken     = flag.String("datasource.bearerToken", "", "Optional bearer auth token to use for -datasource.url.")
	bearerTokenFile = flag.String("datasource.bearerTokenFile", "", "Optional path to bearer token file to use for -datasource.url.")

	tlsInsecureSkipVerify = flag.Bool("datasource.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -datasource.url")
	tlsCertFile           = flag.String("datasource.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -datasource.url")
	tlsKeyFile            = flag.String("datasource.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -datasource.url")
	tlsCAFile             = flag.String("datasource.tlsCAFile", "", `Optional path to TLS CA file to use for verifying connections to -datasource.url. By default, system CA is used`)
	tlsServerName         = flag.String("datasource.tlsServerName", "", `Optional TLS server name to use for connections to -datasource.url. By default, the server name from -datasource.url is used`)

	oauth2ClientID         = flag.String("datasource.oauth2.clientID", "", "Optional OAuth2 clientID to use for -datasource.url. ")
	oauth2ClientSecret     = flag.String("datasource.oauth2.clientSecret", "", "Optional OAuth2 clientSecret to use for -datasource.url.")
	oauth2ClientSecretFile = flag.String("datasource.oauth2.clientSecretFile", "", "Optional OAuth2 clientSecretFile to use for -datasource.url. ")
	oauth2TokenURL         = flag.String("datasource.oauth2.tokenUrl", "", "Optional OAuth2 tokenURL to use for -datasource.url.")
	oauth2Scopes           = flag.String("datasource.oauth2.scopes", "", "Optional OAuth2 scopes to use for -datasource.url. Scopes must be delimited by ';'")

	lookBack  = flag.Duration("datasource.lookback", 0, `Lookback defines how far into the past to look when evaluating queries. For example, if the datasource.lookback=5m then param "time" with value now()-5m will be added to every query.`)
	queryStep = flag.Duration("datasource.queryStep", 0, "queryStep defines how far a value can fallback to when evaluating queries. "+
		"For example, if datasource.queryStep=15s then param \"step\" with value \"15s\" will be added to every query."+
		"If queryStep isn't specified, rule's evaluationInterval will be used instead.")
	queryTimeAlignment = flag.Bool("datasource.queryTimeAlignment", true, `Whether to align "time" parameter with evaluation interval.`+
		"Alignment supposed to produce deterministic results despite of number of vmalert replicas or time they were started. See more details here https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1257")
	maxIdleConnections = flag.Int("datasource.maxIdleConnections", 100, `Defines the number of idle (keep-alive connections) to each configured datasource. Consider setting this value equal to the value: groups_total * group.concurrency. Too low a value may result in a high number of sockets in TIME_WAIT state.`)
	disableKeepAlive   = flag.Bool("datasource.disableKeepAlive", false, `Whether to disable long-lived connections to the datasource. `+
		`If true, disables HTTP keep-alives and will only use the connection to the server for a single HTTP request.`)
	roundDigits = flag.Int("datasource.roundDigits", 0, `Adds "round_digits" GET param to datasource requests. `+
		`In VM "round_digits" limits the number of digits after the decimal point in response values.`)
)

// Param represents an HTTP GET param
type Param struct {
	Key, Value string
}

// Init creates a Querier from provided flag values.
// Provided extraParams will be added as GET params for
// each request.
func Init(extraParams url.Values) (QuerierBuilder, error) {
	if *addr == "" {
		return nil, fmt.Errorf("datasource.url is empty")
	}

	tr, err := utils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	tr.DisableKeepAlives = *disableKeepAlive
	tr.MaxIdleConnsPerHost = *maxIdleConnections
	if tr.MaxIdleConns != 0 && tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
		tr.MaxIdleConns = tr.MaxIdleConnsPerHost
	}

	if extraParams == nil {
		extraParams = url.Values{}
	}
	if *roundDigits > 0 {
		extraParams.Set("round_digits", fmt.Sprintf("%d", *roundDigits))
	}

	authCfg, err := utils.AuthConfig(
		utils.WithBasicAuth(*basicAuthUsername, *basicAuthPassword, *basicAuthPasswordFile),
		utils.WithBearer(*bearerToken, *bearerTokenFile),
		utils.WithOAuth(*oauth2ClientID, *oauth2ClientSecret, *oauth2ClientSecretFile, *oauth2TokenURL, *oauth2Scopes),
		utils.WithHeaders(*headers))
	if err != nil {
		return nil, fmt.Errorf("failed to configure auth: %w", err)
	}

	return &VMStorage{
		c:                &http.Client{Transport: tr},
		authCfg:          authCfg,
		datasourceURL:    strings.TrimSuffix(*addr, "/"),
		appendTypePrefix: *appendTypePrefix,
		lookBack:         *lookBack,
		queryStep:        *queryStep,
		dataSourceType:   datasourcePrometheus,
		extraParams:      extraParams,
	}, nil
}
