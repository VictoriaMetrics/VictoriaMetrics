package main

import (
	"fmt"
	"net/http"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzParsePath(f *testing.F) {
	f.Add([]byte("insert/1/prometheus/api/v1/write"))
	f.Add([]byte("insert/1:2/opentsdb/api/put"))
	f.Add([]byte("invalid/path"))
	for mode := byte(0); mode < 5; mode++ {
		f.Add([]byte{mode})
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		// Don't crash on invalid paths.
		_, _ = parsePath(string(data), nil)

		// Set up random values for accountID and projectID.
		var accountID, projectID uint32
		r := randfill.NewFromGoFuzz(data)
		r.Fill(&accountID)
		r.Fill(&projectID)
		mode := byte(0)
		if len(data) > 0 {
			mode = data[0] % 5
		}

		// Build a path with optional headers and parse it
		const suffix = "prometheus/api/v1/write"
		header := make(http.Header)
		var path, wantTenant string
		switch mode {
		case 0:
			wantTenant = fmt.Sprint(accountID)
			path = "insert/" + wantTenant + "/" + suffix
		case 1:
			wantTenant = fmt.Sprintf("%d:%d", accountID, projectID)
			path = "insert/" + wantTenant + "/" + suffix
		case 2:
			wantTenant = "multitenant"
			path = "insert/multitenant/" + suffix
		case 3:
			header.Set("AccountID", fmt.Sprint(accountID))
			header.Set("ProjectID", fmt.Sprint(projectID))
			wantTenant = fmt.Sprintf("%d:%d", accountID, projectID)
			path = "insert/" + suffix
		default:
			wantTenant = "0:0"
			path = "insert/" + suffix
		}
		p, err := parsePath(path, header)
		if err != nil {
			t.Fatal(err)
		}
		if p.Prefix != "insert" || p.AuthToken != wantTenant || p.Suffix != suffix {
			t.Fatalf("unexpected parsed path: %+v; want tenant %q and suffix %q", p, wantTenant, suffix)
		}
	})
}

func FuzzAuthTokenFromPath(f *testing.F) {
	f.Add([]byte("invalid/path"))
	f.Add([]byte("insert/1/opentsdb/api/put"))
	f.Add([]byte("insert/multitenant/opentsdb/api/put"))
	f.Add([]byte("select/1/opentsdb/api/put"))
	f.Add([]byte("insert/1/prometheus/api/v1/write"))
	f.Add([]byte("insert/invalid/opentsdb/api/put"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		// Don't crash on invalid paths.
		_, _ = getAuthTokenFromPath(string(data), nil)

		// Build paths using random accountID and projectID.
		var accountID, projectID uint32
		r := randfill.NewFromGoFuzz(data)
		r.Fill(&accountID)
		r.Fill(&projectID)

		header := make(http.Header)
		header.Set("AccountID", fmt.Sprint(accountID))
		header.Set("ProjectID", fmt.Sprint(projectID))
		for _, tc := range []struct {
			path    string
			header  http.Header
			wantErr bool
		}{
			{path: fmt.Sprintf("insert/%d:%d/opentsdb/api/put", accountID, projectID)},
			{path: "insert/opentsdb/api/put", header: header},
			{path: fmt.Sprintf("select/%d/opentsdb/api/put", accountID), wantErr: true},
			{path: fmt.Sprintf("insert/%d/prometheus/api/v1/write", accountID), wantErr: true},
			{path: "insert/invalid/opentsdb/api/put", wantErr: true},
		} {
			at, err := getAuthTokenFromPath(tc.path, tc.header)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.path)
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if at == nil || at.AccountID != accountID || at.ProjectID != projectID {
				t.Fatalf("unexpected auth token for %q with headers %v: %+v", tc.path, tc.header, at)
			}
		}
	})
}
