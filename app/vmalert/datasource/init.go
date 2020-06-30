package datasource

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

var (
	addr = flag.String("datasource.url", "", "Victoria Metrics or VMSelect url. Required parameter."+
		" E.g. http://127.0.0.1:8428")
	basicAuthUsername = flag.String("datasource.basicAuth.username", "", "Optional basic auth username for -datasource.url")
	basicAuthPassword = flag.String("datasource.basicAuth.password", "", "Optional basic auth password for -datasource.url")

	tlsInsecureSkipVerify = flag.Bool("datasource.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -datasource.url")
	tlsCertFile           = flag.String("datasource.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -datasource.url")
	tlsKeyFile            = flag.String("datasource.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -datasource.url")
	tlsCAFile             = flag.String("datasource.tlsCAFile", "", "Optional path to TLS CA file to use for verifying connections to -datasource.url. "+
		"By default system CA is used")
	tlsServerName = flag.String("datasource.tlsServerName", "", "Optional TLS server name to use for connections to -datasource.url. "+
		"By default the server name from -datasource.url is used")
)

// Init creates a Querier from provided flag values.
func Init() (Querier, error) {
	if *addr == "" {
		flag.PrintDefaults()
		return nil, fmt.Errorf("datasource.url is empty")
	}
	tr, err := utils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	c := &http.Client{Transport: tr}
	return NewVMStorage(*addr, *basicAuthUsername, *basicAuthPassword, c), nil
}
