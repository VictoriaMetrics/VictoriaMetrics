package notifier

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
)

func TestConfigWatcherReload(t *testing.T) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer fs.MustRemovePath(f.Name())

	writeToFile(f.Name(), `
static_configs:
  - targets:
      - localhost:9093
      - localhost:9094
`)
	cfg, err := parseConfig(f.Name())
	if err != nil {
		t.Fatalf("failed to parse config: %s", err)
	}
	cw, err := newWatcher(cfg, nil)
	if err != nil {
		t.Fatalf("failed to start config watcher: %s", err)
	}
	defer cw.mustStop()
	ns := cw.notifiers()
	if len(ns) != 2 {
		t.Fatalf("expected to have 2 notifiers; got %d %#v", len(ns), ns)
	}

	f2, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer fs.MustRemovePath(f2.Name())

	writeToFile(f2.Name(), `
static_configs:
  - targets:
      - 127.0.0.1:9093
`)
	checkErr(t, cw.reload(f2.Name()))

	ns = cw.notifiers()
	if len(ns) != 1 {
		t.Fatalf("expected to have 1 notifier; got %d", len(ns))
	}
	expAddr := "http://127.0.0.1:9093/api/v2/alerts"
	if ns[0].Addr() != expAddr {
		t.Fatalf("expected to get %q; got %q instead", expAddr, ns[0].Addr())
	}
}

func TestConfigWatcherStart(t *testing.T) {
	oldSDCheckInterval := consul.SDCheckInterval
	defer func() { consul.SDCheckInterval = oldSDCheckInterval }()
	consulCheckInterval := 100 * time.Millisecond
	consul.SDCheckInterval = &consulCheckInterval

	consulSDServer := newFakeConsulServer()
	defer consulSDServer.Close()

	consulSDFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer fs.MustRemovePath(consulSDFile.Name())

	writeToFile(consulSDFile.Name(), fmt.Sprintf(`
scheme: https
path_prefix: proxy
consul_sd_configs:
  - server: %s
    services:
      - alertmanager
  - server: %s
    services:
      - alertmanager
    alert_relabel_configs:
    - target_label: "foo"
      replacement: "tar"
`, consulSDServer.URL, consulSDServer.URL))

	cfg, err := parseConfig(consulSDFile.Name())
	if err != nil {
		t.Fatalf("failed to parse config: %s", err)
	}
	cw, err := newWatcher(cfg, nil)
	if err != nil {
		t.Fatalf("failed to start config watcher: %s", err)
	}
	defer cw.mustStop()

	if len(cw.notifiers()) != 3 {
		t.Fatalf("expected to get 3 notifiers; got %d", len(cw.notifiers()))
	}

	expAddr1 := fmt.Sprintf("https://%s/proxy/api/v2/alerts", fakeConsulService1)
	expAddr2 := fmt.Sprintf("https://%s/proxy/api/v2/alerts", fakeConsulService2)
	expAddr3 := fmt.Sprintf("https://%s/proxy/api/v2/alerts", fakeConsulService3)

	n1, n2, n3 := cw.notifiers()[0], cw.notifiers()[1], cw.notifiers()[2]
	if n1.Addr() != expAddr1 {
		t.Fatalf("exp address %q; got %q", expAddr1, n1.Addr())
	}
	if n2.Addr() != expAddr2 {
		t.Fatalf("exp address %q; got %q", expAddr2, n2.Addr())
	}
	if n3.Addr() != expAddr3 {
		t.Fatalf("exp address %q; got %q", expAddr3, n3.Addr())
	}

	if n1.(*AlertManager).relabelConfigs.String() != "" {
		t.Fatalf("unexpected relabel configs: %q", n1.(*AlertManager).relabelConfigs.String())
	}
	if n2.(*AlertManager).relabelConfigs.String() != "" {
		t.Fatalf("unexpected relabel configs: %q", n2.(*AlertManager).relabelConfigs.String())
	}
	if n3.(*AlertManager).relabelConfigs.String() != "- target_label: foo\n  replacement: tar\n" {
		t.Fatalf("unexpected relabel configs: %q", n3.(*AlertManager).relabelConfigs.String())
	}

	f := func() bool { return len(cw.notifiers()) == 1 }
	if !waitFor(f, time.Second) {
		t.Fatalf("expected to get 1 notifiers; got %d", len(cw.notifiers()))
	}
	n3 = cw.notifiers()[0]
	if n3.Addr() != expAddr3 {
		t.Fatalf("exp address %q; got %q", expAddr3, n3.Addr())
	}
	if n3.(*AlertManager).relabelConfigs.String() != "- target_label: foo\n  replacement: tar\n" {
		t.Fatalf("unexpected relabel configs: %q", n3.(*AlertManager).relabelConfigs.String())
	}
}

