package snapshot

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"

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
	// SnapshotCreateURL is the url flag for which the snapshot is to be created
	SnapshotCreateURL = flagutil.NewSecureURL("snapshot.createURL", "VictoriaMetrics create snapshot url. When this is given a snapshot will automatically be created during backup. "+
		"Example: http://victoriametrics:8428/snapshot/create . There is no need in setting -snapshotName if -snapshot.createURL is set")
	// SnapshotDeleteURL is the url flag for which the snapshot is to be deleted
	SnapshotDeleteURL = flagutil.NewSecureURL("snapshot.deleteURL", "VictoriaMetrics delete snapshot url. Optional. Will be generated from -snapshot.createURL if not provided. "+
		"All created snapshots will be automatically deleted. Example: http://victoriametrics:8428/snapshot/delete")
)

type snapshot struct {
	Status   string `json:"status"`
	Snapshot string `json:"snapshot"`
	Msg      string `json:"msg"`
}

// Create creates a snapshot via the provided api endpoint and returns the snapshot name
func Create() (string, error) {
	logger.Infof("Creating snapshot")
	u, err := url.Parse(SnapshotCreateURL.Get())
	if err != nil {
		return "", err
	}

	// create Transport
	tr, err := httputils.Transport(SnapshotCreateURL.Get(), *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
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
		return "", fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q", SnapshotCreateURL.String(), resp.StatusCode, http.StatusOK, body)
	}

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return "", fmt.Errorf("cannot parse JSON response from %q: %w; response body: %q", SnapshotCreateURL.String(), err, body)
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
func Delete(snapshotName string) error {
	logger.Infof("Deleting snapshot %s", snapshotName)
	formData := url.Values{
		"snapshot": {snapshotName},
	}
	u, err := url.Parse(SnapshotDeleteURL.Get())
	if err != nil {
		return err
	}
	// create Transport
	tr, err := httputils.Transport(SnapshotDeleteURL.Get(), *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
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
		return fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q", SnapshotDeleteURL.String(), resp.StatusCode, http.StatusOK, body)
	}

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return fmt.Errorf("cannot parse JSON response from %q: %w; response body: %q", SnapshotDeleteURL.String(), err, body)
	}

	if snap.Status == "ok" {
		logger.Infof("Snapshot %s deleted", snapshotName)
		return nil
	}
	if snap.Status == "error" {
		return errors.New(snap.Msg)
	}
	return fmt.Errorf("Unknown status: %v", snap.Status)
}
