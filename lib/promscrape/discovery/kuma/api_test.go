package kuma

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestGetAPIServerPathSuccess(t *testing.T) {
	f := func(server, expectedAPIServer, expectedAPIPath string) {
		t.Helper()
		apiServer, apiPath, err := getAPIServerPath(server)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if apiServer != expectedAPIServer {
			t.Fatalf("unexpected API server; got %q; want %q", apiServer, expectedAPIServer)
		}
		if apiPath != expectedAPIPath {
			t.Fatalf("unexpected API path; got %q; want %q", apiPath, expectedAPIPath)
		}
	}
	// url without path
	f("http://localhost:5676", "http://localhost:5676", "/v3/discovery:monitoringassignments")
	// url with path
	f("http://localhost:5676/", "http://localhost:5676", "/v3/discovery:monitoringassignments")
	f("https://foo.bar:1234/a/b", "https://foo.bar:1234", "/a/b/v3/discovery:monitoringassignments")
	// url with query args
	f("https://foo.bar:1234/a/b?c=d&arg2=value2", "https://foo.bar:1234", "/a/b/v3/discovery:monitoringassignments?c=d&arg2=value2")
	// missing scheme
	f("foo.bar", "http://foo.bar", "/v3/discovery:monitoringassignments")
	f("foo.bar:1234/a/b", "http://foo.bar:1234", "/a/b/v3/discovery:monitoringassignments")
	f("foo.bar:1234/a/b?c=d&arg2=value2", "http://foo.bar:1234", "/a/b/v3/discovery:monitoringassignments?c=d&arg2=value2")
}

func TestGetAPIConfigFailure(t *testing.T) {
	f := func(server string) {
		t.Helper()
		sdc := &SDConfig{
			Server: server,
		}
		cfg, err := getAPIConfig(sdc, ".")
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if cfg != nil {
			t.Fatalf("expecting nil cfg; got %v", cfg)
		}
	}
	// empty server url
	f("")
	// invalid server url
	f(":")
}

func TestParseTargetsLabels(t *testing.T) {
	data := `{
    "version_info":"5dc9a5dd-2091-4426-a886-dfdc24fc99d7",
    "resources":[
       {
          "@type":"type.googleapis.com/kuma.observability.v1.MonitoringAssignment",
          "mesh":"default",
          "service":"redis",
          "labels":{ "test":"test1" },
          "targets":[
             {
                "name":"redis",
                "scheme":"http",
                "address":"127.0.0.1:5670",
                "metrics_path":"/metrics",
                "labels":{ "kuma_io_protocol":"tcp" }
             }
          ]
       },
       {
          "@type":"type.googleapis.com/kuma.observability.v1.MonitoringAssignment",
          "mesh":"default",
          "service":"app",
          "labels":{ "test":"test2" },
          "targets":[
             {
                "name":"app",
                "scheme":"https",
                "address":"127.0.0.1:5671",
                "metrics_path":"/metrics/abc",
                "labels":{ "kuma_io_protocol":"http" }
             }
          ]
       }
    ],
    "type_url":"type.googleapis.com/kuma.observability.v1.MonitoringAssignment",
    "nonce": "foobar"
 }`
	labelss, versionInfo, nonce, err := parseTargetsLabels([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedLabelss := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                        "127.0.0.1:5670",
			"__meta_kuma_dataplane":              "redis",
			"__meta_kuma_label_kuma_io_protocol": "tcp",
			"__meta_kuma_label_test":             "test1",
			"__meta_kuma_mesh":                   "default",
			"__meta_kuma_service":                "redis",
			"__metrics_path__":                   "/metrics",
			"__scheme__":                         "http",
			"instance":                           "redis",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                        "127.0.0.1:5671",
			"__meta_kuma_dataplane":              "app",
			"__meta_kuma_label_kuma_io_protocol": "http",
			"__meta_kuma_label_test":             "test2",
			"__meta_kuma_mesh":                   "default",
			"__meta_kuma_service":                "app",
			"__metrics_path__":                   "/metrics/abc",
			"__scheme__":                         "https",
			"instance":                           "app",
		}),
	}
	discoveryutils.TestEqualLabelss(t, labelss, expectedLabelss)

	expectedVersionInfo := "5dc9a5dd-2091-4426-a886-dfdc24fc99d7"
	if versionInfo != expectedVersionInfo {
		t.Fatalf("unexpected versionInfo; got %q; want %q", versionInfo, expectedVersionInfo)
	}

	expectedNonce := "foobar"
	if nonce != expectedNonce {
		t.Fatalf("unexpected nonce; got %q; want %q", nonce, expectedNonce)
	}
}
