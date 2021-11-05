package promscrape

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

func TestNeedSkipScrapeWork(t *testing.T) {
	f := func(key string, membersCount, replicationFactor, memberNum int, needSkipExpected bool) {
		t.Helper()
		needSkip := needSkipScrapeWork(key, membersCount, replicationFactor, memberNum)
		if needSkip != needSkipExpected {
			t.Fatalf("unexpected needSkipScrapeWork(key=%q, membersCount=%d, replicationFactor=%d, memberNum=%d); got %v; want %v",
				key, membersCount, replicationFactor, memberNum, needSkip, needSkipExpected)
		}
	}
	// Disabled clustering
	f("foo", 0, 0, 0, false)

	// A cluster with 2 nodes with disabled replication
	f("foo", 2, 0, 0, true)
	f("foo", 2, 0, 1, false)

	// A cluster with 2 nodes with replicationFactor=2
	f("foo", 2, 2, 0, false)
	f("foo", 2, 2, 1, false)

	// A cluster with 3 nodes with replicationFactor=2
	f("foo", 3, 2, 0, false)
	f("foo", 3, 2, 1, true)
	f("foo", 3, 2, 2, false)
}

func TestLoadStaticConfigs(t *testing.T) {
	scs, err := loadStaticConfigs("testdata/file_sd.json")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(scs) == 0 {
		t.Fatalf("expecting non-zero static configs")
	}

	// Try loading non-existing file
	scs, err = loadStaticConfigs("testdata/non-exsiting-file")
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if scs != nil {
		t.Fatalf("unexpected non-nil static configs: %#v", scs)
	}

	// Try loading invalid file
	scs, err = loadStaticConfigs("testdata/prometheus.yml")
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if scs != nil {
		t.Fatalf("unexpected non-nil static configs: %#v", scs)
	}
}

func TestLoadConfig(t *testing.T) {
	cfg, data, err := loadConfig("testdata/prometheus.yml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if cfg == nil {
		t.Fatalf("expecting non-nil config")
	}
	if data == nil {
		t.Fatalf("expecting non-nil data")
	}

	cfg, data, err = loadConfig("testdata/prometheus-with-scrape-config-files.yml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if cfg == nil {
		t.Fatalf("expecting non-nil config")
	}
	if data == nil {
		t.Fatalf("expecting non-nil data")
	}

	// Try loading non-existing file
	cfg, data, err = loadConfig("testdata/non-existing-file")
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if cfg != nil {
		t.Fatalf("unexpected non-nil config: %#v", cfg)
	}
	if data != nil {
		t.Fatalf("unexpected data wit length=%d: %q", len(data), data)
	}

	// Try loading invalid file
	cfg, data, err = loadConfig("testdata/file_sd_1.yml")
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if cfg != nil {
		t.Fatalf("unexpected non-nil config: %#v", cfg)
	}
	if data != nil {
		t.Fatalf("unexpected data wit length=%d: %q", len(data), data)
	}
}

