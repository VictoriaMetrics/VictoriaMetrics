package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	snapshotNameRegexp = regexp.MustCompile(`^\d{14}-[\dA-Fa-f]+$`)
	snapshotIdx        = uint64(time.Now().UnixNano())
)

type snapshot struct {
	Status   string `json:"status"`
	Snapshot string `json:"snapshot"`
	Msg      string `json:"msg"`
}

// Create creates a snapshot and the provided api endpoint and returns
// the snapshot name
func Create(createSnapshotURL string) (string, error) {
	logger.Infof("Creating snapshot")
	u, err := url.Parse(createSnapshotURL)
	if err != nil {
		return "", err
	}
	resp, err := http.Get(u.String())
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code returned from %q; expecting %d; got %d; response body: %q", createSnapshotURL, resp.StatusCode, http.StatusOK, body)
	}

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return "", fmt.Errorf("cannot parse JSON response from %q: %w; response body: %q", createSnapshotURL, err, body)
	}

	if snap.Status == "ok" {
		logger.Infof("Snapshot %s created", snap.Snapshot)
		return snap.Snapshot, nil
	} else if snap.Status == "error" {
		return "", errors.New(snap.Msg)
	} else {
		return "", fmt.Errorf("Unkown status: %v", snap.Status)
	}
}

// Delete deletes a snapshot and the provided api endpoint returns any failure
func Delete(deleteSnapshotURL string, snapshotName string) error {
	logger.Infof("Deleting snapshot %s", snapshotName)
	formData := url.Values{
		"snapshot": {snapshotName},
	}
	u, err := url.Parse(deleteSnapshotURL)
	if err != nil {
		return err
	}
	resp, err := http.PostForm(u.String(), formData)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code returned from %q; expecting %d; got %d; response body: %q", deleteSnapshotURL, resp.StatusCode, http.StatusOK, body)
	}

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return fmt.Errorf("cannot parse JSON response from %q: %w; response body: %q", deleteSnapshotURL, err, body)
	}

	if snap.Status == "ok" {
		logger.Infof("Snapshot %s deleted", snapshotName)
		return nil
	} else if snap.Status == "error" {
		return errors.New(snap.Msg)
	} else {
		return fmt.Errorf("Unkown status: %v", snap.Status)
	}
}

// Validate check snapshot name for using pattern
func Validate(snapshotName string) bool {
	n := strings.IndexByte(snapshotName, '-')
	if n < 0 {
		return false
	}
	s := snapshotName[:n]
	_, err := time.Parse("20060102150405", s)
	return err == nil && snapshotNameRegexp.MatchString(snapshotName)
}

// Match check does snapshot match using pattern
func Match(snapshotName string) bool {
	return snapshotNameRegexp.MatchString(snapshotName)
}

// Time return snapshot time from snapshot name
func Time(snapshotName string) (time.Time, error) {
	if !snapshotNameRegexp.MatchString(snapshotName) {
		return time.Time{}, fmt.Errorf("unexpected snapshotName must be in the format `YYYYMMDDhhmmss-idx`; got %q", snapshotName)
	}
	n := strings.IndexByte(snapshotName, '-')
	if n < 0 {
		return time.Time{}, fmt.Errorf("cannot find `-` in snapshotName=%q", snapshotName)
	}
	s := snapshotName[:n]
	return time.Parse("20060102150405", s)
}

// NextSnapshotIdx generate next snapshot index
func NextSnapshotIdx() uint64 {
	return atomic.AddUint64(&snapshotIdx, 1)
}
