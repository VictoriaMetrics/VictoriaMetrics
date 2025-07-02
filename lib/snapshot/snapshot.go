package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

var (
	tlsInsecureSkipVerify = flag.Bool("snapshot.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -snapshot.createURL")
	tlsCertFile           = flag.String("snapshot.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -snapshot.createURL")
	tlsKeyFile            = flag.String("snapshot.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -snapshot.createURL")
	tlsCAFile             = flag.String("snapshot.tlsCAFile", "", `Optional path to TLS CA file to use for verifying connections to -snapshot.createURL. By default, system CA is used`)
	tlsServerName         = flag.String("snapshot.tlsServerName", "", `Optional TLS server name to use for connections to -snapshot.createURL. By default, the server name from -snapshot.createURL is used`)
)

type snapshot struct {
	Status   string `json:"status"`
	Snapshot string `json:"snapshot"`
	Msg      string `json:"msg"`
}

// Create creates a snapshot via the provided api endpoint and returns the snapshot name
func Create(ctx context.Context, createSnapshotURL string) (string, error) {
	logger.Infof("Creating snapshot")
	u, err := url.Parse(createSnapshotURL)
	if err != nil {
		return "", fmt.Errorf("cannot parse -snapshot.createURL: %w", err)
	}

	hc, err := GetHTTPClient()
	if err != nil {
		return "", fmt.Errorf("cannot create http client for -snapshot.createURL=%q: %w", createSnapshotURL, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createSnapshotURL, nil)
	if err != nil {
		return "", fmt.Errorf("cannot create request for -snapshot.createURL=%q: %w", createSnapshotURL, err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

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
	return "", fmt.Errorf("unknown status: %v", snap.Status)
}

// Delete deletes a snapshot via the provided api endpoint
func Delete(ctx context.Context, deleteSnapshotURL string, snapshotName string) error {
	logger.Infof("Deleting snapshot %s", snapshotName)
	formData := url.Values{
		"snapshot": {snapshotName},
	}
	u, err := url.Parse(deleteSnapshotURL)
	if err != nil {
		return fmt.Errorf("cannot parse -snapshot.deleteURL: %w", err)
	}
	hc, err := GetHTTPClient()
	if err != nil {
		return fmt.Errorf("cannot create http client for -snapshot.deleteURL=%q: %w", deleteSnapshotURL, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deleteSnapshotURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("cannot create request for -snapshot.deleteURL=%q: %w", deleteSnapshotURL, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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
	return fmt.Errorf("unknown status: %v", snap.Status)
}

// GetHTTPClient returns a new HTTP client configured for snapshot operations.
func GetHTTPClient() (*http.Client, error) {
	tr, err := promauth.NewTLSTransport(*tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify, "vm_snapshot_client")
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %s", err)
	}
	hc := &http.Client{
		Transport: tr,
		// Use quite big timeout for snapshots that take too much time.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1571
		Timeout: 5 * time.Minute,
	}
	return hc, nil
}