func TestBlackboxExporter(t *testing.T) {
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/684
	data := `
scrape_configs:
  - job_name: 'blackbox'
    metrics_path: /probe
    params:
      module: [dns_udp_example]  # Look for  dns response
    static_configs:
      - targets:
        - 8.8.8.8
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: black:9115  # The blackbox exporter's real hostname:port.%
`
	var cfg Config
	allData, err := cfg.parseData([]byte(data), "sss")
	if err != nil {
		t.Fatalf("cannot parase data: %s", err)
	}
	if string(allData) != data {
		t.Fatalf("invalid data returned from parseData;\ngot\n%s\nwant\n%s", allData, data)
	}
	sws := cfg.getStaticScrapeWork()
	resetNonEssentialFields(sws)
	swsExpected := []*ScrapeWork{{
		ScrapeURL:       "http://black:9115/probe?module=dns_udp_example&target=8.8.8.8",
		ScrapeInterval:  defaultScrapeInterval,
		ScrapeTimeout:   defaultScrapeTimeout,
		HonorTimestamps: true,
		Labels: []prompbmarshal.Label{
			{
				Name:  "__address__",
				Value: "black:9115",
			},
			{
				Name:  "__metrics_path__",
				Value: "/probe",
			},
			{
				Name:  "__param_module",
				Value: "dns_udp_example",
			},
			{
				Name:  "__param_target",
				Value: "8.8.8.8",
			},
			{
				Name:  "__scheme__",
				Value: "http",
			},
			{
				Name:  "__scrape_interval__",
				Value: "1m0s",
			},
			{
				Name:  "__scrape_timeout__",
				Value: "10s",
			},
			{
				Name:  "instance",
				Value: "8.8.8.8",
			},
			{
				Name:  "job",
				Value: "blackbox",
			},
		},
		AuthConfig:      &promauth.Config{},
		ProxyAuthConfig: &promauth.Config{},
		jobNameOriginal: "blackbox",
	}}
	if !reflect.DeepEqual(sws, swsExpected) {
		t.Fatalf("unexpected scrapeWork;\ngot\n%+v\nwant\n%+v", sws, swsExpected)
	}
}

func TestGetFileSDScrapeWork(t *testing.T) {
	data := `
scrape_configs:
- job_name: foo
  file_sd_configs:
  - files: [testdata/file_sd.json]
`
	var cfg Config
	allData, err := cfg.parseData([]byte(data), "sss")
	if err != nil {
		t.Fatalf("cannot parase data: %s", err)
	}
	if string(allData) != data {
		t.Fatalf("invalid data returned from parseData;\ngot\n%s\nwant\n%s", allData, data)
	}
	sws := cfg.getFileSDScrapeWork(nil)
	if !equalStaticConfigForScrapeWorks(sws, sws) {
		t.Fatalf("unexpected non-equal static configs;\nsws:\n%#v", sws)
	}

	// Load another static config
	dataNew := `
scrape_configs:
- job_name: foo
  file_sd_configs:
  - files: [testdata/file_sd_1.yml]
`
	var cfgNew Config
	allData, err = cfgNew.parseData([]byte(dataNew), "sss")
	if err != nil {
		t.Fatalf("cannot parse data: %s", err)
	}
	if string(allData) != dataNew {
		t.Fatalf("invalid data returned from parseData;\ngot\n%s\nwant\n%s", allData, dataNew)
	}
	swsNew := cfgNew.getFileSDScrapeWork(sws)
	if equalStaticConfigForScrapeWorks(swsNew, sws) {
		t.Fatalf("unexpected equal static configs;\nswsNew:\n%#v\nsws:\n%#v", swsNew, sws)
	}

	// Try loading invalid static config
	data = `
scrape_configs:
- job_name: foo
  file_sd_configs:
  - files: [testdata/prometheus.yml]
`
	allData, err = cfg.parseData([]byte(data), "sss")
	if err != nil {
		t.Fatalf("cannot parse data: %s", err)
	}
	if string(allData) != data {
		t.Fatalf("invalid data returned from parseData;\ngot\n%s\nwant\n%s", allData, data)
	}
	sws = cfg.getFileSDScrapeWork(swsNew)
	if len(sws) != 0 {
		t.Fatalf("unexpected non-empty sws:\n%#v", sws)
	}

	// Empty target in static config
	data = `
scrape_configs:
- job_name: foo
  file_sd_configs:
  - files: [testdata/empty_target_file_sd.yml]
`
	allData, err = cfg.parseData([]byte(data), "sss")
	if err != nil {
		t.Fatalf("cannot parse data: %s", err)
	}
	if string(allData) != data {
		t.Fatalf("invalid data returned from parseData;\ngot\n%s\nwant\n%s", allData, data)
	}
	sws = cfg.getFileSDScrapeWork(swsNew)
	if len(sws) != 0 {
		t.Fatalf("unexpected non-empty sws:\n%#v", sws)
	}
}

