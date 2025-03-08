package protoparserutil

import (
	"io"
	"net/http"
)

// HandleVMProtoServerHandshake returns true if r contains handshake request for determining the supported protocol version.
func HandleVMProtoServerHandshake(w http.ResponseWriter, r *http.Request) bool {
	q := r.URL.Query()
	if q.Get("get_vm_proto_version") != "" {
		_, _ = io.WriteString(w, "1")
		return true
	}
	return false
}
