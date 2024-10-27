package puppetdb

import (
	"testing"
)

func Test_newAPIConfig(t *testing.T) {
	f := func(url, query string, includeParameters bool, port int, wantErr bool) {
		t.Helper()
		sdc := &SDConfig{
			URL:               url,
			Query:             query,
			IncludeParameters: includeParameters,
			Port:              port,
		}
		if _, err := newAPIConfig(sdc, ""); wantErr != (err != nil) {
			t.Fatalf("newAPIConfig want error = %t, but error = %v", wantErr, err)
		}
	}

	f("https://puppetdb.example.com", `resources { type = "Class" and title = "Prometheus::Node_exporter" }`, true, 9100, false)
	f("", `resources { type = "Class" and title = "Prometheus::Node_exporter" }`, true, 9100, true)
	f("https://puppetdb.example.com", ``, true, 9100, true)
	f("ftp://invalid.url", `resources { type = "Class" and title = "Prometheus::Node_exporter" }`, true, 9100, true)
}
