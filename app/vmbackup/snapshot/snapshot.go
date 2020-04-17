package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
)

type snapshot struct {
	Status   string `json:"status"`
	Snapshot string `json:"snapshot"`
	Msg      string `json:"msg"`
}

// TakeSnapshot creates a snapshot and the provided api endpoint and returns
// the snapshot name
func TakeSnapshot(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, "/snapshot/create")
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
		return snap.Snapshot, nil
	} else if snap.Status == "error" {
		return "", errors.New(snap.Msg)
	} else {
		return "", fmt.Errorf("Unkown status: %v", snap.Status)
	}
}

// DeleteSnapshot deletes a snapshot and the provided api endpoint returns any failure
func DeleteSnapshot(endpoint string, snapshotName string) error {
	formData := url.Values{
		"snapshot": {snapshotName},
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, "/snapshot/delete")
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
		return nil
	} else if snap.Status == "error" {
		return errors.New(snap.Msg)
	} else {
		return fmt.Errorf("Unkown status: %v", snap.Status)
	}
}
