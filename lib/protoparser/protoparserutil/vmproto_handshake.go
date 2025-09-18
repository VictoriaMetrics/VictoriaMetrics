package protoparserutil

import (
	"io"
	"net/http"
)

// HandleVMProtoServerHandshake returns true if r contains handshake request for determining the supported protocol version.
//
// Deprecated: New vmagent versions skip the handshake and use the VictoriaMetrics
// remote write protocol by default.
// See: https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8462
func HandleVMProtoServerHandshake(w http.ResponseWriter, r *http.Request) bool {
	q := r.URL.Query()
	if q.Get("get_vm_proto_version") != "" {
		_, _ = io.WriteString(w, "1")
		return true
	}
	return false
}
