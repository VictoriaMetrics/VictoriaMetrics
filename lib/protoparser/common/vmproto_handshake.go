package common

import (
	"io"
	"net/http"
	"strconv"
	"strings"
)

// HandleVMProtoClientHandshake returns true if the server at remoteWriteURL supports VictoriaMetrics remote write protocol.
func HandleVMProtoClientHandshake(remoteWriteURL string, doRequest func(handshakeURL string) (*http.Response, error)) bool {
	u := remoteWriteURL
	if strings.Contains(u, "?") {
		u += "&"
	} else {
		u += "?"
	}
	u += "get_vm_proto_version=1"
	resp, err := doRequest(u)
	if err != nil {
		return false
	}
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	version, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}
	return version >= 1
}

// HandleVMProtoServerHandshake returns true if r contains handshake request for determining the supported protocol version.
func HandleVMProtoServerHandshake(w http.ResponseWriter, r *http.Request) bool {
	q := r.URL.Query()
	if q.Get("get_vm_proto_version") != "" {
		_, _ = io.WriteString(w, "1")
		return true
	}
	return false
}
