package notifier

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func TestConfigWatcherReload(t *testing.T) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()

	writeToFile(t, f.Name(), `
static_configs:
  - targets:
      - localhost:9093
      - localhost:9094
`)
	cw, err := newWatcher(f.Name(), nil)
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
	defer func() { _ = os.Remove(f2.Name()) }()

	writeToFile(t, f2.Name(), `
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
	consulSDServer := newFakeConsulServer()
	defer consulSDServer.Close()

	consulSDFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(consulSDFile.Name()) }()

	writeToFile(t, consulSDFile.Name(), fmt.Sprintf(`
scheme: https
path_prefix: proxy
consul_sd_configs:
  - server: %s
    services:
      - alertmanager
`, consulSDServer.URL))

	cw, err := newWatcher(consulSDFile.Name(), nil)
	if err != nil {
		t.Fatalf("failed to start config watcher: %s", err)
	}
	defer cw.mustStop()

	if len(cw.notifiers()) != 2 {
		t.Fatalf("expected to get 2 notifiers; got %d", len(cw.notifiers()))
	}

	expAddr1 := fmt.Sprintf("https://%s/proxy/api/v2/alerts", fakeConsulService1)
	expAddr2 := fmt.Sprintf("https://%s/proxy/api/v2/alerts", fakeConsulService2)

	n1, n2 := cw.notifiers()[0], cw.notifiers()[1]
	if n1.Addr() != expAddr1 {
		t.Fatalf("exp address %q; got %q", expAddr1, n1.Addr())
	}
	if n2.Addr() != expAddr2 {
		t.Fatalf("exp address %q; got %q", expAddr2, n2.Addr())
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
	defer func() { _ = os.Remove(consulSDFile.Name()) }()

	writeToFile(t, consulSDFile.Name(), fmt.Sprintf(`
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
	defer func() { _ = os.Remove(staticAndConsulSDFile.Name()) }()

	writeToFile(t, staticAndConsulSDFile.Name(), fmt.Sprintf(`
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

	cw, err := newWatcher(paths[0], nil)
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

func writeToFile(t *testing.T, file, b string) {
	t.Helper()
	checkErr(t, os.WriteFile(file, []byte(b), 0644))
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
)

func newFakeConsulServer() *httptest.Server {
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

func TestParseLabels(t *testing.T) {
	testCases := []struct {
		name            string
		target          string
		cfg             *Config
		expectedAddress string
		expectedErr     bool
	}{
		{
			"invalid address",
			"invalid:*//url",
			&Config{},
			"",
			true,
		},
		{
			"use some default params",
			"alertmanager:9093",
			&Config{PathPrefix: "test"},
			"http://alertmanager:9093/test/api/v2/alerts",
			false,
		},
		{
			"use target address",
			"https://alertmanager:9093/api/v1/alerts",
			&Config{Scheme: "http", PathPrefix: "test"},
			"https://alertmanager:9093/api/v1/alerts",
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			address, _, err := parseLabels(tc.target, nil, tc.cfg)
			if err == nil == tc.expectedErr {
				t.Fatalf("unexpected error; got %t; want %t", err != nil, tc.expectedErr)
			}
			if address != tc.expectedAddress {
				t.Fatalf("unexpected address; got %q; want %q", address, tc.expectedAddress)
			}
		})
	}
}
