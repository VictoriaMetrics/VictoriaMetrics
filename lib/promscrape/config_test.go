package promscrape

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/gce"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

func TestMergeLabels(t *testing.T) {
	f := func(swc *scrapeWorkConfig, target string, extraLabelsMap, metaLabelsMap map[string]string, resultExpected string) {
		t.Helper()
		extraLabels := promutils.NewLabelsFromMap(extraLabelsMap)
		metaLabels := promutils.NewLabelsFromMap(metaLabelsMap)
		labels := promutils.NewLabels(0)
		mergeLabels(labels, swc, target, extraLabels, metaLabels)
		result := labels.String()
		if result != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}
	f(&scrapeWorkConfig{}, "foo", nil, nil, `{__address__="foo",__metrics_path__="",__scheme__="",__scrape_interval__="",__scrape_timeout__="",job=""}`)
	f(&scrapeWorkConfig{}, "foo", map[string]string{"foo": "bar"}, nil, `{__address__="foo",__metrics_path__="",__scheme__="",__scrape_interval__="",__scrape_timeout__="",foo="bar",job=""}`)
	f(&scrapeWorkConfig{}, "foo", map[string]string{"job": "bar"}, nil, `{__address__="foo",__metrics_path__="",__scheme__="",__scrape_interval__="",__scrape_timeout__="",job="bar"}`)
	f(&scrapeWorkConfig{
		jobName:              "xyz",
		scheme:               "https",
		metricsPath:          "/foo/bar",
		scrapeIntervalString: "15s",
		scrapeTimeoutString:  "10s",
	}, "foo", nil, nil, `{__address__="foo",__metrics_path__="/foo/bar",__scheme__="https",__scrape_interval__="15s",__scrape_timeout__="10s",job="xyz"}`)
	f(&scrapeWorkConfig{
		jobName:     "xyz",
		scheme:      "https",
		metricsPath: "/foo/bar",
	}, "foo", map[string]string{
		"job": "extra_job",
		"foo": "extra_foo",
		"a":   "xyz",
	}, map[string]string{
		"__meta_x": "y",
	}, `{__address__="foo",__meta_x="y",__metrics_path__="/foo/bar",__scheme__="https",__scrape_interval__="",__scrape_timeout__="",a="xyz",foo="extra_foo",job="extra_job"}`)
}

func TestScrapeConfigUnmarshalMarshal(t *testing.T) {
	f := func(data string) {
		t.Helper()
		var cfg Config
		data = strings.TrimSpace(data)
		if err := cfg.unmarshal([]byte(data), true); err != nil {
			t.Fatalf("parse error: %s\ndata:\n%s", err, data)
		}
		resultData := string(cfg.marshal())
		result := strings.TrimSpace(resultData)
		if result != data {
			t.Fatalf("unexpected marshaled config:\ngot\n%s\nwant\n%s", result, data)
		}
	}
	f(`
global:
  scrape_interval: 10s
`)
	f(`
scrape_config_files:
- foo
- bar
`)
	f(`
scrape_configs:
- job_name: foo
  scrape_timeout: 1.5s
  static_configs:
  - targets:
    - foo
    - bar
    labels:
      foo: bar
`)
	f(`
scrape_configs:
- job_name: foo
  honor_labels: true
  honor_timestamps: true
  scheme: https
  params:
    foo:
    - x
  authorization:
    type: foobar
  headers:
  - 'TenantID: fooBar'
  - 'X: y:z'
  relabel_configs:
  - source_labels: [abc]
  static_configs:
  - targets:
    - foo
  scrape_align_interval: 1h30m0s
  proxy_bearer_token_file: file.txt
  proxy_headers:
  - 'My-Auth-Header: top-secret'
`)

}

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
	f("foo", 0, 0, 1, false)

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

