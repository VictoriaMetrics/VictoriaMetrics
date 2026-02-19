package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	metricsTenantPlaceholder       = `{{.MetricsTenant}}`
	metricsExtraLabelsPlaceholder  = `{{.MetricsExtraLabels}}`
	metricsExtraFiltersPlaceholder = `{{.MetricsExtraFilters}}`

	logsAccountIDPlaceholder          = `{{.LogsAccountID}}`
	logsProjectIDPlaceholder          = `{{.LogsProjectID}}`
	logsExtraFiltersPlaceholder       = `{{.LogsExtraFilters}}`
	logsExtraStreamFiltersPlaceholder = `{{.LogsExtraStreamFilters}}`
)

var allPlaceholders = []string{
	metricsTenantPlaceholder,
	metricsExtraLabelsPlaceholder,
	metricsExtraFiltersPlaceholder,
	logsAccountIDPlaceholder,
	logsProjectIDPlaceholder,
	logsExtraFiltersPlaceholder,
	logsExtraStreamFiltersPlaceholder,
}

type jwtCache struct {
	// users contain UserInfo`s from AuthConfig with JWTConfig set
	users []*UserInfo
}

type JWTConfig struct {
	PublicKeys     []string `yaml:"public_keys,omitempty"`
	PublicKeyFiles []string `yaml:"public_key_files,omitempty"`
	SkipVerify     bool     `yaml:"skip_verify,omitempty"`

	verifierPool *jwt.VerifierPool

	anyPlaceholderUsed bool

	metricsTenantPlaceholderUsed       bool
	metricsExtraLabelsPlaceholderUsed  bool
	metricsExtraFiltersPlaceholderUsed bool

	logsAccountIDPlaceholderUsed          bool
	logsProjectIDPlaceholderUsed          bool
	logsExtraFiltersPlaceholderUsed       bool
	logsExtraStreamFiltersPlaceholderUsed bool
}

func parseJWTUsers(ac *AuthConfig) ([]*UserInfo, error) {
	jui := make([]*UserInfo, 0, len(ac.Users))
	for _, ui := range ac.Users {
		jwtToken := ui.JWT
		if jwtToken == nil {
			continue
		}

		if ui.AuthToken != "" || ui.BearerToken != "" || ui.Username != "" || ui.Password != "" {
			return nil, fmt.Errorf("auth_token, bearer_token, username and password cannot be specified if jwt is set")
		}
		if len(jwtToken.PublicKeys) == 0 && len(jwtToken.PublicKeyFiles) == 0 && !jwtToken.SkipVerify {
			return nil, fmt.Errorf("jwt must contain at least a single public key, public_key_files or have skip_verify=true")
		}

		if len(jwtToken.PublicKeys) > 0 || len(jwtToken.PublicKeyFiles) > 0 {
			keys := make([]any, 0, len(jwtToken.PublicKeys)+len(jwtToken.PublicKeyFiles))

			for i := range jwtToken.PublicKeys {
				k, err := jwt.ParseKey([]byte(jwtToken.PublicKeys[i]))
				if err != nil {
					return nil, err
				}
				keys = append(keys, k)
			}

			for _, filePath := range jwtToken.PublicKeyFiles {
				keyData, err := os.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("cannot read public key from file %q: %w", filePath, err)
				}
				k, err := jwt.ParseKey(keyData)
				if err != nil {
					return nil, fmt.Errorf("cannot parse public key from file %q: %w", filePath, err)
				}
				keys = append(keys, k)
			}

			vp, err := jwt.NewVerifierPool(keys)
			if err != nil {
				return nil, err
			}

			jwtToken.verifierPool = vp
		}

		if err := ui.initURLs(); err != nil {
			return nil, err
		}

		metricLabels, err := ui.getMetricLabels()
		if err != nil {
			return nil, fmt.Errorf("cannot parse metric_labels: %w", err)
		}
		ui.requests = ac.ms.GetOrCreateCounter(`vmauth_user_requests_total` + metricLabels)
		ui.requestErrors = ac.ms.GetOrCreateCounter(`vmauth_user_request_errors_total` + metricLabels)
		ui.backendRequests = ac.ms.GetOrCreateCounter(`vmauth_user_request_backend_requests_total` + metricLabels)
		ui.backendErrors = ac.ms.GetOrCreateCounter(`vmauth_user_request_backend_errors_total` + metricLabels)
		ui.requestsDuration = ac.ms.GetOrCreateSummary(`vmauth_user_request_duration_seconds` + metricLabels)
		mcr := ui.getMaxConcurrentRequests()
		ui.concurrencyLimitCh = make(chan struct{}, mcr)
		ui.concurrencyLimitReached = ac.ms.GetOrCreateCounter(`vmauth_user_concurrent_requests_limit_reached_total` + metricLabels)
		_ = ac.ms.GetOrCreateGauge(`vmauth_user_concurrent_requests_capacity`+metricLabels, func() float64 {
			return float64(cap(ui.concurrencyLimitCh))
		})
		_ = ac.ms.GetOrCreateGauge(`vmauth_user_concurrent_requests_current`+metricLabels, func() float64 {
			return float64(len(ui.concurrencyLimitCh))
		})

		rt, err := newRoundTripper(ui.TLSCAFile, ui.TLSCertFile, ui.TLSKeyFile, ui.TLSServerName, ui.TLSInsecureSkipVerify)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize HTTP RoundTripper: %w", err)
		}
		ui.rt = rt

		if err := validateJWTPlaceholders(&ui); err != nil {
			return nil, err
		}

		jui = append(jui, &ui)
	}

	// the limitation will be lifted once claim based matching will be implemented
	if len(jui) > 1 {
		return nil, fmt.Errorf("multiple users with JWT tokens are not supported; found %d users", len(jui))
	}

	return jui, nil
}

