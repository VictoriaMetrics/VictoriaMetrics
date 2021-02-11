package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/metrics"
	"gopkg.in/yaml.v2"
)

var (
	authConfigPath = flag.String("auth.config", "", "Path to auth config. See https://victoriametrics.github.io/vmauth.html "+
		"for details on the format of this auth config")
)

// AuthConfig represents auth config.
type AuthConfig struct {
	Users []UserInfo `yaml:"users"`
}

// UserInfo is user information read from authConfigPath
type UserInfo struct {
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	URLPrefix string   `yaml:"url_prefix"`
	URLMap    []URLMap `yaml:"url_map"`

	requests *metrics.Counter
}

// URLMap is a mapping from source paths to target urls.
type URLMap struct {
	SrcPaths  []string `yaml:"src_paths"`
	URLPrefix string   `yaml:"url_prefix"`
}

func initAuthConfig() {
	if len(*authConfigPath) == 0 {
		logger.Fatalf("missing required `-auth.config` command-line flag")
	}
	m, err := readAuthConfig(*authConfigPath)
	if err != nil {
		logger.Fatalf("cannot load auth config from `-auth.config=%s`: %s", *authConfigPath, err)
	}
	authConfig.Store(m)
	stopCh = make(chan struct{})
	authConfigWG.Add(1)
	go func() {
		defer authConfigWG.Done()
		authConfigReloader()
	}()
}

func stopAuthConfig() {
	close(stopCh)
	authConfigWG.Wait()
}

func authConfigReloader() {
	sighupCh := procutil.NewSighupChan()
	for {
		select {
		case <-stopCh:
			return
		case <-sighupCh:
			logger.Infof("SIGHUP received; loading -auth.config=%q", *authConfigPath)
			m, err := readAuthConfig(*authConfigPath)
			if err != nil {
				logger.Errorf("failed to load -auth.config=%q; using the last successfully loaded config; error: %s", *authConfigPath, err)
				continue
			}
			authConfig.Store(m)
			logger.Infof("Successfully reloaded -auth.config=%q", *authConfigPath)
		}
	}
}

var authConfig atomic.Value
var authConfigWG sync.WaitGroup
var stopCh chan struct{}

func readAuthConfig(path string) (map[string]*UserInfo, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}
	m, err := parseAuthConfig(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", path, err)
	}
	logger.Infof("Loaded information about %d users from %q", len(m), path)
	return m, nil
}

func parseAuthConfig(data []byte) (map[string]*UserInfo, error) {
	data = envtemplate.Replace(data)
	var ac AuthConfig
	if err := yaml.UnmarshalStrict(data, &ac); err != nil {
		return nil, fmt.Errorf("cannot unmarshal AuthConfig data: %w", err)
	}
	uis := ac.Users
	if len(uis) == 0 {
		return nil, fmt.Errorf("`users` section cannot be empty in AuthConfig")
	}
	m := make(map[string]*UserInfo, len(uis))
	for i := range uis {
		ui := &uis[i]
		if m[ui.Username] != nil {
			return nil, fmt.Errorf("duplicate username found; username: %q", ui.Username)
		}
		if len(ui.URLPrefix) > 0 {
			urlPrefix, err := sanitizeURLPrefix(ui.URLPrefix)
			if err != nil {
				return nil, err
			}
			ui.URLPrefix = urlPrefix
		}
		for _, e := range ui.URLMap {
			if len(e.SrcPaths) == 0 {
				return nil, fmt.Errorf("missing `src_paths`")
			}
			for _, path := range e.SrcPaths {
				if !strings.HasPrefix(path, "/") {
					return nil, fmt.Errorf("`src_path`=%q must start with `/`", path)
				}
			}
			urlPrefix, err := sanitizeURLPrefix(e.URLPrefix)
			if err != nil {
				return nil, err
			}
			e.URLPrefix = urlPrefix
		}
		if len(ui.URLMap) == 0 && len(ui.URLPrefix) == 0 {
			return nil, fmt.Errorf("missing `url_prefix`")
		}
		ui.requests = metrics.GetOrCreateCounter(fmt.Sprintf(`vmauth_user_requests_total{username=%q}`, ui.Username))
		m[ui.Username] = ui
	}
	return m, nil
}

func sanitizeURLPrefix(urlPrefix string) (string, error) {
	// Remove trailing '/' from urlPrefix
	for strings.HasSuffix(urlPrefix, "/") {
		urlPrefix = urlPrefix[:len(urlPrefix)-1]
	}
	// Validate urlPrefix
	target, err := url.Parse(urlPrefix)
	if err != nil {
		return "", fmt.Errorf("invalid `url_prefix: %q`: %w", urlPrefix, err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme for `url_prefix: %q`: %q; must be `http` or `https`", urlPrefix, target.Scheme)
	}
	if target.Host == "" {
		return "", fmt.Errorf("missing hostname in `url_prefix %q`", urlPrefix)
	}
	return urlPrefix, nil
}