func getFileSDScrapeWork(data []byte, path string) ([]*ScrapeWork, error) {
	var cfg Config
	allData, err := cfg.parseData(data, path)
	if err != nil {
		return nil, fmt.Errorf("cannot parse data: %w", err)
	}
	if !bytes.Equal(allData, data) {
		return nil, fmt.Errorf("invalid data returned from parseData;\ngot\n%s\nwant\n%s", allData, data)
	}
	return cfg.getFileSDScrapeWork(nil), nil
}

func getStaticScrapeWork(data []byte, path string) ([]*ScrapeWork, error) {
	var cfg Config
	allData, err := cfg.parseData(data, path)
	if err != nil {
		return nil, fmt.Errorf("cannot parse data: %w", err)
	}
	if !bytes.Equal(allData, data) {
		return nil, fmt.Errorf("invalid data returned from parseData;\ngot\n%s\nwant\n%s", allData, data)
	}
	return cfg.getStaticScrapeWork(), nil
}

func TestGetStaticScrapeWorkFailure(t *testing.T) {
	f := func(data string) {
		t.Helper()
		sws, err := getStaticScrapeWork([]byte(data), "non-existing-file")
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if sws != nil {
			t.Fatalf("expecting nil sws")
		}
	}

	// incorrect yaml
	f(`foo bar baz`)

	// Missing job_name
	f(`
scrape_configs:
- static_configs:
  - targets: ["foo"]
`)

	// Duplicate job_name
	f(`
scrape_configs:
- job_name: foo
  static_configs:
    targets: ["foo"]
- job_name: foo
  static_configs:
    targets: ["bar"]
`)

	// Invalid scheme
	f(`
scrape_configs:
- job_name: x
  scheme: asdf
  static_configs:
  - targets: ["foo"]
`)

	// Missing username in `basic_auth`
	f(`
scrape_configs:
- job_name: x
  basic_auth:
    password: sss
  static_configs:
  - targets: ["a"]
`)

	// Both password and password_file set in `basic_auth`
	f(`
scrape_configs:
- job_name: x
  basic_auth:
    username: foobar
    password: sss
    password_file: sdfdf
  static_configs:
  - targets: ["a"]
`)

	// Invalid password_file set in `basic_auth`
	f(`
scrape_configs:
- job_name: x
  basic_auth:
    username: foobar
    password_file: ['foobar']
  static_configs:
  - targets: ["a"]
`)

	// Both `bearer_token` and `bearer_token_file` are set
	f(`
scrape_configs:
- job_name: x
  bearer_token: foo
  bearer_token_file: bar
  static_configs:
  - targets: ["a"]
`)

	// Both `basic_auth` and `bearer_token` are set
	f(`
scrape_configs:
- job_name: x
  bearer_token: foo
  basic_auth:
    username: foo
    password: bar
  static_configs:
  - targets: ["a"]
`)

	// Both `authorization` and `basic_auth` are set
	f(`
scrape_configs:
- job_name: x
  authorization:
    credentials: foobar
  basic_auth:
    username: foobar
  static_configs:
  - targets: ["a"]
`)

	// Both `authorization` and `bearer_token` are set
	f(`
scrape_configs:
- job_name: x
  authorization:
    credentials: foobar
  bearer_token: foo
  static_configs:
  - targets: ["a"]
`)

	// Invalid `bearer_token_file`
	f(`
scrape_configs:
- job_name: x
  bearer_token_file: [foobar]
  static_configs:
  - targets: ["a"]
`)

	// non-existing ca_file
	f(`
scrape_configs:
- job_name: aa
  tls_config:
    ca_file: non/extising/file
  static_configs:
  - targets: ["s"]
`)

	// invalid ca_file
	f(`
scrape_configs:
- job_name: aa
  tls_config:
    ca_file: testdata/prometheus.yml
  static_configs:
  - targets: ["s"]
`)

	// non-existing cert_file
	f(`
scrape_configs:
- job_name: aa
  tls_config:
    cert_file: non/extising/file
  static_configs:
  - targets: ["s"]
`)

	// non-existing key_file
	f(`
scrape_configs:
- job_name: aa
  tls_config:
    key_file: non/extising/file
  static_configs:
  - targets: ["s"]
`)

	// Invalid regex in relabel_configs
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - regex: "("
    source_labels: [foo]
    target_label: bar
  static_configs:
  - targets: ["s"]
`)

	// Missing target_label for action=replace in relabel_configs
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: replace
    source_labels: [foo]
  static_configs:
  - targets: ["s"]
`)

	// Missing source_labels for action=keep in relabel_configs
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: keep
  static_configs:
  - targets: ["s"]
