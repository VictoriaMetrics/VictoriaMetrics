package remotewrite

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// RWClient represents an HTTP client for pushing data via remote write protocol
type RWClient interface {
	// Push pushes the give time series to remote storage
	Push(s prompb.TimeSeries) error
	// Close stops the client. Client can't be reused after Close call.
	Close() error
}
