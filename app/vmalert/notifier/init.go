package notifier

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

var (
	addr                  = flag.String("notifier.url", "", "Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093")
	tlsInsecureSkipVerify = flag.Bool("notifier.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -notifier.url")
	tlsCertFile           = flag.String("notifier.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -notifier.url")
	tlsKeyFile            = flag.String("notifier.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -notifier.url")
	tlsCAFile             = flag.String("notifier.tlsCAFile", "", "Optional path to TLS CA file to use for verifying connections to -notifier.url. "+
		"By default system CA is used")
	tlsServerName = flag.String("notifier.tlsServerName", "", "Optional TLS server name to use for connections to -notifier.url. "+
		"By default the server name from -notifier.url is used")
)

// Init creates a Notifier object based on provided flags.
func Init(gen AlertURLGenerator) (Notifier, error) {
	if *addr == "" {
		flag.PrintDefaults()
		return nil, fmt.Errorf("notifier.url is empty")
	}
	tr, err := utils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %s", err)
	}
	return NewAlertManager(*addr, gen, &http.Client{Transport: tr}), nil
}
