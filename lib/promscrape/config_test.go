package promscrape

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/gce"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
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

func TestGetClusterMemberNumsForScrapeWork(t *testing.T) {
	f := func(key string, membersCount, replicationFactor int, expectedMemberNums []int) {
		t.Helper()
		memberNums := getClusterMemberNumsForScrapeWork(key, membersCount, replicationFactor)
		if !reflect.DeepEqual(memberNums, expectedMemberNums) {
			t.Fatalf("unexpected memberNums; got %d; want %d", memberNums, expectedMemberNums)
		}
	}
	// Disabled clustering
	f("foo", 0, 0, []int{0})
	f("foo", 0, 0, []int{0})

	// A cluster with 2 nodes with disabled replication
	f("baz", 2, 0, []int{0})
	f("foo", 2, 0, []int{1})

	// A cluster with 2 nodes with replicationFactor=2
	f("baz", 2, 2, []int{0, 1})
	f("foo", 2, 2, []int{1, 0})

	// A cluster with 3 nodes with replicationFactor=2
	f("abc", 3, 2, []int{0, 1})
	f("bar", 3, 2, []int{1, 2})
	f("foo", 3, 2, []int{2, 0})
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
	cfg, err := loadConfig("testdata/prometheus.yml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if cfg == nil {
		t.Fatalf("expecting non-nil config")
	}

	cfg, err = loadConfig("testdata/prometheus-with-scrape-config-files.yml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if cfg == nil {
		t.Fatalf("expecting non-nil config")
	}

	// Try loading non-existing file
	cfg, err = loadConfig("testdata/non-existing-file")
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if cfg != nil {
		t.Fatalf("unexpected non-nil config: %#v", cfg)
	}

	// Try loading invalid file
	cfg, err = loadConfig("testdata/file_sd_1.yml")
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if cfg != nil {
		t.Fatalf("unexpected non-nil config: %#v", cfg)
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
	if err := cfg.parseData([]byte(data), "sss"); err != nil {
		t.Fatalf("cannot parase data: %s", err)
	}
	sws := cfg.getStaticScrapeWork()
	swsExpected := []*ScrapeWork{
		{
			ScrapeURL:      "http://host1/metric/path1?x=y",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host1:80",
				"job":      "abc",
			}),
			jobNameOriginal: "abc",
		},
		{
			ScrapeURL:      "https://host2/metric/path2?x=y",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host2:443",
				"job":      "abc",
			}),
			jobNameOriginal: "abc",
		},
		{
			ScrapeURL:      "http://host3:1234/metric/path3?arg1=value1&x=y",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host3:1234",
				"job":      "abc",
			}),
			jobNameOriginal: "abc",
		},
		{
			ScrapeURL:      "https://host4:1234/foo/bar?x=y",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host4:1234",
				"job":      "abc",
			}),
			jobNameOriginal: "abc",
		},
	}
	checkEqualScrapeWorks(t, sws, swsExpected)
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
	if err := cfg.parseData([]byte(data), "sss"); err != nil {
		t.Fatalf("cannot parase data: %s", err)
	}
	sws := cfg.getStaticScrapeWork()
	swsExpected := []*ScrapeWork{{
		ScrapeURL:      "http://black:9115/probe?module=dns_udp_example&target=8.8.8.8",
		ScrapeInterval: defaultScrapeInterval,
		ScrapeTimeout:  defaultScrapeTimeout,
		MaxScrapeSize:  maxScrapeSize.N,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"instance": "8.8.8.8",
			"job":      "blackbox",
		}),
		jobNameOriginal: "blackbox",
	}}
	checkEqualScrapeWorks(t, sws, swsExpected)
}

