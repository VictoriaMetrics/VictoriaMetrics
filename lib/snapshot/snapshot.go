package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var snapshotNameRegexp = regexp.MustCompile(`^[0-9]{14}-[0-9A-Fa-f]+$`)

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
	resp, err := http.Get(u.String())
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
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
	resp, err := http.PostForm(u.String(), formData)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
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

// Validate validates the snapshotName
func Validate(snapshotName string) error {
	_, err := Time(snapshotName)
	return err
}

// Time returns snapshot creation time from the given snapshotName
func Time(snapshotName string) (time.Time, error) {
	if !snapshotNameRegexp.MatchString(snapshotName) {
		return time.Time{}, fmt.Errorf("unexpected snapshot name=%q; it must match %q regexp", snapshotName, snapshotNameRegexp.String())
	}
	n := strings.IndexByte(snapshotName, '-')
	if n < 0 {
		logger.Panicf("BUG: cannot find `-` in snapshotName=%q", snapshotName)
	}
	s := snapshotName[:n]
	t, err := time.Parse("20060102150405", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("unexpected timestamp=%q in snapshot name: %w; it must match YYYYMMDDhhmmss pattern", s, err)
	}
	return t, nil
}

// NewName returns new name for new snapshot
func NewName() string {
	return fmt.Sprintf("%s-%08X", time.Now().UTC().Format("20060102150405"), nextSnapshotIdx())
}

func nextSnapshotIdx() uint64 {
	return atomic.AddUint64(&snapshotIdx, 1)
}

var snapshotIdx = uint64(time.Now().UnixNano())