`)

	// Missing source_labels for action=drop in relabel_configs
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: drop
  static_configs:
  - targets: ["s"]
`)

	// Missing source_labels for action=hashmod in relabel_configs
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    target_label: bar
    modulus: 123
  static_configs:
  - targets: ["s"]
`)

	// Missing target for action=hashmod in relabel_configs
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    source_labels: [foo]
    modulus: 123
  static_configs:
  - targets: ["s"]
`)

	// Missing modulus for action=hashmod in relabel_configs
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    source_labels: [foo]
    target_label: bar
  static_configs:
  - targets: ["s"]
`)

	// Invalid action in relabel_configs
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: foobar
  static_configs:
  - targets: ["s"]
`)

	// Invalid scrape_config_files contents
	f(`
scrape_config_files:
- job_name: aa
  static_configs:
  - targets: ["s"]
`)
}

func resetNonEssentialFields(sws []*ScrapeWork) {
	for i := range sws {
		sws[i].OriginalLabels = nil
	}
}

// String returns human-readable representation for sw.
func (sw *ScrapeWork) String() string {
	return strconv.Quote(sw.key())
}

func TestGetFileSDScrapeWorkSuccess(t *testing.T) {
	f := func(data string, expectedSws []*ScrapeWork) {
		t.Helper()
		sws, err := getFileSDScrapeWork([]byte(data), "non-existing-file")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		resetNonEssentialFields(sws)

		// Remove `__vm_filepath` label, since its value depends on the current working dir.
		for _, sw := range sws {
			for j := range sw.Labels {
				label := &sw.Labels[j]
				if label.Name == "__vm_filepath" {
					label.Value = ""
				}
			}
		}
		if !reflect.DeepEqual(sws, expectedSws) {
			t.Fatalf("unexpected scrapeWork; got\n%+v\nwant\n%+v", sws, expectedSws)
		}
	}
	f(`
scrape_configs:
- job_name: foo
  static_configs:
  - targets: ["xxx"]
`, []*ScrapeWork{})
	f(`
scrape_configs:
- job_name: foo
  metrics_path: /abc/de
  file_sd_configs:
  - files: ["testdata/file_sd.json", "testdata/file_sd*.yml"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://host1:80/abc/de",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "host1",
				},
				{
					Name:  "__metrics_path__",
					Value: "/abc/de",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "__vm_filepath",
					Value: "",
				},
				{
					Name:  "instance",
					Value: "host1:80",
				},
				{
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "qwe",
					Value: "rty",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:       "http://host2:80/abc/de",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "host2",
				},
				{
					Name:  "__metrics_path__",
					Value: "/abc/de",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "__vm_filepath",
					Value: "",
				},
				{
					Name:  "instance",
					Value: "host2:80",
				},
				{
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "qwe",
					Value: "rty",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:       "http://localhost:9090/abc/de",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "localhost:9090",
				},
				{
					Name:  "__metrics_path__",
					Value: "/abc/de",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "__vm_filepath",
					Value: "",
				},
				{
					Name:  "instance",
					Value: "localhost:9090",
				},
				{
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "yml",
					Value: "test",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
	})
}

func TestGetStaticScrapeWorkSuccess(t *testing.T) {
	f := func(data string, expectedSws []*ScrapeWork) {
		t.Helper()
		sws, err := getStaticScrapeWork([]byte(data), "non-exsiting-file")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		resetNonEssentialFields(sws)
		if !reflect.DeepEqual(sws, expectedSws) {
			t.Fatalf("unexpected scrapeWork; got\n%+v\nwant\n%+v", sws, expectedSws)
		}
	}
	f(``, nil)
	f(`
scrape_configs:
- job_name: foo
  static_configs:
  - targets: ["foo.bar:1234"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "instance",
					Value: "foo.bar:1234",
				},
				{
					Name:  "job",
					Value: "foo",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
	})
	f(`
global:
  external_labels:
    datacenter: foobar
    jobs: xxx
scrape_configs:
- job_name: foo
  static_configs:
  - targets: ["foo.bar:1234"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "datacenter",
					Value: "foobar",
				},
				{
					Name:  "instance",
					Value: "foo.bar:1234",
				},
				{
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "jobs",
					Value: "xxx",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
	})
	f(`
global:
  scrape_interval: 8s
  scrape_timeout: 34s
scrape_configs:
- job_name: foo
  scrape_interval: 54s
  scrape_timeout: 12s
  metrics_path: /foo/bar
  scheme: https
  honor_labels: true
  honor_timestamps: false
  follow_redirects: false
  params:
    p: ["x&y", "="]
    xaa:
  proxy_url: http://foo.bar
  static_configs:
  - targets: ["foo.bar", "aaa"]
    labels:
      x: y
      __scrape_timeout__: "5s"
- job_name: qwer
  tls_config:
    server_name: foobar
    insecure_skip_verify: true
  static_configs:
  - targets: [1.2.3.4]
- job_name: asdf
  static_configs:
  - targets: [foobar]
`, []*ScrapeWork{
		{
			ScrapeURL:       "https://foo.bar:443/foo/bar?p=x%26y&p=%3D",
			ScrapeInterval:  54 * time.Second,
			ScrapeTimeout:   5 * time.Second,
			HonorLabels:     true,
			HonorTimestamps: false,
			DenyRedirects:   true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar",
				},
				{
					Name:  "__metrics_path__",
					Value: "/foo/bar",
				},
				{
					Name:  "__param_p",
					Value: "x&y",
				},
				{
					Name:  "__scheme__",
					Value: "https",
				},
				{
					Name:  "__scrape_interval__",
					Value: "54s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "5s",
				},
				{
					Name:  "instance",
					Value: "foo.bar:443",
				},
				{
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "x",
					Value: "y",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			ProxyURL:        proxy.MustNewURL("http://foo.bar"),
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:       "https://aaa:443/foo/bar?p=x%26y&p=%3D",
			ScrapeInterval:  54 * time.Second,
			ScrapeTimeout:   5 * time.Second,
			HonorLabels:     true,
			HonorTimestamps: false,
			DenyRedirects:   true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "aaa",
				},
				{
					Name:  "__metrics_path__",
					Value: "/foo/bar",
				},
				{
					Name:  "__param_p",
					Value: "x&y",
				},
				{
					Name:  "__scheme__",
					Value: "https",
				},
				{
					Name:  "__scrape_interval__",
					Value: "54s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "5s",
				},
				{
					Name:  "instance",
					Value: "aaa:443",
				},
				{
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "x",
					Value: "y",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			ProxyURL:        proxy.MustNewURL("http://foo.bar"),
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:       "http://1.2.3.4:80/metrics",
			ScrapeInterval:  8 * time.Second,
			ScrapeTimeout:   8 * time.Second,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "1.2.3.4",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "8s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "8s",
				},
				{
					Name:  "instance",
					Value: "1.2.3.4:80",
				},
				{
					Name:  "job",
					Value: "qwer",
				},
			},
			AuthConfig: &promauth.Config{
				TLSServerName:         "foobar",
				TLSInsecureSkipVerify: true,
			},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "qwer",
		},
		{
			ScrapeURL:       "http://foobar:80/metrics",
			ScrapeInterval:  8 * time.Second,
			ScrapeTimeout:   8 * time.Second,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foobar",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "8s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "8s",
				},
				{
					Name:  "instance",
					Value: "foobar:80",
				},
				{
					Name:  "job",
					Value: "asdf",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "asdf",
		},
	})
	f(`
scrape_configs:
- job_name: foo
  relabel_configs:
  - source_labels: [__scheme__, __address__]
    separator: "://"
    target_label: __tmp_url
  - source_labels: [__tmp_url, __metrics_path__]
    separator: ""
    target_label: url
  - action: labeldrop
    regex: "job|__tmp_.+"
  - action: drop
    source_labels: [__address__]
    regex: "drop-.*"
  - action: keep
    source_labels: [__param_x]
    regex: keep_me
  - action: labelkeep
    regex: "__.*|url"
  - action: labelmap
    regex: "(url)"
    replacement: "prefix:${1}"
  - action: hashmod
    modulus: 123
    source_labels: [__address__]
    target_label: hash
  - action: replace
    source_labels: [__address__]
    target_label: foobar
    replacement: ""
  params:
    x: [keep_me]
  static_configs:
  - targets: ["foo.bar:1234", "drop-this-target"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics?x=keep_me",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__param_x",
					Value: "keep_me",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "hash",
					Value: "82",
				},
				{
					Name:  "instance",
					Value: "foo.bar:1234",
				},
				{
					Name:  "prefix:url",
					Value: "http://foo.bar:1234/metrics",
				},
				{
					Name:  "url",
					Value: "http://foo.bar:1234/metrics",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
	})
	f(`
scrape_configs:
- job_name: foo
  scheme: https
  relabel_configs:
  - action: replace
    source_labels: [non-existing-label]
    target_label: instance
    replacement: fake.addr
  - action: replace
    source_labels: [__address__]
    target_label: foobar
    regex: "missing-regex"
    replacement: aaabbb
  - action: replace
    source_labels: [__scheme__]
    target_label: job
  - action: replace
    source_labels: [__scheme__]
    target_label: __scheme__
    replacement: mailto
  - target_label: __metrics_path__
    replacement: /abc.de
  - target_label: __param_a
    replacement: b
  static_configs:
  - targets: ["foo.bar:1234"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "mailto://foo.bar:1234/abc.de?a=b",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "__metrics_path__",
					Value: "/abc.de",
				},
				{
					Name:  "__param_a",
					Value: "b",
				},
				{
					Name:  "__scheme__",
					Value: "mailto",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "instance",
					Value: "fake.addr",
				},
				{
					Name:  "job",
					Value: "https",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
	})
	f(`
scrape_configs:
- job_name: foo
  scheme: https
  relabel_configs:
  - action: keep
    source_labels: [__address__]
    regex: "foo\\.bar:.*"
  - action: hashmod
    source_labels: [job]
    modulus: 4
    target_label: job
  - action: labeldrop
    regex: "non-matching-regex"
  - action: labelkeep
    regex: "job|__address__"
  - action: labeldrop
    regex: ""
  static_configs:
  - targets: ["foo.bar:1234", "xyz"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "instance",
					Value: "foo.bar:1234",
				},
				{
					Name:  "job",
					Value: "3",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
	})

	f(`
scrape_configs:
- job_name: foo
  metric_relabel_configs:
  - source_labels: [foo]
    target_label: abc
  static_configs:
  - targets: ["foo.bar:1234"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "instance",
					Value: "foo.bar:1234",
				},
				{
					Name:  "job",
					Value: "foo",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			MetricRelabelConfigs: mustParseRelabelConfigs(`
- source_labels: [foo]
  target_label: abc
`),
			jobNameOriginal: "foo",
		},
	})
	f(`
scrape_configs:
- job_name: foo
  static_configs:
  - targets: ["foo.bar:1234"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "instance",
					Value: "foo.bar:1234",
				},
				{
					Name:  "job",
					Value: "foo",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
	})
	f(`
scrape_configs:
- job_name: foo
  static_configs:
  - targets: ["foo.bar:1234"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "instance",
					Value: "foo.bar:1234",
				},
				{
					Name:  "job",
					Value: "foo",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
	})
	f(`
global:
  external_labels:
    job: foobar
    foo: xx
    q: qwe
    __address__: aaasdf
    __param_a: jlfd
scrape_configs:
- job_name: aaa
  params:
    a: [b, xy]
  static_configs:
  - targets: ["a"]
    labels:
      foo: bar
      __param_a: c
      __address__: pp
      job: yyy
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://pp:80/metrics?a=c&a=xy",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "pp",
				},
				{
					Name:  "__metrics_path__",
					Value: "/metrics",
				},
				{
					Name:  "__param_a",
					Value: "c",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "foo",
					Value: "bar",
				},
				{
					Name:  "instance",
					Value: "pp:80",
				},
				{
					Name:  "job",
					Value: "yyy",
				},
				{
					Name:  "q",
					Value: "qwe",
				},
			},
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "aaa",
		},
	})
	f(`
scrape_configs:
  - job_name: 'snmp'
    sample_limit: 100
    disable_keepalive: true
    disable_compression: true
    scrape_align_interval: 1s
    scrape_offset: 0.5s
    static_configs:
      - targets:
        - 192.168.1.2  # SNMP device.
    metrics_path: /snmp
    params:
      module: [if_mib]
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: 127.0.0.1:9116  # The SNMP exporter's real hostname:port.
      - target_label: __series_limit__
        replacement: 1234
      - target_label: __stream_parse__
        replacement: true
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://127.0.0.1:9116/snmp?module=if_mib&target=192.168.1.2",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "127.0.0.1:9116",
				},
				{
					Name:  "__metrics_path__",
					Value: "/snmp",
				},
				{
					Name:  "__param_module",
					Value: "if_mib",
				},
				{
					Name:  "__param_target",
					Value: "192.168.1.2",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "__series_limit__",
					Value: "1234",
				},
				{
					Name:  "__stream_parse__",
					Value: "true",
				},
				{
					Name:  "instance",
					Value: "192.168.1.2",
				},
				{
					Name:  "job",
					Value: "snmp",
				},
			},
			AuthConfig:          &promauth.Config{},
			ProxyAuthConfig:     &promauth.Config{},
			SampleLimit:         100,
			DisableKeepAlive:    true,
			DisableCompression:  true,
			StreamParse:         true,
			ScrapeAlignInterval: time.Second,
			ScrapeOffset:        500 * time.Millisecond,
			SeriesLimit:         1234,
			jobNameOriginal:     "snmp",
		},
	})
	f(`
scrape_configs:
- job_name: path wo slash
  static_configs: 
  - targets: ["foo.bar:1234"]
  relabel_configs:
  - replacement: metricspath
    target_label: __metrics_path__
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metricspath",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorTimestamps: true,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "__metrics_path__",
					Value: "metricspath",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name:  "__scrape_interval__",
					Value: "1m0s",
				},
				{
					Name:  "__scrape_timeout__",
					Value: "10s",
				},
				{
					Name:  "instance",
					Value: "foo.bar:1234",
				},
				{
					Name:  "job",
					Value: "path wo slash",
				},
			},
			jobNameOriginal: "path wo slash",
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
		},
	})
}

func equalStaticConfigForScrapeWorks(a, b []*ScrapeWork) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		keyA := a[i].key()
		keyB := b[i].key()
		if keyA != keyB {
			return false
		}
	}
	return true
}
