package snapshot

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	tlsInsecureSkipVerify = flag.Bool("snapshot.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -snapshotCreateURL")
	tlsCertFile           = flag.String("snapshot.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -snapshotCreateURL")
	tlsKeyFile            = flag.String("snapshot.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -snapshotCreateURL")
	tlsCAFile             = flag.String("snapshot.tlsCAFile", "", `Optional path to TLS CA file to use for verifying connections to -snapshotCreateURL. By default, system CA is used`)
	tlsServerName         = flag.String("snapshot.tlsServerName", "", `Optional TLS server name to use for connections to -snapshotCreateURL. By default, the server name from -snapshotCreateURL is used`)
	basicAuthUser         = flagutil.NewPassword("snapshot.basicAuthUsername", `Optional basic auth username to use for connections to -snapshotCreateURL and -snapshot.deleteURL`)
	basicAuthPassword     = flagutil.NewPassword("snapshot.basicAuthPassword", `Optional basic auth password to use for connections to -snapshotCreateURL and -snapshot.deleteURL`)
	snapshotAuthKey       = flagutil.NewPassword("snapshot.authKey", `Optional authKey to be passed in query string for the connections to -snapshotCreateURL and -snapshot.deleteURL`)
)

type snapshot struct {
	Status   string `json:"status"`
	Snapshot string `json:"snapshot"`
	Msg      string `json:"msg"`
}

// Create creates a snapshot via the provided api endpoint and returns the snapshot name
func Create(createSnapshotURL string) (string, error) {
	logger.Infof("Creating snapshot")
	u, err := url.Parse(createSnapshotURL)
	if err != nil {
		return "", err
	}
	cpyURL := *u
	addSnapshotAuthKeyQueryParam(u)
	// create Transport
	tr, err := httputils.Transport(u.String(), *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return "", err
	}
	hc := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", err
	}
	addBasicAuthHeader(req)
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q", cpyURL.Redacted(), resp.StatusCode, http.StatusOK, body)
	}

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return "", fmt.Errorf("cannot parse JSON response from %q: %w; response body: %q", cpyURL.Redacted(), err, body)
	}

	if snap.Status == "ok" {
		logger.Infof("Snapshot %s created", snap.Snapshot)
		return snap.Snapshot, nil
	}
	if snap.Status == "error" {
		return "", errors.New(snap.Msg)
	}
	return "", fmt.Errorf("Unkown status: %v", snap.Status)
}

// Delete deletes a snapshot via the provided api endpoint
func Delete(deleteSnapshotURL string, snapshotName string) error {
	logger.Infof("Deleting snapshot %s", snapshotName)
	formData := url.Values{
		"snapshot": {snapshotName},
	}
	u, err := url.Parse(deleteSnapshotURL)
	if err != nil {
		return err
	}
	cpyURL := *u
	addSnapshotAuthKeyQueryParam(u)
	// create Transport
	tr, err := httputils.Transport(u.String(), *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return err
	}
	hc := &http.Client{Transport: tr}
	req, err := http.NewRequest("POST", u.String(), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	addBasicAuthHeader(req)
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q", cpyURL.Redacted(), resp.StatusCode, http.StatusOK, body)
	}

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return fmt.Errorf("cannot parse JSON response from %q: %w; response body: %q", cpyURL.Redacted(), err, body)
	}

	if snap.Status == "ok" {
		logger.Infof("Snapshot %s deleted", snapshotName)
		return nil
	}
	if snap.Status == "error" {
		return errors.New(snap.Msg)
	}
	return fmt.Errorf("Unkown status: %v", snap.Status)
}

// addBasicAuthHeader adds basic auth header to request if snapshot.basicAuthUsername and snapshot.basicAuthPassword flags are set.
func addBasicAuthHeader(req *http.Request) {
	if basicAuthUser.Get() != "" {
		auth := basicAuthUser.Get() + ":" + basicAuthPassword.Get()
		authHeader := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Set("Authorization", "Basic "+authHeader)
	}
}

// addSnapshotAuthKeyQueryParam adds authKey query string to URL if snapshot.authKey flag is set.
func addSnapshotAuthKeyQueryParam(u *url.URL) {
	if snapshotAuthKey.Get() != "" {
		qp := u.Query()
		qp.Add("authKey", snapshotAuthKey.Get())
		u.RawQuery = qp.Encode()
	}
}
