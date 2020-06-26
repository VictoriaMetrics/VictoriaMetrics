package datasource

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

// Querier interface wraps Query method which
// executes given query and returns list of Metrics
// as result
type Querier interface {
	Query(ctx context.Context, query string) ([]Metric, error)
}

// Metric is the basic entity which should be return by datasource
// It represents single data point with full list of labels
type Metric struct {
	Labels    []Label
	Timestamp int64
	Value     float64
}

// Label represents metric's label
type Label struct {
	Name  string
	Value string
}

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

func Init() (Querier, error) {
	dst, err := utils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("cannot create datasource transport: %s", err)
	}
	c := &http.Client{Transport: dst}
	return NewVMStorage(*addr, *basicAuthUsername, *basicAuthPassword, c), nil
}
