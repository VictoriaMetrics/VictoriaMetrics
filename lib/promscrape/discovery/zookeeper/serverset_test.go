package zookeeper

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestParseServersetMemberFailure(t *testing.T) {
	f := func(data string) {
		t.Helper()
		_, err := parseServersetMember([]byte(data), "/path/member_0000000000")
		if err == nil {
			t.Fatalf("expecting non-nil error for data %q", data)
		}
	}
	// empty data
	f("")
	// invalid JSON
	f("{invalid")
}

func TestParseServersetMemberSuccess(t *testing.T) {
	data := `{
		"serviceEndpoint": {"host": "192.168.1.10", "port": 9090},
		"status": "ALIVE",
		"shard": 0
	}`
	m, err := parseServersetMember([]byte(data), "/discovery/service/member_0000000000")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	want := promutil.NewLabels(6)
	want.Add("__address__", "192.168.1.10:9090")
	want.Add("__meta_serverset_endpoint_host", "192.168.1.10")
	want.Add("__meta_serverset_endpoint_port", "9090")
	want.Add("__meta_serverset_path", "/discovery/service/member_0000000000")
	want.Add("__meta_serverset_shard", "0")
	want.Add("__meta_serverset_status", "ALIVE")
	discoveryutil.TestEqualLabelss(t, []*promutil.Labels{m}, []*promutil.Labels{want})
}

func TestParseServersetMemberWithAdditionalEndpoints(t *testing.T) {
	data := `{
		"serviceEndpoint": {"host": "10.0.0.1", "port": 8080},
		"additionalEndpoints": {
			"health-check": {"host": "10.0.0.1", "port": 8081},
			"admin": {"host": "10.0.0.1", "port": 8082}
		},
		"status": "ALIVE",
		"shard": 1
	}`
	m, err := parseServersetMember([]byte(data), "/services/web/member_0000000001")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	want := promutil.NewLabels(10)
	want.Add("__address__", "10.0.0.1:8080")
	want.Add("__meta_serverset_endpoint_host", "10.0.0.1")
	want.Add("__meta_serverset_endpoint_host_admin", "10.0.0.1")
	want.Add("__meta_serverset_endpoint_host_health_check", "10.0.0.1")
	want.Add("__meta_serverset_endpoint_port", "8080")
	want.Add("__meta_serverset_endpoint_port_admin", "8082")
	want.Add("__meta_serverset_endpoint_port_health_check", "8081")
	want.Add("__meta_serverset_path", "/services/web/member_0000000001")
	want.Add("__meta_serverset_shard", "1")
	want.Add("__meta_serverset_status", "ALIVE")
	discoveryutil.TestEqualLabelss(t, []*promutil.Labels{m}, []*promutil.Labels{want})
}

func TestParseServersetMemberIPv6(t *testing.T) {
	data := `{
		"serviceEndpoint": {"host": "::1", "port": 9090},
		"status": "ALIVE",
		"shard": 0
	}`
	m, err := parseServersetMember([]byte(data), "/services/ipv6/member_0000000000")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	want := promutil.NewLabels(6)
	want.Add("__address__", "[::1]:9090")
	want.Add("__meta_serverset_endpoint_host", "::1")
	want.Add("__meta_serverset_endpoint_port", "9090")
	want.Add("__meta_serverset_path", "/services/ipv6/member_0000000000")
	want.Add("__meta_serverset_shard", "0")
	want.Add("__meta_serverset_status", "ALIVE")
	discoveryutil.TestEqualLabelss(t, []*promutil.Labels{m}, []*promutil.Labels{want})
}
