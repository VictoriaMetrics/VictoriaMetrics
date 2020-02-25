package promscrape

import (
	"crypto/tls"
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

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
	if n := cfg.fileSDConfigsCount(); n != 2 {
		t.Fatalf("unexpected number of `file_sd_configs`; got %d; want %d; cfg:\n%#v", n, 2, cfg)
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

func TestGetFileSDScrapeWork(t *testing.T) {
	data := `
scrape_configs:
- job_name: foo
  file_sd_configs:
  - files: [testdata/file_sd.json]
`
	var cfg Config
	if err := cfg.parse([]byte(data), "sss"); err != nil {
		t.Fatalf("cannot parase data: %s", err)
	}
	sws, err := cfg.getFileSDScrapeWork(nil)
	if err != nil {
		t.Fatalf("cannot obtain `file_sd_config`: %s", err)
	}
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
	if err := cfgNew.parse([]byte(dataNew), "sss"); err != nil {
		t.Fatalf("cannot parse data: %s", err)
	}
	swsNew, err := cfgNew.getFileSDScrapeWork(sws)
	if err != nil {
		t.Fatalf("cannot obtain `file_sd_config`: %s", err)
	}
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
	if err := cfg.parse([]byte(data), "sss"); err != nil {
		t.Fatalf("cannot parse data: %s", err)
	}
	sws, err = cfg.getFileSDScrapeWork(swsNew)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
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
	if err := cfg.parse([]byte(data), "sss"); err != nil {
		t.Fatalf("cannot parse data: %s", err)
	}
	sws, err = cfg.getFileSDScrapeWork(swsNew)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sws) != 0 {
		t.Fatalf("unexpected non-empty sws:\n%#v", sws)
	}
}

func getFileSDScrapeWork(data []byte, path string) ([]ScrapeWork, error) {
	var cfg Config
	if err := cfg.parse(data, path); err != nil {
		return nil, fmt.Errorf("cannot parse data: %s", err)
	}
	return cfg.getFileSDScrapeWork(nil)
}

func getStaticScrapeWork(data []byte, path string) ([]ScrapeWork, error) {
	var cfg Config
	if err := cfg.parse(data, path); err != nil {
		return nil, fmt.Errorf("cannot parse data: %s", err)
	}
	return cfg.getStaticScrapeWork()
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

	// Invalid scheme
	f(`
scrape_configs:
- job_name: x
  scheme: asdf
  static_configs:
  - targets: ["foo"]
`)

	// Empty target
	f(`
scrape_configs:
- job_name: x
  static_configs:
  - targets: ["foo", ""]
`)

	// Invalid url
	f(`
scrape_configs:
- job_name: x
  static_configs:
  - targets: ["a b"]
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
    password_file: /non_existing_file.pass
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

	// Invalid `bearer_token_file`
	f(`
scrape_configs:
- job_name: x
  bearer_token_file: non_existing_file.bearer
  static_configs:
  - targets: ["a"]
`)

	// Clash of external_label with job or instance
	f(`
global:
  external_labels:
    job: foobar
scrape_configs:
- job_name: aaa
  static_configs:
  - targets: ["a"]
`)

	// Clash of external_label with static_configs label
	f(`
global:
  external_labels:
    xxx: foobar
scrape_configs:
- job_name: aaa
  static_configs:
  - targets: ["a"]
    labels:
      xxx: yyy
`)

	// Clash of param with external_labels
	f(`
global:
  external_labels:
    __param_xxx: foobar
scrape_configs:
- job_name: aaa
  params:
    xxx: [abcd]
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
}

func TestGetFileSDScrapeWorkSuccess(t *testing.T) {
	f := func(data string, expectedSws []ScrapeWork) {
		t.Helper()
		sws, err := getFileSDScrapeWork([]byte(data), "non-existing-file")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(sws, expectedSws) {
			t.Fatalf("unexpected scrapeWork; got\n%v\nwant\n%v", sws, expectedSws)
		}
	}
	f(`
scrape_configs:
- job_name: foo
  static_configs:
  - targets: ["xxx"]
`, nil)
	f(`
scrape_configs:
- job_name: foo
  metrics_path: /abc/de
  file_sd_configs:
  - files: ["testdata/file_sd.json", "testdata/file_sd*.yml"]
`, []ScrapeWork{
		{
			ScrapeURL:       "http://host1:80/abc/de",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorLabels:     false,
			HonorTimestamps: false,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "host1",
				},
				{
					Name:  "__meta_filepath",
					Value: "testdata/file_sd.json",
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
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "qwe",
					Value: "rty",
				},
			},
		},
		{
			ScrapeURL:       "http://host2:80/abc/de",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorLabels:     false,
			HonorTimestamps: false,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "host2",
				},
				{
					Name:  "__meta_filepath",
					Value: "testdata/file_sd.json",
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
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "qwe",
					Value: "rty",
				},
			},
		},
		{
			ScrapeURL:       "http://localhost:9090/abc/de",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorLabels:     false,
			HonorTimestamps: false,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "localhost:9090",
				},
				{
					Name:  "__meta_filepath",
					Value: "testdata/file_sd_1.yml",
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
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "yml",
					Value: "test",
				},
			},
		},
	})
}