func TestAddressWithFullURL(t *testing.T) {
	data := `
scrape_configs:
- job_name: abc
  metrics_path: /foo/bar
  scheme: https
  params:
    x: [y]
  static_configs:
  - targets:
    # the following targets are scraped by the provided urls
    - 'http://host1/metric/path1'
    - 'https://host2/metric/path2'
    - 'http://host3:1234/metric/path3?arg1=value1'
    # the following target is scraped by <scheme>://host4:1234<metrics_path>
    - host4:1234
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
	swsExpected := []*ScrapeWork{
		{
			ScrapeURL:      "http://host1:80/metric/path1?x=y",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host1:80",
				"job":      "abc",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "abc",
		},
		{
			ScrapeURL:      "https://host2:443/metric/path2?x=y",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host2:443",
				"job":      "abc",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "abc",
		},
		{
			ScrapeURL:      "http://host3:1234/metric/path3?arg1=value1&x=y",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host3:1234",
				"job":      "abc",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "abc",
		},
		{
			ScrapeURL:      "https://host4:1234/foo/bar?x=y",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host4:1234",
				"job":      "abc",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "abc",
		},
	}
	if !reflect.DeepEqual(sws, swsExpected) {
		t.Fatalf("unexpected scrapeWork;\ngot\n%#v\nwant\n%#v", sws, swsExpected)
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
		ScrapeURL:      "http://black:9115/probe?module=dns_udp_example&target=8.8.8.8",
		ScrapeInterval: defaultScrapeInterval,
		ScrapeTimeout:  defaultScrapeTimeout,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"instance": "8.8.8.8",
			"job":      "blackbox",
		}),
		AuthConfig:      &promauth.Config{},
		ProxyAuthConfig: &promauth.Config{},
		jobNameOriginal: "blackbox",
	}}
	if !reflect.DeepEqual(sws, swsExpected) {
		t.Fatalf("unexpected scrapeWork;\ngot\n%#v\nwant\n%#v", sws, swsExpected)
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
	tests := []struct {
		name      string
		data      string
		expectSws []*ScrapeWork
		wantErr   bool
	}{
		{
			name:    "Incorrect yaml",
			data:    `foo bar baz`,
			wantErr: true,
		},
		{
			name: "Missing job_name",
			data: `
scrape_configs:
- static_configs:
  - targets: ["foo"]
`,
		},
		{
			name: "Duplicate job_name",
			data: `
scrape_configs:
- job_name: foo
  static_configs:
  - targets: ["foo"]
- job_name: foo
  static_configs:
  - targets: ["bar"]
`,
			wantErr: true,
		},
		{
			name: "Invalid scheme",
			data: `
scrape_configs:
- job_name: x
  scheme: asdf
  static_configs:
  - targets: ["foo"]
`,
		},
		{
			name: "Missing username in `basic_auth`",
			data: `
scrape_configs:
- job_name: x
  basic_auth:
    password: sss
  static_configs:
  - targets: ["a"]
`,
		},
		{
			name: "Both password and password_file set in `basic_auth`",
			data: `
scrape_configs:
- job_name: x
  basic_auth:
    username: foobar
    password: sss
    password_file: sdfdf
  static_configs:
  - targets: ["a"]
`,
		},
		{
			name: "Invalid password_file set in `basic_auth`",
			data: `
scrape_configs:
- job_name: x
  basic_auth:
    username: foobar
    password_file: ['foobar']
  static_configs:
  - targets: ["a"]
`,
			wantErr: true,
		},
		{
			name: "Both `bearer_token` and `bearer_token_file` are set",
			data: `
scrape_configs:
- job_name: x
  bearer_token: foo
  bearer_token_file: bar
  static_configs:
  - targets: ["a"]
`,
		},
		{
			name: "Both `basic_auth` and `bearer_token` are set",
			data: `
scrape_configs:
- job_name: x
  bearer_token: foo
  basic_auth:
    username: foo
    password: bar
  static_configs:
  - targets: ["a"]
`,
		},
		{
			name: "Both `authorization` and `basic_auth` are set",
			data: `
scrape_configs:
- job_name: x
  authorization:
    credentials: foobar
  basic_auth:
    username: foobar
  static_configs:
  - targets: ["a"]
`,
		},
		{
			name: "Both `authorization` and `bearer_token` are set",
			data: `
scrape_configs:
- job_name: x
  authorization:
    credentials: foobar
  bearer_token: foo
  static_configs:
  - targets: ["a"]
`,
		},
		{
			name: "Invalid `bearer_token_file`",
			data: `
scrape_configs:
- job_name: x
  bearer_token_file: [foobar]
  static_configs:
  - targets: ["a"]
`,
			wantErr: true,
		},
		{
			name: "non-existing ca_file",
			data: `
scrape_configs:
- job_name: aa
  tls_config:
    ca_file: non/extising/file
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "invalid ca_file",
			data: `
scrape_configs:
- job_name: aa
  tls_config:
    ca_file: testdata/prometheus.yml
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "non-existing cert_file",
			data: `
scrape_configs:
- job_name: aa
  tls_config:
    cert_file: non/extising/file
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "non-existing key_file",
			data: `
scrape_configs:
- job_name: aa
  tls_config:
    key_file: non/extising/file
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Invalid regex in relabel_configs",
			data: `
scrape_configs:
- job_name: aa
  relabel_configs:
  - regex: "("
    source_labels: [foo]
    target_label: bar
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Missing target_label for action=replace in relabel_configs",
			data: `
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: replace
    source_labels: [foo]
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Missing source_labels for action=keep in relabel_configs",
			data: `
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: keep
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Missing source_labels for action=drop in relabel_configs",
			data: `
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: drop
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Missing source_labels for action=hashmod in relabel_configs",
			data: `
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    target_label: bar
    modulus: 123
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Missing target for action=hashmod in relabel_configs",
			data: `
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    source_labels: [foo]
    modulus: 123
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Missing modulus for action=hashmod in relabel_configs",
			data: `
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    source_labels: [foo]
    target_label: bar
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Invalid action in relabel_configs",
			data: `
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: foobar
  static_configs:
  - targets: ["s"]
`,
		},
		{
			name: "Invalid scrape_config_files contents",
			data: `
scrape_config_files:
- job_name: aa
  static_configs:
  - targets: ["s"]
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sws, err := getStaticScrapeWork([]byte(tt.data), "non-existing-file")
			if (err != nil) != tt.wantErr {
				t.Fatalf("failed to test %s, wantErr %v, got %v", tt.name, tt.wantErr, err)
				return
			}
			if len(sws) != len(tt.expectSws) {
				t.Errorf("failed to test %s, want %d sws, got %d", tt.name, len(tt.expectSws), len(sws))
			}
		})
	}
}

func resetNonEssentialFields(sws []*ScrapeWork) {
	for _, sw := range sws {
		sw.OriginalLabels = nil
		sw.RelabelConfigs = nil
		sw.MetricRelabelConfigs = nil
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
			ScrapeURL:      "http://host1:80/abc/de",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host1:80",
				"job":      "foo",
				"qwe":      "rty",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:      "http://host2:80/abc/de",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host2:80",
				"job":      "foo",
				"qwe":      "rty",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:      "http://localhost:9090/abc/de",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "localhost:9090",
				"job":      "foo",
				"yml":      "test",
			}),
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
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
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
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"datacenter": "foobar",
				"jobs":       "xxx",
			}),
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
  honor_timestamps: true
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
			HonorTimestamps: true,
			DenyRedirects:   true,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:443",
				"job":      "foo",
				"x":        "y",
			}),
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
			HonorTimestamps: true,
			DenyRedirects:   true,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "aaa:443",
				"job":      "foo",
				"x":        "y",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			ProxyURL:        proxy.MustNewURL("http://foo.bar"),
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:      "http://1.2.3.4:80/metrics",
			ScrapeInterval: 8 * time.Second,
			ScrapeTimeout:  8 * time.Second,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "1.2.3.4:80",
				"job":      "qwer",
			}),
			AuthConfig: &promauth.Config{
				TLSServerName:         "foobar",
				TLSInsecureSkipVerify: true,
			},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "qwer",
		},
		{
			ScrapeURL:      "http://foobar:80/metrics",
			ScrapeInterval: 8 * time.Second,
			ScrapeTimeout:  8 * time.Second,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foobar:80",
				"job":      "asdf",
			}),
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
			ScrapeURL:      "http://foo.bar:1234/metrics?x=keep_me",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"hash":       "82",
				"instance":   "foo.bar:1234",
				"prefix:url": "http://foo.bar:1234/metrics",
				"url":        "http://foo.bar:1234/metrics",
			}),
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
			ScrapeURL:      "mailto://foo.bar:1234/abc.de?a=b",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "fake.addr",
				"job":      "https",
			}),
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
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "3",
			}),
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
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
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
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
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
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
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
			ScrapeURL:      "http://pp:80/metrics?a=c&a=xy",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"foo":      "bar",
				"instance": "pp:80",
				"job":      "yyy",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"__address__": "aaasdf",
				"__param_a":   "jlfd",
				"foo":         "xx",
				"job":         "foobar",
				"q":           "qwe",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "aaa",
		},
	})

	opts := &promauth.Options{
		Headers: []string{"My-Auth: foo-Bar"},
	}
	ac, err := opts.NewConfig()
	if err != nil {
		t.Fatalf("unexpected error when creating promauth.Config: %s", err)
	}
	opts = &promauth.Options{
		Headers: []string{"Foo:bar"},
	}
	proxyAC, err := opts.NewConfig()
	if err != nil {
		t.Fatalf("unexpected error when creating promauth.Config for proxy: %s", err)
	}
	f(`
scrape_configs:
  - job_name: 'snmp'
    sample_limit: 100
    disable_keepalive: true
    disable_compression: true
    headers:
    - "My-Auth: foo-Bar"
    proxy_headers:
    - "Foo: bar"
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
			ScrapeURL:      "http://127.0.0.1:9116/snmp?module=if_mib&target=192.168.1.2",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "192.168.1.2",
				"job":      "snmp",
			}),
			AuthConfig:          ac,
			ProxyAuthConfig:     proxyAC,
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
			ScrapeURL:      "http://foo.bar:1234/metricspath",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "path wo slash",
			}),
			jobNameOriginal: "path wo slash",
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
		},
	})
	f(`
global:
  scrape_timeout: 1d
scrape_configs:
- job_name: foo
  scrape_interval: 1w
  scrape_align_interval: 1d
  scrape_offset: 2d
  no_stale_markers: true
  static_configs:
  - targets: ["foo.bar:1234"]
`, []*ScrapeWork{
		{
			ScrapeURL:           "http://foo.bar:1234/metrics",
			ScrapeInterval:      time.Hour * 24 * 7,
			ScrapeTimeout:       time.Hour * 24,
			ScrapeAlignInterval: time.Hour * 24,
			ScrapeOffset:        time.Hour * 24 * 2,
			NoStaleMarkers:      true,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
			AuthConfig:      &promauth.Config{},
			ProxyAuthConfig: &promauth.Config{},
			jobNameOriginal: "foo",
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

func TestScrapeConfigClone(t *testing.T) {
	f := func(sc *ScrapeConfig) {
		t.Helper()
		scCopy := sc.clone()
		scJSON := sc.marshalJSON()
		scCopyJSON := scCopy.marshalJSON()
		if !reflect.DeepEqual(scJSON, scCopyJSON) {
			t.Fatalf("unexpected cloned result:\ngot\n%s\nwant\n%s", scCopyJSON, scJSON)
		}
	}

	f(&ScrapeConfig{})

	var ie promrelabel.IfExpression
	if err := ie.Parse(`{foo=~"bar",baz!="z"}`); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	f(&ScrapeConfig{
		JobName:        "foo",
		ScrapeInterval: promutils.NewDuration(time.Second * 47),
		HonorLabels:    true,
		Params: map[string][]string{
			"foo": {"bar", "baz"},
		},
		HTTPClientConfig: promauth.HTTPClientConfig{
			Authorization: &promauth.Authorization{
				Credentials: promauth.NewSecret("foo"),
			},
			BasicAuth: &promauth.BasicAuthConfig{
				Username: "user_x",
				Password: promauth.NewSecret("pass_x"),
			},
			BearerToken: promauth.NewSecret("zx"),
			OAuth2: &promauth.OAuth2Config{
				ClientSecret: promauth.NewSecret("aa"),
				Scopes:       []string{"foo", "bar"},
				TLSConfig: &promauth.TLSConfig{
					CertFile: "foo",
				},
			},
			TLSConfig: &promauth.TLSConfig{
				KeyFile: "aaa",
			},
		},
		ProxyURL: proxy.MustNewURL("https://foo.bar:3434/assdf/dsfd?sdf=dsf"),
		RelabelConfigs: []promrelabel.RelabelConfig{{
			SourceLabels: []string{"foo", "aaa"},
			Regex: &promrelabel.MultiLineRegex{
				S: "foo\nbar",
			},
			If: &ie,
		}},
		SampleLimit: 10,
		GCESDConfigs: []gce.SDConfig{{
			Project: "foo",
			Zone: gce.ZoneYAML{
				Zones: []string{"a", "b"},
			},
		}},
		StreamParse: true,
		ProxyClientConfig: promauth.ProxyClientConfig{
			BearerTokenFile: "foo",
		},
	})
}
