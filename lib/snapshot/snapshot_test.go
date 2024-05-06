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

func TestCreateSnapshotWithAuthCredentials(t *testing.T) {
	tests := map[string]struct {
		mockServerUsername        string
		mockServerPassword        string
		mockServerSnapshotAuthKey string
		createURLEndpoint         string
		username                  string
		password                  string
		snapshotAuthKey           string
		wantError                 bool
	}{
		"creation of snapshot with basic auth": {
			mockServerUsername:        "foo",
			mockServerPassword:        "bar",
			mockServerSnapshotAuthKey: "",
			createURLEndpoint:         "/snapshot/create",
			username:                  "foo",
			password:                  "bar",
			snapshotAuthKey:           "",
			wantError:                 false},
		"creation of snapshot with wrong password": {
			mockServerUsername:        "foo",
			mockServerPassword:        "bar",
			mockServerSnapshotAuthKey: "",
			createURLEndpoint:         "/snapshot/create",
			username:                  "foo",
			password:                  "wrongPass",
			snapshotAuthKey:           "",
			wantError:                 true},
		"creation of snapshot with snapshot authkey": {
			mockServerUsername:        "",
			mockServerPassword:        "",
			mockServerSnapshotAuthKey: "bas",
			createURLEndpoint:         "/snapshot/create",
			username:                  "",
			password:                  "",
			snapshotAuthKey:           "bas",
			wantError:                 false},
		"creation of snapshot with wrong snapshot authkey": {
			mockServerUsername:        "",
			mockServerPassword:        "",
			mockServerSnapshotAuthKey: "bas",
			createURLEndpoint:         "/snapshot/create",
			username:                  "",
			password:                  "",
			snapshotAuthKey:           "wrongKey",
			wantError:                 true},
		"creation of snapshot with snapshot authkey provided via query param": {
			mockServerUsername:        "",
			mockServerPassword:        "",
			mockServerSnapshotAuthKey: "bas",
			createURLEndpoint:         "/snapshot/create?authKey=bas",
			username:                  "",
			password:                  "",
			snapshotAuthKey:           "",
			wantError:                 false},
		"creation of snapshot with bith basic auth and snapshot authkey": {
			mockServerUsername:        "foo",
			mockServerPassword:        "bar",
			mockServerSnapshotAuthKey: "bas",
			createURLEndpoint:         "/snapshot/create",
			username:                  "foo",
			password:                  "bar",
			snapshotAuthKey:           "bas",
			wantError:                 false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			origUsername := basicAuthUser.Get()
			origPassword := basicAuthPassword.Get()
			origSnapshotAuthKey := snapshotAuthKey.Get()
			// reset the flags after tests
			defer func() {
				if err := basicAuthUser.Set(origUsername); err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if err := basicAuthPassword.Set(origPassword); err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if err := snapshotAuthKey.Set(origSnapshotAuthKey); err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}()
			// init flags with test values
			if err := basicAuthUser.Set(test.username); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if err := basicAuthPassword.Set(test.password); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if err := snapshotAuthKey.Set(test.snapshotAuthKey); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/snapshot/create" {
					// validate auth credentials
					ok := checkAuthCredentials(w, r, test.mockServerUsername, test.mockServerPassword, test.mockServerSnapshotAuthKey)
					if !ok {
						return
					}
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
			snapshotName, err := Create(server.URL + test.createURLEndpoint)
			if !test.wantError && err != nil {
				t.Fatalf("expected no error, but got %v", err)
			}
			if !test.wantError && snapshotName != "mysnapshot" {
				t.Fatalf("Snapshot name is not correct, got %v", snapshotName)
			}
		})
	}
}

func TestDeleteSnapshotWithAuthCredentials(t *testing.T) {
	tests := map[string]struct {
		mockServerUsername        string
		mockServerPassword        string
		mockServerSnapshotAuthKey string
		deleteURLEndpoint         string
		username                  string
		password                  string
		snapshotAuthKey           string
		wantError                 bool
	}{
		"deletion of snapshot with basic auth": {
			mockServerUsername:        "foo",
			mockServerPassword:        "bar",
			mockServerSnapshotAuthKey: "",
			deleteURLEndpoint:         "/snapshot/delete",
			username:                  "foo",
			password:                  "bar",
			snapshotAuthKey:           "",
			wantError:                 false},
		"deletion of snapshot with wrong password": {
			mockServerUsername:        "foo",
			mockServerPassword:        "bar",
			mockServerSnapshotAuthKey: "",
			deleteURLEndpoint:         "/snapshot/delete",
			username:                  "foo",
			password:                  "wrongPass",
			snapshotAuthKey:           "",
			wantError:                 true},
		"deletion of snapshot with snapshot authkey": {
			mockServerUsername:        "",
			mockServerPassword:        "",
			mockServerSnapshotAuthKey: "bas",
			deleteURLEndpoint:         "/snapshot/delete",
			username:                  "",
			password:                  "",
			snapshotAuthKey:           "bas",
			wantError:                 false},
		"deletion of snapshot with wrong snapshot authkey": {
			mockServerUsername:        "",
			mockServerPassword:        "",
			mockServerSnapshotAuthKey: "bas",
			deleteURLEndpoint:         "/snapshot/delete",
			username:                  "",
			password:                  "",
			snapshotAuthKey:           "wrongKey",
			wantError:                 true},
		"deletion of snapshot with snapshot authkey provided via query param": {
			mockServerUsername:        "",
			mockServerPassword:        "",
			mockServerSnapshotAuthKey: "bas",
			deleteURLEndpoint:         "/snapshot/delete?authKey=bas",
			username:                  "",
			password:                  "",
			snapshotAuthKey:           "",
			wantError:                 false},
		"deletion of snapshot with bith basic auth and snapshot authkey": {
			mockServerUsername:        "foo",
			mockServerPassword:        "bar",
			mockServerSnapshotAuthKey: "bas",
			deleteURLEndpoint:         "/snapshot/delete",
			username:                  "foo",
			password:                  "bar",
			snapshotAuthKey:           "bas",
			wantError:                 false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			origUsername := basicAuthUser.Get()
			origPassword := basicAuthPassword.Get()
			origSnapshotAuthKey := snapshotAuthKey.Get()
			// reset the flags after tests
			defer func() {
				if err := basicAuthUser.Set(origUsername); err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if err := basicAuthPassword.Set(origPassword); err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if err := snapshotAuthKey.Set(origSnapshotAuthKey); err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}()
			// init flags with test values
			if err := basicAuthUser.Set(test.username); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if err := basicAuthPassword.Set(test.password); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if err := snapshotAuthKey.Set(test.snapshotAuthKey); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			snapshotName := "mysnapshot"

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/snapshot/delete" {
					checkAuthCredentials(w, r, test.mockServerUsername, test.mockServerPassword, test.mockServerSnapshotAuthKey)
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

			err := Delete(server.URL+test.deleteURLEndpoint, snapshotName)
			if !test.wantError && err != nil {
				t.Fatalf("expected no error, but got %v", err)
			}
		})
	}
}

func checkAuthCredentials(w http.ResponseWriter, r *http.Request, username string, password string, authkey string) bool {
	if authkey != "" {
		if r.FormValue("authKey") != authkey {
			if r.Header.Get("X-AuthKey") != authkey {
				http.Error(w, "The provided authKey doesn't match -snapshotAuthKey", http.StatusUnauthorized)
				return false
			}
		}
	}
	if username != "" {
		uname, pass, _ := r.BasicAuth()
		if uname != username || pass != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="MockServer"`)
			http.Error(w, "", http.StatusUnauthorized)
			return false
		}

	}
	return true
}
