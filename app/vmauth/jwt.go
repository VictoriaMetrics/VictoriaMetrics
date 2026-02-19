package main

import (
	"fmt"
	"net/url"
	"os"
	"slices"
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

	placeholderPrefix = `{{`
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

var urlPathPlaceHolders = []string{
	metricsTenantPlaceholder,
	logsAccountIDPlaceholder,
	logsProjectIDPlaceholder,
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
		if err := parseJWTPlaceholdersForUserInfo(&ui, true); err != nil {
			return nil, err
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

		jui = append(jui, &ui)
	}

	// TODO: the limitation will be lifted once claim based matching will be implemented
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
				// TODO: add more context:
				// token claims with issuer
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

func replaceJWTPlaceholders(bu *backendURL, hc HeadersConf, vma *jwt.VMAccessClaim) (*url.URL, HeadersConf) {
	if !bu.hasPlaceHolders && !hc.hasAnyPlaceHolders {
		return bu.url, hc
	}
	targetURL := bu.url
	data := jwtClaimsData(vma)
	if bu.hasPlaceHolders {
		// template url params and request path
		// make a copy of url
		uCopy := *bu.url
		for _, uph := range urlPathPlaceHolders {
			replacement := data[uph]
			uCopy.Path = strings.ReplaceAll(uCopy.Path, uph, replacement[0])
		}
		query := uCopy.Query()
		var foundAnyQueryPlaceholder bool
		var templatedValues []string
		for param, values := range query {
			templatedValues = templatedValues[:0]
			// filter in-place values with placeholders
			// and accumulate replacements
			// it will change the order of param values
			// but it's not guaranteed
			// and will be changed in any way with multiple arg templates
			var cnt int
			for _, value := range values {
				if dv, ok := data[value]; ok {
					foundAnyQueryPlaceholder = true
					templatedValues = append(templatedValues, dv...)
					continue
				}
				values[cnt] = value
				cnt++
			}
			values = values[:cnt]
			values = append(values, templatedValues...)
			query[param] = values
		}
		if foundAnyQueryPlaceholder {
			uCopy.RawQuery = query.Encode()
		}
		targetURL = &uCopy
	}
	if hc.hasAnyPlaceHolders {
		// make a copy of headers and update only values with placeholder
		rhs := make([]*Header, 0, len(hc.RequestHeaders))
		for _, rh := range hc.RequestHeaders {
			if dv, ok := data[rh.Value]; ok {
				rh := &Header{
					Name:  rh.Name,
					Value: strings.Join(dv, ","),
				}
				rhs = append(rhs, rh)
				continue
			}
			rhs = append(rhs, rh)
		}
		hc.RequestHeaders = rhs
	}

	return targetURL, hc
}

func jwtClaimsData(vma *jwt.VMAccessClaim) map[string][]string {
	data := map[string][]string{
		// TODO: optimize at parsing stage
		metricsTenantPlaceholder:       {fmt.Sprintf("%d:%d", vma.MetricsAccountID, vma.MetricsProjectID)},
		metricsExtraLabelsPlaceholder:  vma.MetricsExtraLabels,
		metricsExtraFiltersPlaceholder: vma.MetricsExtraFilters,

		// TODO: optimize at parsing stage
		logsAccountIDPlaceholder:          {fmt.Sprintf("%d", vma.LogsAccountID)},
		logsProjectIDPlaceholder:          {fmt.Sprintf("%d", vma.LogsProjectID)},
		logsExtraFiltersPlaceholder:       vma.LogsExtraFilters,
		logsExtraStreamFiltersPlaceholder: vma.LogsExtraStreamFilters,
	}
	return data
}

func parseJWTPlaceholdersForUserInfo(ui *UserInfo, isAllowed bool) error {
	if ui.URLPrefix != nil {
		if err := validateJWTPlaceholdersForURL(ui.URLPrefix, isAllowed); err != nil {
			return err
		}
	}
	if err := parsePlaceholdersForHC(&ui.HeadersConf, isAllowed); err != nil {
		return err
	}
	if ui.DefaultURL != nil {
		if err := validateJWTPlaceholdersForURL(ui.DefaultURL, isAllowed); err != nil {
			return fmt.Errorf("invalid `default_url` placeholders: %w", err)
		}
	}
	for i := range ui.URLMaps {
		e := &ui.URLMaps[i]
		if e.URLPrefix != nil {
			if err := validateJWTPlaceholdersForURL(e.URLPrefix, isAllowed); err != nil {
				return fmt.Errorf("invalid `url_map` `url_prefix` placeholders: %w", err)
			}
		}
		if err := parsePlaceholdersForHC(&e.HeadersConf, isAllowed); err != nil {
			return fmt.Errorf("invalid `url_map` headers placeholders: %w", err)
		}

	}
	return nil
}

func validateJWTPlaceholdersForURL(up *URLPrefix, isAllowed bool) error {
	for _, bu := range up.busOriginal {
		ok := strings.Contains(bu.Path, placeholderPrefix)
		if ok && !isAllowed {
			return fmt.Errorf("placeholder: %q is only allowed at JWT token context", bu.Path)
		}
		if ok {
			p := bu.Path
			for _, ph := range allPlaceholders {
				p = strings.ReplaceAll(p, ph, ``)
			}
			if strings.Contains(p, placeholderPrefix) {
				return fmt.Errorf("invalid placeholder found in URL request path: %q, supported values are: %s", bu.Path, strings.Join(allPlaceholders, ", "))

			}
		}
		for param, values := range bu.Query() {
			for _, value := range values {
				ok := strings.Contains(value, placeholderPrefix)
				if ok && !isAllowed {
					return fmt.Errorf("query param: %q with placeholder: %q is only allowed at JWT token context", param, value)
				}
				if ok {
					// possible placeholder
					if !slices.Contains(allPlaceholders, value) {
						return fmt.Errorf("query param: %q has unsupported placeholder string: %q, supported values are: %s", param, value, strings.Join(allPlaceholders, ", "))
					}
				}
			}
		}
	}
	return nil
}

func parsePlaceholdersForHC(hc *HeadersConf, isAllowed bool) error {
	for _, rhs := range hc.RequestHeaders {
		ok := strings.Contains(rhs.Value, placeholderPrefix)
		if ok && !isAllowed {
			return fmt.Errorf("request header: %q placeholder: %q is only supported at JWT context", rhs.Name, rhs.Value)
		}
		if ok {
			if !slices.Contains(allPlaceholders, rhs.Value) {
				return fmt.Errorf("request header: %q has unsupported placeholder: %q, supported values are: %s", rhs.Name, rhs.Value, strings.Join(allPlaceholders, ", "))
			}
			hc.hasAnyPlaceHolders = true
		}
	}
	for _, rhs := range hc.ResponseHeaders {
		if strings.Contains(rhs.Value, placeholderPrefix) {
			return fmt.Errorf("response header placeholders are not supported; found placeholder prefix at header: %q with value: %q", rhs.Name, rhs.Value)
		}
	}
	return nil
}

func hasAnyPlaceholders(u *url.URL) bool {
	if strings.Contains(u.Path, placeholderPrefix) {
		return true
	}
	if len(u.Query()) == 0 {
		return false
	}
	for _, values := range u.Query() {
		for _, value := range values {
			if strings.HasPrefix(value, placeholderPrefix) {
				return true
			}
		}

	}
	return false
}
