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

func Test_Validate(t *testing.T) {
	tests := []struct {
		name         string
		snapshotName string
		want         bool
	}{
		{
			name:         "empty snapshot name",
			snapshotName: "",
			want:         false,
		},
		{
			name:         "short snapshot name",
			snapshotName: "",
			want:         false,
		},
		{
			name:         "short first part of the snapshot name",
			snapshotName: "2022050312163-16EB56ADB4110CF2",
			want:         false,
		},
		{
			name:         "short second part of the snapshot name",
			snapshotName: "20220503121638-16EB56ADB4110CF",
			want:         true,
		},
		{
			name:         "correct snapshot name",
			snapshotName: "20220503121638-16EB56ADB4110CF2",
			want:         true,
		},
		{
			name:         "invalid time part snapshot name",
			snapshotName: "00000000000000-16EB56ADB4110CF2",
			want:         false,
		},
		{
			name:         "not enough parts of the snapshot name",
			snapshotName: "2022050312163816EB56ADB4110CF2",
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.snapshotName); (err == nil) != tt.want {
				t.Errorf("checkSnapshotName() = %v, want %v", err, tt.want)
			}
		})
	}
}