// TestConfigWatcherReloadConcurrent supposed to test concurrent
// execution of configuration update.
// Should be executed with -race flag
func TestConfigWatcherReloadConcurrent(t *testing.T) {
	consulSDServer1 := newFakeConsulServer()
	defer consulSDServer1.Close()
	consulSDServer2 := newFakeConsulServer()
	defer consulSDServer2.Close()

	consulSDFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer fs.MustRemovePath(consulSDFile.Name())

	writeToFile(consulSDFile.Name(), fmt.Sprintf(`
consul_sd_configs:
  - server: %s
    services:
      - alertmanager
  - server: %s
    services:
      - consul
`, consulSDServer1.URL, consulSDServer2.URL))

	staticAndConsulSDFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer fs.MustRemovePath(staticAndConsulSDFile.Name())

	writeToFile(staticAndConsulSDFile.Name(), fmt.Sprintf(`
static_configs:
  - targets:
      - localhost:9093
      - localhost:9095
consul_sd_configs:
  - server: %s
    services:
      - alertmanager
  - server: %s
    services:
      - consul
`, consulSDServer1.URL, consulSDServer2.URL))

	paths := []string{
		staticAndConsulSDFile.Name(),
		consulSDFile.Name(),
		"testdata/static.good.yaml",
		"unknownFields.bad.yaml",
	}

	cfg, err := parseConfig(paths[0])
	if err != nil {
		t.Fatalf("failed to parse config: %s", err)
	}
	cw, err := newWatcher(cfg, nil)
	if err != nil {
		t.Fatalf("failed to start config watcher: %s", err)
	}
	defer cw.mustStop()

	const workers = 500
	const iterations = 10
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(n)))
			for i := 0; i < iterations; i++ {
				rnd := r.Intn(len(paths))
				_ = cw.reload(paths[rnd]) // update can fail and this is expected
				_ = cw.notifiers()
			}
		}(i)
	}
	wg.Wait()
}

func writeToFile(file, b string) {
	fs.MustWriteSync(file, []byte(b))
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
}

const (
	fakeConsulService1 = "127.0.0.1:9093"
	fakeConsulService2 = "127.0.0.1:9095"
	fakeConsulService3 = "127.0.0.1:9097"
)

