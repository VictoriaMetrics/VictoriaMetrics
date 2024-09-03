package opentelemetry

import (
	"net/http"
)

// RequestHandler processes Opentelemetry insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	switch path {
	case "/api/v1/push":
		handleInsert(r, w)
		return true
	default:
		return false
	}
}

func handleInsert(r *http.Request, w http.ResponseWriter) {
	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "application/json":
		handleJSON(r, w)
	default:
		handleProtobuf(r, w)
	}
}
