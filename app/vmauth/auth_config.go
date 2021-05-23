package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
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
	authConfigPath = flag.String("auth.config", "", "Path to auth config. See https://docs.victoriametrics.com/vmauth.html "+
		"for details on the format of this auth config")
)

// AuthConfig represents auth config.
type AuthConfig struct {
	Users []UserInfo `yaml:"users"`
}

// UserInfo is user information read from authConfigPath
type UserInfo struct {
	BearerToken string   `yaml:"bearer_token"`
	Username    string   `yaml:"username"`
	Password    string   `yaml:"password"`
	URLPrefix   *yamlURL `yaml:"url_prefix"`
	URLMap      []URLMap `yaml:"url_map"`

	requests *metrics.Counter
}

// URLMap is a mapping from source paths to target urls.
type URLMap struct {
	SrcPaths  []*SrcPath `yaml:"src_paths"`
	URLPrefix *yamlURL   `yaml:"url_prefix"`
}

// SrcPath represents an src path
type SrcPath struct {
	sOriginal string
	re        *regexp.Regexp
}

type yamlURL struct {
	u *url.URL
}

func (yu *yamlURL) UnmarshalYAML(f func(interface{}) error) error {
	var s string
	if err := f(&s); err != nil {
		return err
	}
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("cannot unmarshal %q into url: %w", s, err)
	}
	yu.u = u
	return nil
}

func (yu *yamlURL) MarshalYAML() (interface{}, error) {
	return yu.u.String(), nil
}

func (sp *SrcPath) match(s string) bool {
	prefix, ok := sp.re.LiteralPrefix()
	if ok {
		// Fast path - literal match
		return s == prefix
	}
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	return sp.re.MatchString(s)
}

// UnmarshalYAML implements yaml.Unmarshaler
func (sp *SrcPath) UnmarshalYAML(f func(interface{}) error) error {
	var s string
	if err := f(&s); err != nil {
		return err
	}
	sAnchored := "^(?:" + s + ")$"
	re, err := regexp.Compile(sAnchored)
	if err != nil {
		return fmt.Errorf("cannot build regexp from %q: %w", s, err)
	}
	sp.sOriginal = s
	sp.re = re
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (sp *SrcPath) MarshalYAML() (interface{}, error) {
	return sp.sOriginal, nil
}

func initAuthConfig() {
	if len(*authConfigPath) == 0 {
		logger.Fatalf("missing required `-auth.config` command-line flag")
	}

	// Register SIGHUP handler for config re-read just before readAuthConfig call.
	// This guarantees that the config will be re-read if the signal arrives during readAuthConfig call.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240
	sighupCh := procutil.NewSighupChan()

	m, err := readAuthConfig(*authConfigPath)
	if err != nil {
		logger.Fatalf("cannot load auth config from `-auth.config=%s`: %s", *authConfigPath, err)
	}
	authConfig.Store(m)
	stopCh = make(chan struct{})
	authConfigWG.Add(1)
	go func() {
		defer authConfigWG.Done()
		authConfigReloader(sighupCh)
	}()
}

func stopAuthConfig() {
	close(stopCh)
	authConfigWG.Wait()
}

func authConfigReloader(sighupCh <-chan os.Signal) {
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
	byAuthToken := make(map[string]*UserInfo, len(uis))
	byUsername := make(map[string]bool, len(uis))
	byBearerToken := make(map[string]bool, len(uis))
	for i := range uis {
		ui := &uis[i]
		if ui.BearerToken == "" && ui.Username == "" {
			return nil, fmt.Errorf("either bearer_token or username must be set")
		}
		if ui.BearerToken != "" && ui.Username != "" {
			return nil, fmt.Errorf("bearer_token=%q and username=%q cannot be set simultaneously", ui.BearerToken, ui.Username)
		}
		if byBearerToken[ui.BearerToken] {
			return nil, fmt.Errorf("duplicate bearer_token found; bearer_token: %q", ui.BearerToken)
		}
		if byUsername[ui.Username] {
			return nil, fmt.Errorf("duplicate username found; username: %q", ui.Username)
		}
		authToken := getAuthToken(ui.BearerToken, ui.Username, ui.Password)
		if byAuthToken[authToken] != nil {
			return nil, fmt.Errorf("duplicate auth token found for bearer_token=%q, username=%q: %q", authToken, ui.BearerToken, ui.Username)
		}
		if ui.URLPrefix != nil {
			urlPrefix, err := sanitizeURLPrefix(ui.URLPrefix.u)
			if err != nil {
				return nil, err
			}
			ui.URLPrefix.u = urlPrefix
		}
		for _, e := range ui.URLMap {
			if len(e.SrcPaths) == 0 {
				return nil, fmt.Errorf("missing `src_paths` in `url_map`")
			}
			if e.URLPrefix == nil {
				return nil, fmt.Errorf("missing `url_prefix` in `url_map`")
			}
			urlPrefix, err := sanitizeURLPrefix(e.URLPrefix.u)
			if err != nil {
				return nil, err
			}
			e.URLPrefix.u = urlPrefix
		}
		if len(ui.URLMap) == 0 && ui.URLPrefix == nil {
			return nil, fmt.Errorf("missing `url_prefix`")
		}
		if ui.BearerToken != "" {
			if ui.Password != "" {
				return nil, fmt.Errorf("password shouldn't be set for bearer_token %q", ui.BearerToken)
			}
			ui.requests = metrics.GetOrCreateCounter(`vmauth_user_requests_total{username="bearer_token"}`)
			byBearerToken[ui.BearerToken] = true
		}
		if ui.Username != "" {
			ui.requests = metrics.GetOrCreateCounter(fmt.Sprintf(`vmauth_user_requests_total{username=%q}`, ui.Username))
			byUsername[ui.Username] = true
		}
		byAuthToken[authToken] = ui
	}
	return byAuthToken, nil
}

func getAuthToken(bearerToken, username, password string) string {
	if bearerToken != "" {
		return "Bearer " + bearerToken
	}
	token := username + ":" + password
	token64 := base64.StdEncoding.EncodeToString([]byte(token))
	return "Basic " + token64
}

func sanitizeURLPrefix(urlPrefix *url.URL) (*url.URL, error) {
	// Remove trailing '/' from urlPrefix
	for strings.HasSuffix(urlPrefix.Path, "/") {
		urlPrefix.Path = urlPrefix.Path[:len(urlPrefix.Path)-1]
	}
	// Validate urlPrefix
	if urlPrefix.Scheme != "http" && urlPrefix.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme for `url_prefix: %q`: %q; must be `http` or `https`", urlPrefix, urlPrefix.Scheme)
	}
	if urlPrefix.Host == "" {
		return nil, fmt.Errorf("missing hostname in `url_prefix %q`", urlPrefix.Host)
	}
	return urlPrefix, nil
}
