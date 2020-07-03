package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/metrics"
	"gopkg.in/yaml.v2"
)

var (
	authConfigPath = flag.String("auth.config", "", "Path to auth config. See https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmauth/README.md "+
		"for details on the format of this auth config")
)

// AuthConfig represents auth config.
type AuthConfig struct {
	Users []UserInfo `yaml:"users"`
}

// UserInfo is user information read from authConfigPath
type UserInfo struct {
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	URLPrefix string `yaml:"url_prefix"`

	requests *metrics.Counter
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
		urlPrefix := ui.URLPrefix
		// Remove trailing '/' from urlPrefix
		for strings.HasSuffix(urlPrefix, "/") {
			urlPrefix = urlPrefix[:len(urlPrefix)-1]
		}
		// Validate urlPrefix
		target, err := url.Parse(urlPrefix)
		if err != nil {
			return nil, fmt.Errorf("invalid `url_prefix: %q`: %w", urlPrefix, err)
		}
		if target.Scheme != "http" && target.Scheme != "https" {
			return nil, fmt.Errorf("unsupported scheme for `url_prefix: %q`: %q; must be `http` or `https`", urlPrefix, target.Scheme)
		}

		ui.URLPrefix = urlPrefix
		ui.requests = metrics.GetOrCreateCounter(fmt.Sprintf(`vmauth_user_requests_total{username=%q}`, ui.Username))
		m[ui.Username] = ui
	}
	return m, nil
}
