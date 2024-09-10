package httpserver

import (
	"errors"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// SendPrometheusError sends err to w in Prometheus querying API response format.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#format-overview for more details
func SendPrometheusError(w http.ResponseWriter, r *http.Request, err error) {
	logger.WarnfSkipframes(1, "error in %q: %s", GetRequestURI(r), err)

	w.Header().Set("Content-Type", "application/json")
	statusCode := http.StatusUnprocessableEntity
	var esc *ErrorWithStatusCode
	if errors.As(err, &esc) {
		statusCode = esc.StatusCode
	}
	w.WriteHeader(statusCode)

	var ure *UserReadableError
	if errors.As(err, &ure) {
		err = ure
	}
	WritePrometheusErrorResponse(w, statusCode, err)
}

// UserReadableError is a type of error which supposed to be returned to the user without additional context.
type UserReadableError struct {
	// Err is the error which needs to be returned to the user.
	Err error
}

// Unwrap returns ure.Err.
//
// This is used by standard errors package. See https://golang.org/pkg/errors
func (ure *UserReadableError) Unwrap() error {
	return ure.Err
}

// Error satisfies Error interface
func (ure *UserReadableError) Error() string {
	return ure.Err.Error()
}
