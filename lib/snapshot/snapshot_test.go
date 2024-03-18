package snapshot

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSnapshot(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/snapshot/create" {
			_, err := io.WriteString(w, `{"status":"ok","snapshot":"mysnapshot"}`)
			if err != nil {
				t.Fatalf("Failed to write response output: %v", err)
			}
		} else {
			t.Fatalf("Invalid path, got %v", r.URL.Path)
		}
	})

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	snapshotName, err := Create(server.URL + "/snapshot/create")
	if err != nil {
		t.Fatalf("Failed taking snapshot: %v", err)
	}

	if snapshotName != "mysnapshot" {
		t.Fatalf("Snapshot name is not correct, got %v", snapshotName)
	}
}

func TestCreateSnapshotWithBasicAuth(t *testing.T) {
	origUsername := basicAuthUser.Get()
	origPassword := basicAuthPassword.Get()
	basicAuthUser.Set("foo")
	basicAuthPassword.Set("bar")
	defer func() {
		if err := basicAuthUser.Set(origUsername); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := basicAuthPassword.Set(origPassword); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			if username != "foo" || password != "bar" {
				w.Header().Set("WWW-Authenticate", `Basic realm="MockServer"`)
				http.Error(w, "", http.StatusUnauthorized)
				return
			}
		}
		if r.URL.Path == "/snapshot/create" {
			_, err := io.WriteString(w, `{"status":"ok","snapshot":"mysnapshot"}`)
			if err != nil {
				t.Fatalf("Failed to write response output: %v", err)
			}
		} else {
			t.Fatalf("Invalid path, got %v", r.URL.Path)
		}
	})
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()
	snapshotName, err := Create(server.URL + "/snapshot/create")
	if err != nil {
		t.Fatalf("Failed taking snapshot: %v", err)
	}
	if snapshotName != "mysnapshot" {
		t.Fatalf("Snapshot name is not correct, got %v", snapshotName)
	}
}

func TestCreateSnapshotWithAuthKey(t *testing.T) {
	origSnapshotAuthKey := snapshotAuthKey.Get()
	snapshotAuthKey.Set("foobar")
	defer func() {
		if err := snapshotAuthKey.Set(origSnapshotAuthKey); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("authKey") != "foobar" {
			http.Error(w, "The provided authKey doesn't match -foobar", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/snapshot/create" {
			_, err := io.WriteString(w, `{"status":"ok","snapshot":"mysnapshot"}`)
			if err != nil {
				t.Fatalf("Failed to write response output: %v", err)
			}
		} else {
			t.Fatalf("Invalid path, got %v", r.URL.Path)
		}
	})
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()
	snapshotName, err := Create(server.URL + "/snapshot/create")
	if err != nil {
		t.Fatalf("Failed taking snapshot: %v", err)
	}
	if snapshotName != "mysnapshot" {
		t.Fatalf("Snapshot name is not correct, got %v", snapshotName)
	}
}

func TestCreateSnapshotFailed(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/snapshot/create" {
			_, err := io.WriteString(w, `{"status":"error","msg":"I am unwell"}`)
			if err != nil {
				t.Fatalf("Failed to write response output: %v", err)
			}
		} else {
			t.Fatalf("Invalid path, got %v", r.URL.Path)
		}
	})

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	snapshotName, err := Create(server.URL + "/snapshot/create")
	if err == nil {
		t.Fatalf("Snapshot did not fail, got snapshot: %v", snapshotName)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	snapshotName := "mysnapshot"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/snapshot/delete" {
			_, err := io.WriteString(w, `{"status":"ok"}`)
			if err != nil {
				t.Fatalf("Failed to write response output: %v", err)
			}
		} else {
			t.Fatalf("Invalid path, got %v", r.URL.Path)
		}
		if r.FormValue("snapshot") != snapshotName {
			t.Fatalf("Invalid snapshot name, got %v", snapshotName)
		}
	})

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	err := Delete(server.URL+"/snapshot/delete", snapshotName)
	if err != nil {
		t.Fatalf("Failed to delete snapshot: %v", err)
	}
}

func TestDeleteSnapshotWithBasicAuth(t *testing.T) {
	origUsername := basicAuthUser.Get()
	origPassword := basicAuthPassword.Get()
	basicAuthUser.Set("foo")
	basicAuthPassword.Set("bar")
	defer func() {
		if err := basicAuthUser.Set(origUsername); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := basicAuthPassword.Set(origPassword); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}()
	snapshotName := "mysnapshot"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			if username != "foo" || password != "bar" {
				w.Header().Set("WWW-Authenticate", `Basic realm="MockServer"`)
				http.Error(w, "", http.StatusUnauthorized)
				return
			}
		}
		if r.URL.Path == "/snapshot/delete" {
			_, err := io.WriteString(w, `{"status":"ok"}`)
			if err != nil {
				t.Fatalf("Failed to write response output: %v", err)
			}
		} else {
			t.Fatalf("Invalid path, got %v", r.URL.Path)
		}
		if r.FormValue("snapshot") != snapshotName {
			t.Fatalf("Invalid snapshot name, got %v", snapshotName)
		}
	})
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()
	err := Delete(server.URL+"/snapshot/delete", snapshotName)
	if err != nil {
		t.Fatalf("Failed to delete snapshot: %v", err)
	}
}

func TestDeleteSnapshotWithAuthKey(t *testing.T) {
	origSnapshotAuthKey := snapshotAuthKey.Get()
	snapshotAuthKey.Set("foobar")
	defer func() {
		if err := snapshotAuthKey.Set(origSnapshotAuthKey); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}()
	snapshotName := "mysnapshot"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("authKey") != "foobar" {
			http.Error(w, "The provided authKey doesn't match -foobar", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/snapshot/delete" {
			_, err := io.WriteString(w, `{"status":"ok"}`)
			if err != nil {
				t.Fatalf("Failed to write response output: %v", err)
			}
		} else {
			t.Fatalf("Invalid path, got %v", r.URL.Path)
		}
		if r.FormValue("snapshot") != snapshotName {
			t.Fatalf("Invalid snapshot name, got %v", snapshotName)
		}
	})
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()
	err := Delete(server.URL+"/snapshot/delete", snapshotName)
	if err != nil {
		t.Fatalf("Failed to delete snapshot: %v", err)
	}
}

func TestDeleteSnapshotFailed(t *testing.T) {
	snapshotName := "mysnapshot"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/snapshot/delete" {
			_, err := io.WriteString(w, `{"status":"error", "msg":"failed to delete"}`)
			if err != nil {
				t.Fatalf("Failed to write response output: %v", err)
			}
		} else {
			t.Fatalf("Invalid path, got %v", r.URL.Path)
		}
		if r.FormValue("snapshot") != snapshotName {
			t.Fatalf("Invalid snapshot name, got %v", snapshotName)
		}
	})

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	err := Delete(server.URL+"/snapshot/delete", snapshotName)
	if err == nil {
		t.Fatalf("Snapshot should have failed, got: %v", err)
	}
}
