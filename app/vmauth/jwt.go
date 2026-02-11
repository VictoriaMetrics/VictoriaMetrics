package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type jwtAuthState struct {
	// users holds UserInfo structs with JWTToken field set.
	users []*UserInfo

	cacheMux sync.Mutex
	cache    map[string]*jwtUserInfo
	cleanT   *time.Ticker
}

type JWTToken struct {
	PublicKeys []string `yaml:"public_keys,omitempty"`
	SkipVerify bool     `yaml:"skip_verify,omitempty"`

	verifierPool *jwt.VerifierPool
}

type jwtUserInfo struct {
	UserInfo
	Token *jwt.Token
}

func parseJWTUsers(ac *AuthConfig) ([]*UserInfo, error) {
	jui := make([]*UserInfo, 0, len(ac.Users))
	for _, ui := range ac.Users {
		jwtToken := ui.JWTToken
		if jwtToken == nil {
			continue
		}

		if ui.AuthToken != "" || ui.BearerToken != "" || ui.Username != "" || ui.Password != "" {
			return nil, fmt.Errorf("auth_token, bearer_token, username and password cannot be specified if jwt_token is set")
		}
		if len(jwtToken.PublicKeys) == 0 && !jwtToken.SkipVerify {
			return nil, fmt.Errorf("jwt_token must contain at least a single public key or have skip_verify=true")
		}

		if len(jwtToken.PublicKeys) > 0 {
			keys := make([]any, 0, len(jwtToken.PublicKeys))
			for i := range jwtToken.PublicKeys {
				k, err := jwt.ParseKey([]byte(jwtToken.PublicKeys[i]))
				if err != nil {
					return nil, err
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

		jui = append(jui, &ui)
	}

	// the limitation will be lifted once claim based matching will be implemented
	if len(jui) > 1 {
		return nil, fmt.Errorf("multiple users with JWT tokens are not supported; found %d users", len(jui))
	}

	return jui, nil
}

func getUserInfoByJWTToken(ats []string) *UserInfo {
	js := jwtState.Load()

	removeExpiredJWTTokens(js)

	for _, at := range ats {
		if strings.Count(at, ".") != 2 {
			continue
		}

		at, _ = strings.CutPrefix(at, `http_auth:`)

		js.cacheMux.Lock()
		jui, ok := js.cache[at]
		js.cacheMux.Unlock()
		if ok {
			if jui.Token.IsExpired(time.Now()) {
				if *logInvalidAuthTokens {
					logger.Infof("jwt token is expired")
				}
				return nil
			}
			return &jui.UserInfo
		}

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
			if ui.JWTToken.SkipVerify {
				return ui
			}

			if err := ui.JWTToken.verifierPool.Verify(tkn); err != nil {
				if *logInvalidAuthTokens {
					logger.Infof("cannot verify jwt token: %s", err)
				}
				continue
			}

			js.cacheMux.Lock()
			jui, ok := js.cache[at]
			if ok {
				js.cacheMux.Unlock()
				return &jui.UserInfo
			}
			js.cache[at] = &jwtUserInfo{
				UserInfo: *ui,
				Token:    tkn,
			}
			js.cacheMux.Unlock()

			return ui
		}
	}

	return nil
}

func removeExpiredJWTTokens(js *jwtAuthState) {
	select {
	case <-js.cleanT.C:
	default:
		return
	}

	now := time.Now()
	js.cacheMux.Lock()
	for at, jui := range js.cache {
		if jui.Token.IsExpired(now) {
			delete(js.cache, at)
		}
	}
	js.cacheMux.Unlock()
}