func getUserInfoByJWTToken(ats []string) (*UserInfo, *jwt.Token) {
	js := *jwtAuthCache.Load()
	if len(js.users) == 0 {
		return nil, nil
	}

	for _, at := range ats {
		if strings.Count(at, ".") != 2 {
			continue
		}

		at, _ = strings.CutPrefix(at, `http_auth:`)

		tkn, err := jwt.NewToken(at, true)
		if err != nil {
			if *logInvalidAuthTokens {
				logger.Infof("cannot parse jwt token: %s", err)
			}
			continue
		}
		if tkn.IsExpired(time.Now()) {
			if *logInvalidAuthTokens {
				logger.Infof("jwt token is expired")
			}
			continue
		}

		for _, ui := range js.users {
			if ui.JWT.SkipVerify {
				return ui, tkn
			}

			if err := ui.JWT.verifierPool.Verify(tkn); err != nil {
				if *logInvalidAuthTokens {
					logger.Infof("cannot verify jwt token: %s", err)
				}
				continue
			}

			return ui, tkn
		}
	}

	return nil, nil
}

func validateJWTPlaceholders(ui *UserInfo) error {
	if ui.JWT == nil {
		logger.Panicf("BUG: validateJWTPlaceholders must be called for users with JWT authentication method defined")
	}

	var s string
	if ui.URLPrefix != nil {
		for _, u := range ui.URLPrefix.busOriginal {
			s += ", " + u.Path
			s += ", " + u.RawQuery
		}
	}
	for _, rh := range ui.HeadersConf.RequestHeaders {
		s += ", " + rh.Value
	}

	for _, um := range ui.URLMaps {
		for _, u := range um.URLPrefix.busOriginal {
			s += ", " + u.Path
			s += ", " + u.RawQuery
		}
		for _, rh := range um.HeadersConf.RequestHeaders {
			s += ", " + rh.Value
		}
	}

	ui.JWT.metricsTenantPlaceholderUsed = strings.Contains(s, metricsTenantPlaceholder)
	ui.JWT.metricsExtraLabelsPlaceholderUsed = strings.Contains(s, metricsExtraLabelsPlaceholder)
	ui.JWT.metricsExtraFiltersPlaceholderUsed = strings.Contains(s, metricsExtraFiltersPlaceholder)
	ui.JWT.logsAccountIDPlaceholderUsed = strings.Contains(s, logsAccountIDPlaceholder)
	ui.JWT.logsProjectIDPlaceholderUsed = strings.Contains(s, logsProjectIDPlaceholder)
	ui.JWT.logsExtraFiltersPlaceholderUsed = strings.Contains(s, logsExtraFiltersPlaceholder)
	ui.JWT.logsExtraStreamFiltersPlaceholderUsed = strings.Contains(s, logsExtraStreamFiltersPlaceholder)

	ui.JWT.anyPlaceholderUsed = ui.JWT.metricsTenantPlaceholderUsed ||
		ui.JWT.metricsExtraLabelsPlaceholderUsed ||
		ui.JWT.metricsExtraFiltersPlaceholderUsed ||
		ui.JWT.logsAccountIDPlaceholderUsed ||
		ui.JWT.logsProjectIDPlaceholderUsed ||
		ui.JWT.logsExtraFiltersPlaceholderUsed ||
		ui.JWT.logsExtraStreamFiltersPlaceholderUsed

	for _, p := range allPlaceholders {
		s = strings.ReplaceAll(s, p, ``)
	}
	if strings.Contains(s, `{{`) || strings.Contains(s, `}}`) {
		return fmt.Errorf("invalid placeholder found in URL or headers; allowed placeholders are: %s", strings.Join(allPlaceholders, ", "))
	}

	return nil
}

