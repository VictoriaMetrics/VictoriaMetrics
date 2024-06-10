package snapshot

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	tlsInsecureSkipVerify = flag.Bool("snapshot.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -snapshotCreateURL")
	tlsCertFile           = flag.String("snapshot.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -snapshotCreateURL")
	tlsKeyFile            = flag.String("snapshot.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -snapshotCreateURL")
	tlsCAFile             = flag.String("snapshot.tlsCAFile", "", `Optional path to TLS CA file to use for verifying connections to -snapshotCreateURL. By default, system CA is used`)
	tlsServerName         = flag.String("snapshot.tlsServerName", "", `Optional TLS server name to use for connections to -snapshotCreateURL. By default, the server name from -snapshotCreateURL is used`)
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

	// create Transport
	tr, err := httputils.Transport(createSnapshotURL, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return "", err
	}
	hc := &http.Client{Transport: tr}

	resp, err := hc.Get(u.String())
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q", u.Redacted(), resp.StatusCode, http.StatusOK, body)
	}

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return "", fmt.Errorf("cannot parse JSON response from %q: %w; response body: %q", u.Redacted(), err, body)
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
	// create Transport
	tr, err := httputils.Transport(deleteSnapshotURL, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return err
	}
	hc := &http.Client{Transport: tr}
	resp, err := hc.PostForm(u.String(), formData)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q", u.Redacted(), resp.StatusCode, http.StatusOK, body)
	}

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return fmt.Errorf("cannot parse JSON response from %q: %w; response body: %q", u.Redacted(), err, body)
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