func TestGetStaticScrapeWorkSuccess(t *testing.T) {
	f := func(data string, expectedSws []ScrapeWork) {
		t.Helper()
		sws, err := getStaticScrapeWork([]byte(data), "non-exsiting-file")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(sws, expectedSws) {
			t.Fatalf("unexpected scrapeWork; got\n%v\nwant\n%v", sws, expectedSws)
		}
	}
	f(``, nil)
	f(`
scrape_configs:
- job_name: foo
  static_configs:
  - targets: ["foo.bar:1234"]
`, []ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorLabels:     false,
			HonorTimestamps: false,
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
					Name:  "job",
					Value: "foo",
				},
			},
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
`, []ScrapeWork{
		{
			ScrapeURL:       "http://foo.bar:1234/metrics",
			ScrapeInterval:  defaultScrapeInterval,
			ScrapeTimeout:   defaultScrapeTimeout,
			HonorLabels:     false,
			HonorTimestamps: false,
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
					Name:  "datacenter",
					Value: "foobar",
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
		},
	})
	f(`
global:
  scrape_interval: 8s
  scrape_timeout: 34s
scrape_configs:
- job_name: foo
  scrape_interval: 543s
  scrape_timeout: 12s
  metrics_path: /foo/bar
  scheme: https
  honor_labels: true
  honor_timestamps: true
  params:
    p: ["x&y", "="]
    xaa:
  bearer_token: xyz
  static_configs:
  - targets: ["foo.bar", "aaa"]
    labels:
      x: y
- job_name: qwer
  basic_auth:
    username: user
    password: pass
  tls_config:
    server_name: foobar
    insecure_skip_verify: true
  static_configs:
  - targets: [1.2.3.4]
