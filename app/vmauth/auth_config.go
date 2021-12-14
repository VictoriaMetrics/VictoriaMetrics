package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/metrics"
	"gopkg.in/yaml.v2"
)

var (
	authConfigPath = flag.String("auth.config", "", "Path to auth config. It can point either to local file or to http url. "+
		"See https://docs.victoriametrics.com/vmauth.html for details on the format of this auth config")
)

// AuthConfig represents auth config.
type AuthConfig struct {
	Users []UserInfo `yaml:"users,omitempty"`
}

// UserInfo is user information read from authConfigPath
type UserInfo struct {
	Name        string     `yaml:"name,omitempty"`
	BearerToken string     `yaml:"bearer_token,omitempty"`
	Username    string     `yaml:"username,omitempty"`
	Password    string     `yaml:"password,omitempty"`
	URLPrefix   *URLPrefix `yaml:"url_prefix,omitempty"`
	URLMap      []URLMap   `yaml:"url_map,omitempty"`
	Headers     []Header   `yaml:"headers,omitempty"`

	requests *metrics.Counter
}

// Header is `Name: Value` http header, which must be added to the proxied request.
type Header struct {
	Name  string
	Value string
}

// UnmarshalYAML unmarshals h from f.
func (h *Header) UnmarshalYAML(f func(interface{}) error) error {
	var s string
	if err := f(&s); err != nil {
		return err
	}
	n := strings.IndexByte(s, ':')
	if n < 0 {
		return fmt.Errorf("missing speparator char ':' between Name and Value in the header %q; expected format - 'Name: Value'", s)
	}
	h.Name = strings.TrimSpace(s[:n])
	h.Value = strings.TrimSpace(s[n+1:])
	return nil
}

// MarshalYAML marshals h to yaml.
func (h *Header) MarshalYAML() (interface{}, error) {
	s := fmt.Sprintf("%s: %s", h.Name, h.Value)
	return s, nil
}

// URLMap is a mapping from source paths to target urls.
type URLMap struct {
	SrcPaths  []*SrcPath `yaml:"src_paths,omitempty"`
	URLPrefix *URLPrefix `yaml:"url_prefix,omitempty"`
	Headers   []Header   `yaml:"headers,omitempty"`
}

// SrcPath represents an src path
type SrcPath struct {
	sOriginal string
	re        *regexp.Regexp
}

// URLPrefix represents pased `url_prefix`
type URLPrefix struct {
	n    uint32
	urls []*url.URL
}

func (up *URLPrefix) getNextURL() *url.URL {
	n := atomic.AddUint32(&up.n, 1)
	idx := n % uint32(len(up.urls))
	return up.urls[idx]
}

// UnmarshalYAML unmarshals up from yaml.
func (up *URLPrefix) UnmarshalYAML(f func(interface{}) error) error {
	var v interface{}
	if err := f(&v); err != nil {
		return err
	}
	var urls []string
	switch x := v.(type) {
	case string:
		urls = []string{x}
	case []interface{}:
		if len(x) == 0 {
			return fmt.Errorf("`url_prefix` must contain at least a single url")
		}
		us := make([]string, len(x))
		for i, xx := range x {
			s, ok := xx.(string)
			if !ok {
				return fmt.Errorf("`url_prefix` must contain array of strings; got %T", xx)
			}
			us[i] = s
		}
		urls = us
	default:
		return fmt.Errorf("unexpected type for `url_prefix`: %T; want string or []string", v)
	}
	pus := make([]*url.URL, len(urls))
	for i, u := range urls {
		pu, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("cannot unmarshal %q into url: %w", u, err)
		}
		pus[i] = pu
	}
	up.urls = pus
	return nil
}

// MarshalYAML marshals up to yaml.
func (up *URLPrefix) MarshalYAML() (interface{}, error) {
	var b []byte
	if len(up.urls) == 1 {
		u := up.urls[0].String()
		b = strconv.AppendQuote(b, u)
		return string(b), nil
	}
	b = append(b, '[')
	for i, pu := range up.urls {
		u := pu.String()
		b = strconv.AppendQuote(b, u)
		if i+1 < len(up.urls) {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return string(b), nil
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
	data, err := fs.ReadFileOrHTTP(path)
	if err != nil {
		return nil, err
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
		at1, at2 := getAuthTokens(ui.BearerToken, ui.Username, ui.Password)
		if byAuthToken[at1] != nil {
			return nil, fmt.Errorf("duplicate auth token found for bearer_token=%q, username=%q: %q", ui.BearerToken, ui.Username, at1)
		}
		if byAuthToken[at2] != nil {
			return nil, fmt.Errorf("duplicate auth token found for bearer_token=%q, username=%q: %q", ui.BearerToken, ui.Username, at2)
		}
		if ui.URLPrefix != nil {
			if err := ui.URLPrefix.sanitize(); err != nil {
				return nil, err
			}
		}
		for _, e := range ui.URLMap {
			if len(e.SrcPaths) == 0 {
				return nil, fmt.Errorf("missing `src_paths` in `url_map`")
			}
			if e.URLPrefix == nil {
				return nil, fmt.Errorf("missing `url_prefix` in `url_map`")
			}
			if err := e.URLPrefix.sanitize(); err != nil {
				return nil, err
			}
		}
		if len(ui.URLMap) == 0 && ui.URLPrefix == nil {
			return nil, fmt.Errorf("missing `url_prefix`")
		}
		if ui.BearerToken != "" {
			name := "bearer_token"
			if ui.Name != "" {
				name = ui.Name
			}
			if ui.Password != "" {
				return nil, fmt.Errorf("password shouldn't be set for bearer_token %q", ui.BearerToken)
			}
			ui.requests = metrics.GetOrCreateCounter(fmt.Sprintf(`vmauth_user_requests_total{username=%q}`, name))
			byBearerToken[ui.BearerToken] = true
		}
		if ui.Username != "" {
			name := ui.Username
			if ui.Name != "" {
				name = ui.Name
			}
			ui.requests = metrics.GetOrCreateCounter(fmt.Sprintf(`vmauth_user_requests_total{username=%q}`, name))
			byUsername[ui.Username] = true
		}
		byAuthToken[at1] = ui
		byAuthToken[at2] = ui
	}
	return byAuthToken, nil
}

func getAuthTokens(bearerToken, username, password string) (string, string) {
	if bearerToken != "" {
		// Accept the bearerToken as Basic Auth username with empty password
		at1 := getAuthToken(bearerToken, "", "")
		at2 := getAuthToken("", bearerToken, "")
		return at1, at2
	}
	at := getAuthToken("", username, password)
	return at, at
}

func getAuthToken(bearerToken, username, password string) string {
	if bearerToken != "" {
		return "Bearer " + bearerToken
	}
	token := username + ":" + password
	token64 := base64.StdEncoding.EncodeToString([]byte(token))
	return "Basic " + token64
}

func (up *URLPrefix) sanitize() error {
	for i, pu := range up.urls {
		puNew, err := sanitizeURLPrefix(pu)
		if err != nil {
			return err
		}
		up.urls[i] = puNew
	}
	return nil
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