func TestGetFileSDScrapeWork(t *testing.T) {
	data := `
scrape_configs:
- job_name: foo
  file_sd_configs:
  - files: [testdata/file_sd.json]
`
	var cfg Config
	if err := cfg.parseData([]byte(data), "sss"); err != nil {
		t.Fatalf("cannot parase data: %s", err)
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
	if err := cfgNew.parseData([]byte(dataNew), "sss"); err != nil {
		t.Fatalf("cannot parse data: %s", err)
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
	if err := cfg.parseData([]byte(data), "sss"); err != nil {
		t.Fatalf("cannot parse data: %s", err)
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
	if err := cfg.parseData([]byte(data), "sss"); err != nil {
		t.Fatalf("cannot parse data: %s", err)
	}
	sws = cfg.getFileSDScrapeWork(swsNew)
	if len(sws) != 0 {
		t.Fatalf("unexpected non-empty sws:\n%#v", sws)
	}
}

func getFileSDScrapeWork(data []byte, path string) ([]*ScrapeWork, error) {
	var cfg Config
	if err := cfg.parseData(data, path); err != nil {
		return nil, fmt.Errorf("cannot parse data: %w", err)
	}
	return cfg.getFileSDScrapeWork(nil), nil
}

func getStaticScrapeWork(data []byte, path string) ([]*ScrapeWork, error) {
	var cfg Config
	if err := cfg.parseData(data, path); err != nil {
		return nil, fmt.Errorf("cannot parse data: %w", err)
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

	// yaml with unsupported fields
	f(`foo: bar`)
	f(`
scrape_configs:
- foo: bar
`)

	// invalid scrape_config_files contents
	f(`
scrape_config_files:
- job_name: aa
  static_configs:
  - targets: ["s"]
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
}

// String returns human-readable representation for sw.
func (sw *ScrapeWork) String() string {
	return stringsutil.JSONString(sw.key())
}

func TestGetFileSDScrapeWorkSuccess(t *testing.T) {
	f := func(data string, expectedSws []*ScrapeWork) {
		t.Helper()
		sws, err := getFileSDScrapeWork([]byte(data), "non-existing-file")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		checkEqualScrapeWorks(t, sws, expectedSws)
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
			ScrapeURL:      "http://host1/abc/de",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host1:80",
				"job":      "foo",
				"qwe":      "rty",
			}),
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:      "http://host2/abc/de",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "host2:80",
				"job":      "foo",
				"qwe":      "rty",
			}),
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:      "http://localhost:9090/abc/de",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "localhost:9090",
				"job":      "foo",
				"yml":      "test",
			}),
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
		checkEqualScrapeWorks(t, sws, expectedSws)
	}
	f(``, nil)

	// Scrape config with missing modulus for action=hashmod in relabel_configs must be skipped
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    source_labels: [foo]
    target_label: bar
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{})

	// Scrape config with invalid action in relabel_configs must be skipped
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: foobar
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{})

	// Scrape config with missing source_labels for action=keep in relabel_configs must be skipped
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: keep
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{})

	// Scrape config with missing source_labels for action=drop in relabel_configs must be skipped
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: drop
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{})

	// Scrape config with missing source_labels for action=hashmod in relabel_configs must be skipped
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    target_label: bar
    modulus: 123
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{})

	// Scrape config with missing target for action=hashmod in relabel_configs must be skipped
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: hashmod
    source_labels: [foo]
    modulus: 123
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{})

	// Scrape config with invalid regex in relabel_configs must be skipped
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - regex: "("
    source_labels: [foo]
    target_label: bar
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{})

	// Scrape config with missing target_label for action=replace in relabel_configs must be skipped
	f(`
scrape_configs:
- job_name: aa
  relabel_configs:
  - action: replace
    source_labels: [foo]
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{})

	// Scrape config with both `authorization` and `bearer_token` set must be skipped
	f(`
scrape_configs:
- job_name: x
  authorization:
    credentials: foobar
  bearer_token: foo
  static_configs:
  - targets: ["a"]
`, []*ScrapeWork{})

	// Scrape config with both `bearer_token` and `bearer_token_file` set must be skipped
	f(`
scrape_configs:
- job_name: x
  bearer_token: foo
  bearer_token_file: bar
  static_configs:
  - targets: ["a"]
`, []*ScrapeWork{})

	// Scrape config with both `basic_auth` and `bearer_token` set must be skipped
	f(`
scrape_configs:
- job_name: x
  bearer_token: foo
  basic_auth:
    username: foo
    password: bar
  static_configs:
  - targets: ["a"]
`, []*ScrapeWork{})

	// Scrape config with both `authorization` and `basic_auth` set must be skipped
	f(`
scrape_configs:
- job_name: x
  authorization:
    credentials: foobar
  basic_auth:
    username: foobar
  static_configs:
  - targets: ["a"]
`, []*ScrapeWork{})

	// Scrape config with invalid scheme must be skipped
	f(`
scrape_configs:
- job_name: x
  scheme: asdf
  static_configs:
  - targets: ["foo"]
`, []*ScrapeWork{})

	// Scrape config with missing job_name must be skipped
	f(`
scrape_configs:
- static_configs:
  - targets: ["foo"]
`, []*ScrapeWork{})

	// Scrape config with missing username in `basic_auth` must be skipped
	f(`
scrape_configs:
- job_name: x
  basic_auth:
    password: sss
  static_configs:
  - targets: ["a"]
`, []*ScrapeWork{})

	// Scrape config with both password and password_file set in `basic_auth` must be skipped
	f(`
scrape_configs:
- job_name: x
  basic_auth:
    username: foobar
    password: sss
    password_file: sdfdf
  static_configs:
  - targets: ["a"]
`, []*ScrapeWork{})

	// Scrape config with invalid ca_file must be properly parsed, since ca_file may become valid later
	f(`
scrape_configs:
- job_name: aa
  tls_config:
    ca_file: testdata/prometheus.yml
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{
		{
			ScrapeURL:      "http://s/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "s:80",
				"job":      "aa",
			}),
			jobNameOriginal: "aa",
		},
	})

	// Scrape config with non-existing ca_file must be properly parsed, since the ca_file can become valid later
	f(`
scrape_configs:
- job_name: aa
  tls_config:
    ca_file: non/extising/file
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{
		{
			ScrapeURL:      "http://s/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "s:80",
				"job":      "aa",
			}),
			jobNameOriginal: "aa",
		},
	})

	// Scrape config with non-existing cert_file must be properly parsed, since the cert_file can become valid later
	f(`
scrape_configs:
- job_name: aa
  tls_config:
    cert_file: non/extising/file
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{
		{
			ScrapeURL:      "http://s/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "s:80",
				"job":      "aa",
			}),
			jobNameOriginal: "aa",
		},
	})

	// Scrape config with non-existing key_file must be properly parsed, since the key_file can become valid later
	f(`
scrape_configs:
- job_name: aa
  tls_config:
    key_file: non/extising/file
  static_configs:
  - targets: ["s"]
`, []*ScrapeWork{
		{
			ScrapeURL:      "http://s/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "s:80",
				"job":      "aa",
			}),
			jobNameOriginal: "aa",
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
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
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
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"datacenter": "foobar",
				"jobs":       "xxx",
			}),
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
			ScrapeURL:       "https://foo.bar/foo/bar?p=x%26y&p=%3D",
			ScrapeInterval:  54 * time.Second,
			ScrapeTimeout:   5 * time.Second,
			MaxScrapeSize:   maxScrapeSize.N,
			HonorLabels:     true,
			HonorTimestamps: true,
			DenyRedirects:   true,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:443",
				"job":      "foo",
				"x":        "y",
			}),
			ProxyURL:        proxy.MustNewURL("http://foo.bar"),
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:       "https://aaa/foo/bar?p=x%26y&p=%3D",
			ScrapeInterval:  54 * time.Second,
			ScrapeTimeout:   5 * time.Second,
			MaxScrapeSize:   maxScrapeSize.N,
			HonorLabels:     true,
			HonorTimestamps: true,
			DenyRedirects:   true,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "aaa:443",
				"job":      "foo",
				"x":        "y",
			}),
			ProxyURL:        proxy.MustNewURL("http://foo.bar"),
			jobNameOriginal: "foo",
		},
		{
			ScrapeURL:      "http://1.2.3.4/metrics",
			ScrapeInterval: 8 * time.Second,
			ScrapeTimeout:  8 * time.Second,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "1.2.3.4:80",
				"job":      "qwer",
			}),
			jobNameOriginal: "qwer",
		},
		{
			ScrapeURL:      "http://foobar/metrics",
			ScrapeInterval: 8 * time.Second,
			ScrapeTimeout:  8 * time.Second,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foobar:80",
				"job":      "asdf",
			}),
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
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"hash":       "82",
				"instance":   "foo.bar:1234",
				"prefix:url": "http://foo.bar:1234/metrics",
				"url":        "http://foo.bar:1234/metrics",
			}),
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
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "fake.addr",
				"job":      "https",
			}),
			jobNameOriginal: "foo",
		},
	})
	f(`
scrape_configs:
- job_name: foo
  scheme: https
  max_scrape_size: 1
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
			MaxScrapeSize:  1,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "3",
			}),
			jobNameOriginal: "foo",
		},
	})

	f(`
scrape_configs:
- job_name: foo
  max_scrape_size: 8MiB
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
			MaxScrapeSize:  8 * 1024 * 1024,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
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
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
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
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
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
			ScrapeURL:      "http://pp/metrics?a=c&a=xy",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
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
			jobNameOriginal: "aaa",
		},
	})

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
      - target_label: __sample_limit__
        replacement: 5678
      - target_label: __stream_parse__
        replacement: true
`, []*ScrapeWork{
		{
			ScrapeURL:      "http://127.0.0.1:9116/snmp?module=if_mib&target=192.168.1.2",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "192.168.1.2",
				"job":      "snmp",
			}),
			SampleLimit:         5678,
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
  enable_compression: false
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
			MaxScrapeSize:  maxScrapeSize.N,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "path wo slash",
			}),
			DisableCompression: true,
			jobNameOriginal:    "path wo slash",
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
			MaxScrapeSize:       maxScrapeSize.N,
			NoStaleMarkers:      true,
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
			jobNameOriginal: "foo",
		},
	})

	defaultSeriesLimitPerTarget := *seriesLimitPerTarget
	*seriesLimitPerTarget = 1e3
	f(`
scrape_configs:
- job_name: foo
  series_limit: 0
  static_configs:
  - targets: ["foo.bar:1234"]
`, []*ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			MaxScrapeSize:   maxScrapeSize.N,
			jobNameOriginal: "foo",
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"instance": "foo.bar:1234",
				"job":      "foo",
			}),
			SeriesLimit: 0,
		},
	})
	*seriesLimitPerTarget = defaultSeriesLimitPerTarget
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

func checkEqualScrapeWorks(t *testing.T, got, want []*ScrapeWork) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unexpected number of ScrapeWork items; got %d; want %d", len(got), len(want))
	}
	for i := range got {
		gotItem := *got[i]
		wantItem := want[i]

		// Zero fields with internal state before comparing the items.
		gotItem.ProxyAuthConfig = nil
		gotItem.AuthConfig = nil
		gotItem.OriginalLabels = nil
		gotItem.RelabelConfigs = nil
		gotItem.MetricRelabelConfigs = nil

		if !reflect.DeepEqual(&gotItem, wantItem) {
			t.Fatalf("unexpected scrapeWork at position %d out of %d;\ngot\n%#v\nwant\n%#v", i, len(got), &gotItem, wantItem)
		}
	}
}
