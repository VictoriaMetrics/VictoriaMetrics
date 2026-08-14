package httpserver

import (
	"errors"
	"net/http"
)

// InvalidParamError sets HTTP status code to 400 Bad Request for Prometheus querying APIs when parameters are missing or incorrect,
// see https://prometheus.io/docs/prometheus/latest/querying/api/#format-overview.
func InvalidParamError(err error) *ErrorWithStatusCode {
	return &ErrorWithStatusCode{
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}
}

// SendPrometheusError sends err to w in Prometheus querying API response format,
// and sets HTTP status code to 422 Unprocessable Entity when code is not set,
// see https://prometheus.io/docs/prometheus/latest/querying/api/#format-overview for more details.
func SendPrometheusError(w http.ResponseWriter, r *http.Request, err error) {
	errStr := err.Error()
	logHTTPError(r, errStr)

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
