package common

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
)

func HandleVMProtoClientHandshake(remoteWriteURL *url.URL) bool {
	u := *remoteWriteURL
	q := u.Query()
	q.Set("get_vm_proto_version", "1")
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
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
		io.WriteString(w, "1")
		return true
	}
	return false
}