func replaceJWTPlaceholders(targetURL *url.URL, rhs []*Header, jwtConf *JWTConfig, vma *jwt.VMAccessClaim) {
	if !jwtConf.anyPlaceholderUsed {
		return
	}

	if jwtConf.metricsTenantPlaceholderUsed {
		tenant := fmt.Sprintf("%d:%d", vma.MetricsAccountID, vma.MetricsProjectID)
		targetURL.Path = strings.ReplaceAll(targetURL.Path, metricsTenantPlaceholder, tenant)
	}

	if jwtConf.logsAccountIDPlaceholderUsed || jwtConf.logsProjectIDPlaceholderUsed {
		for _, h := range rhs {
			if h.Name == `AccountID` && jwtConf.logsAccountIDPlaceholderUsed {
				h.Value = strings.ReplaceAll(h.Value, logsAccountIDPlaceholder, strconv.FormatInt(vma.LogsAccountID, 10))
			}
			if h.Name == `ProjectID` && jwtConf.logsProjectIDPlaceholderUsed {
				h.Value = strings.ReplaceAll(h.Value, logsProjectIDPlaceholder, strconv.FormatInt(vma.LogsProjectID, 10))
			}
		}
	}

	query := targetURL.Query()
	if jwtConf.metricsExtraLabelsPlaceholderUsed {
		vals := append([]string{}, query[`extra_label`]...)
		query[`extra_label`] = nil
		for _, v := range vals {
			if v == metricsExtraLabelsPlaceholder {
				for _, label := range vma.MetricsExtraLabels {
					query.Add(`extra_label`, label)
				}
				continue
			}

			query.Add(`extra_label`, v)
		}
	}
	if jwtConf.metricsExtraLabelsPlaceholderUsed {
		vals := append([]string{}, query[`extra_filters`]...)
		query[`extra_filters`] = nil
		for _, v := range vals {
			if v == metricsExtraFiltersPlaceholder {
				for _, label := range vma.MetricsExtraFilters {
					query.Add(`extra_filters`, label)
				}
				continue
			}

			query.Add(`extra_filters`, v)
		}
	}
	if jwtConf.logsExtraFiltersPlaceholderUsed {
		vals := append([]string{}, query[`extra_filters`]...)
		query[`extra_filters`] = nil
		for _, v := range vals {
			if v == logsExtraFiltersPlaceholder {
				for _, label := range vma.LogsExtraFilters {
					query.Add(`extra_filters`, label)
				}
				continue
			}

			query.Add(`extra_filters`, v)
		}
	}
	if jwtConf.logsExtraFiltersPlaceholderUsed {
		vals := append([]string{}, query[`extra_stream_filters`]...)
		query[`extra_stream_filters`] = nil
		for _, v := range vals {
			if v == logsExtraStreamFiltersPlaceholder {
				for _, label := range vma.LogsExtraStreamFilters {
					query.Add(`extra_stream_filters`, label)
				}
				continue
			}

			query.Add(`extra_stream_filters`, v)
		}
	}
	targetURL.RawQuery = query.Encode()
}

var placeholderRegexp = regexp.MustCompile(`{{.*?}}`)

func validateNoPlaceholders(ui *UserInfo) error {
	if ui.URLPrefix != nil {
		for _, u := range ui.URLPrefix.busOriginal {
			if placeholderRegexp.MatchString(u.Path) || placeholderRegexp.MatchString(u.RawQuery) {
				return fmt.Errorf("placeholders are not allowed in URL prefix path or query")
			}
		}
	}
	for _, rh := range ui.HeadersConf.RequestHeaders {
		if placeholderRegexp.MatchString(rh.Value) {
			return fmt.Errorf("placeholders are not allowed in headers")
		}
	}

	for _, um := range ui.URLMaps {
		for _, u := range um.URLPrefix.busOriginal {
			if placeholderRegexp.MatchString(u.Path) || placeholderRegexp.MatchString(u.RawQuery) {
				return fmt.Errorf("placeholders are not allowed in URL prefix path or query")
			}
		}
		for _, rh := range um.HeadersConf.RequestHeaders {
			if placeholderRegexp.MatchString(rh.Value) {
				return fmt.Errorf("placeholders are not allowed in headers")
			}
		}
	}

	return nil
}