`, []ScrapeWork{
		{
			ScrapeURL:       "https://foo.bar:443/foo/bar?p=x%26y&p=%3D",
			ScrapeInterval:  543 * time.Second,
			ScrapeTimeout:   12 * time.Second,
			HonorLabels:     true,
			HonorTimestamps: true,
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
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "x",
					Value: "y",
				},
			},
			Authorization: "Bearer xyz",
		},
		{
			ScrapeURL:       "https://aaa:443/foo/bar?p=x%26y&p=%3D",
			ScrapeInterval:  543 * time.Second,
			ScrapeTimeout:   12 * time.Second,
			HonorLabels:     true,
			HonorTimestamps: true,
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
					Name:  "job",
					Value: "foo",
				},
				{
					Name:  "x",
					Value: "y",
				},
			},
			Authorization: "Bearer xyz",
		},
		{
			ScrapeURL:       "http://1.2.3.4:80/metrics",
			ScrapeInterval:  8 * time.Second,
			ScrapeTimeout:   34 * time.Second,
			HonorLabels:     false,
			HonorTimestamps: false,
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
					Name:  "job",
					Value: "qwer",
				},
			},
			Authorization:         "Basic dXNlcjpwYXNz",
			TLSServerName:         "foobar",
			TLSInsecureSkipVerify: true,
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
`, []ScrapeWork{
		{
			ScrapeURL:      "http://foo.bar:1234/metrics?x=keep_me",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
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
					Name:  "hash",
					Value: "82",
				},
				{
					Name:  "prefix:url",
					Value: "http://foo.bar:1234/metrics",
				},
			},
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
`, []ScrapeWork{
		{
			ScrapeURL:      "mailto://foo.bar:1234/abc.de?a=b",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
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
					Name:  "instance",
					Value: "fake.addr",
				},
				{
					Name:  "job",
					Value: "https",
				},
			},
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
`, []ScrapeWork{
		{
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
			Labels: []prompbmarshal.Label{
				{
					Name:  "__address__",
					Value: "foo.bar:1234",
				},
				{
					Name:  "job",
					Value: "3",
				},
			},
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
`, []ScrapeWork{
		{
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
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
					Name:  "job",
					Value: "foo",
				},
			},
			MetricRelabelConfigs: []promrelabel.ParsedRelabelConfig{
				{
					SourceLabels: []string{"foo"},
					Separator:    ";",
					TargetLabel:  "abc",
					Regex:        defaultRegexForRelabelConfig,
					Replacement:  "$1",
					Action:       "replace",
				},
			},
		},
	})
	f(`
scrape_configs:
- job_name: foo
  basic_auth:
    username: xyz
    password_file: testdata/password.txt
  static_configs:
  - targets: ["foo.bar:1234"]
`, []ScrapeWork{
		{
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
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
					Name:  "job",
					Value: "foo",
				},
			},
			Authorization: "Basic eHl6OnNlY3JldC1wYXNz",
		},
	})
	f(`
scrape_configs:
- job_name: foo
  bearer_token_file: testdata/password.txt
  static_configs:
  - targets: ["foo.bar:1234"]
`, []ScrapeWork{
		{
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
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
					Name:  "job",
					Value: "foo",
				},
			},
			Authorization: "Bearer secret-pass",
		},
	})
	snakeoilCert, err := tls.LoadX509KeyPair("testdata/ssl-cert-snakeoil.pem", "testdata/ssl-cert-snakeoil.key")
	if err != nil {
		t.Fatalf("cannot load snakeoil cert: %s", err)
	}
	f(`
scrape_configs:
- job_name: foo
  tls_config:
    cert_file: testdata/ssl-cert-snakeoil.pem
    key_file: testdata/ssl-cert-snakeoil.key
  static_configs:
  - targets: ["foo.bar:1234"]
`, []ScrapeWork{
		{
			ScrapeURL:      "http://foo.bar:1234/metrics",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
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
					Name:  "job",
					Value: "foo",
				},
			},
			TLSCertificate: &snakeoilCert,
		},
	})
	f(`
scrape_configs:
  - job_name: 'snmp'
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
`, []ScrapeWork{
		{
			ScrapeURL:      "http://127.0.0.1:9116/snmp?module=if_mib&target=192.168.1.2",
			ScrapeInterval: defaultScrapeInterval,
			ScrapeTimeout:  defaultScrapeTimeout,
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
					Name: "__param_module",
					Value: "if_mib",
				},
				{
					Name: "__param_target",
					Value: "192.168.1.2",
				},
				{
					Name:  "__scheme__",
					Value: "http",
				},
				{
					Name: "instance",
					Value: "192.168.1.2",
				},
				{
					Name:  "job",
					Value: "snmp",
				},
			},
		},
	})
}

var defaultRegexForRelabelConfig = regexp.MustCompile("^(.*)$")
