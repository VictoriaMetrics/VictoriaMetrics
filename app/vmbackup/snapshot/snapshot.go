package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type snapshot struct {
	Status   string `json:"status"`
	Snapshot string `json:"snapshot"`
	Msg      string `json:"msg"`
}

// Create creates a snapshot and the provided api endpoint and returns
// the snapshot name
func Create(createSnapshotURL string) (string, error) {
	logger.Infof("%s", "Creating snapshot")
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

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return "", err
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

	snap := snapshot{}
	err = json.Unmarshal(body, &snap)
	if err != nil {
		return err
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