func newFakeConsulServer() *httptest.Server {
	var requestCount atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/self", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Write([]byte(`{"Config": {"Datacenter": "dc1"}}`))
	})
	mux.HandleFunc("/v1/catalog/services", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("X-Consul-Index", "1")
		rw.Write([]byte(`{
    "alertmanager": [
        "alertmanager",
        "__scheme__=http"
    ]
}`))
	})
	mux.HandleFunc("/v1/health/service/alertmanager", func(rw http.ResponseWriter, _ *http.Request) {
		if requestCount.Load() == 0 {
			rw.Header().Set("X-Consul-Index", "1")
			rw.Write([]byte(`
[
    {
        "Node": {
            "ID": "e8e3629a-3f50-9d6e-aaf8-f173b5b05c72",
            "Node": "machine",
            "Address": "127.0.0.1",
            "Datacenter": "dc1",
            "TaggedAddresses": {
                "lan": "127.0.0.1",
                "lan_ipv4": "127.0.0.1",
                "wan": "127.0.0.1",
                "wan_ipv4": "127.0.0.1"
            },
            "Meta": {
                "consul-network-segment": ""
            },
            "CreateIndex": 13,
            "ModifyIndex": 14
        },
        "Service": {
            "ID": "am1",
            "Service": "alertmanager",
            "Tags": [
                "alertmanager",
                "__scheme__=http"
            ],
            "Address": "",
            "Meta": null,
            "Port": 9093,
            "Weights": {
                "Passing": 1,
                "Warning": 1
            },
            "EnableTagOverride": false,
            "Proxy": {
                "Mode": "",
                "MeshGateway": {},
                "Expose": {}
            },
            "Connect": {},
            "CreateIndex": 16,
            "ModifyIndex": 16
        }
    },
    {
        "Node": {
            "ID": "e8e3629a-3f50-9d6e-aaf8-f173b5b05c72",
            "Node": "machine",
            "Address": "127.0.0.1",
            "Datacenter": "dc1",
            "TaggedAddresses": {
                "lan": "127.0.0.1",
                "lan_ipv4": "127.0.0.1",
                "wan": "127.0.0.1",
                "wan_ipv4": "127.0.0.1"
            },
            "Meta": {
                "consul-network-segment": ""
            },
            "CreateIndex": 13,
            "ModifyIndex": 14
        },
        "Service": {
            "ID": "am2",
            "Service": "alertmanager",
            "Tags": [
                "alertmanager",
                "bad-node"
            ],
            "Address": "",
            "Meta": null,
            "Port": 9095,
            "Weights": {
                "Passing": 1,
                "Warning": 1
            },
            "EnableTagOverride": false,
            "Proxy": {
                "Mode": "",
                "MeshGateway": {},
                "Expose": {}
            },
            "Connect": {},
            "CreateIndex": 15,
            "ModifyIndex": 15
        }
    }
]`))
		} else {
			rw.Header().Set("X-Consul-Index", "2")
			rw.Write([]byte(`
[
    {
        "Node": {
            "ID": "e8e3629a-3f50-9d6e-aaf8-f173b5b05c72",
            "Node": "machine",
            "Address": "127.0.0.1",
            "Datacenter": "dc1",
            "TaggedAddresses": {
                "lan": "127.0.0.1",
                "lan_ipv4": "127.0.0.1",
                "wan": "127.0.0.1",
                "wan_ipv4": "127.0.0.1"
            },
            "Meta": {
                "consul-network-segment": ""
            },
            "CreateIndex": 13,
            "ModifyIndex": 14
        },
        "Service": {
            "ID": "am3",
            "Service": "alertmanager",
            "Tags": [
                "alertmanager",
                "__scheme__=http"
            ],
            "Address": "",
            "Meta": null,
            "Port": 9097,
            "Weights": {
                "Passing": 1,
                "Warning": 1
            },
            "EnableTagOverride": false,
            "Proxy": {
                "Mode": "",
                "MeshGateway": {},
                "Expose": {}
            },
            "Connect": {},
            "CreateIndex": 16,
            "ModifyIndex": 16
        }
    }
]`))
		}
		requestCount.Add(1)
	})

	return httptest.NewServer(mux)
}

func TestMergeHTTPClientConfigs(t *testing.T) {
	cfg1 := promauth.HTTPClientConfig{Headers: []string{"Header:Foo"}}
	cfg2 := promauth.HTTPClientConfig{BasicAuth: &promauth.BasicAuthConfig{
		Username: "foo",
		Password: promauth.NewSecret("bar"),
	}}

	result := mergeHTTPClientConfigs(cfg1, cfg2)

	if result.Headers == nil {
		t.Fatalf("expected Headers to be inherited")
	}
	if result.BasicAuth == nil {
		t.Fatalf("expected BasicAuth tp be present")
	}
}

func TestParseLabels_Failure(t *testing.T) {
	f := func(target string, cfg *Config) {
		t.Helper()

		_, _, err := parseLabels(target, nil, cfg)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// invalid address
	f("invalid:*//url", &Config{})
}

func TestParseLabels_Success(t *testing.T) {
	f := func(target string, cfg *Config, expectedAddress string) {
		t.Helper()

		address, _, err := parseLabels(target, nil, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if address != expectedAddress {
			t.Fatalf("unexpected address; got %q; want %q", address, expectedAddress)
		}
	}

	// use some default params
	f("alertmanager:9093", &Config{
		PathPrefix: "test",
	}, "http://alertmanager:9093/test/api/v2/alerts")

	// use target address
	f("https://alertmanager:9093/api/v1/alerts", &Config{
		Scheme:     "http",
		PathPrefix: "test",
	}, "https://alertmanager:9093/api/v1/alerts")
}

func waitFor(f func() bool, timeout time.Duration) bool {
	for start := time.Now(); time.Since(start) < timeout; {
		if f() == true {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
