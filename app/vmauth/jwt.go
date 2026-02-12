package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

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
	jc := jwtAuthCache.Load()
	if len(jc.users) == 0 {
		return nil
	}

	jc.removeExpired()

	if jce, found := jc.getFirstVerified(ats); found {
		return jce.ui
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

		for _, ui := range jc.users {
			if ui.JWT.SkipVerify {
				return jc.addVerifiedIfNotExist(at, jwtVerified{ui: ui, tkn: tkn}).ui
			}

			if err := ui.JWT.verifierPool.Verify(tkn); err != nil {
				if *logInvalidAuthTokens {
					logger.Infof("cannot verify jwt token: %s", err)
				}
				continue
			}

			return jc.addVerifiedIfNotExist(at, jwtVerified{ui: ui, tkn: tkn}).ui
		}
	}

	return nil
}

type jwtVerified struct {
	ui  *UserInfo
	tkn *jwt.Token
}

type jwtCache struct {
	// users contain UserInfo`s from AuthConfig with JWTConfig set
	users []*UserInfo

	verifiedMux    sync.Mutex
	verified       map[string]jwtVerified
	removeExpiredT *time.Ticker
}

func (jc *jwtCache) getFirstVerified(ats []string) (jwtVerified, bool) {
	jc.verifiedMux.Lock()
	defer jc.verifiedMux.Unlock()

	for _, at := range ats {
		if strings.Count(at, ".") != 2 {
			continue
		}

		at, _ = strings.CutPrefix(at, `http_auth:`)
		jce, ok := jc.verified[at]
		if !ok {
			continue
		}

		if jce.tkn.IsExpired(time.Now()) {
			if *logInvalidAuthTokens {
				logger.Infof("jwt token is expired")
			}
			continue
		}

		return jce, true
	}

	return jwtVerified{}, false
}

func (jc *jwtCache) addVerifiedIfNotExist(at string, new jwtVerified) jwtVerified {
	jc.verifiedMux.Lock()
	defer jc.verifiedMux.Unlock()

	jv, ok := jc.verified[at]
	if !ok {
		jc.verified[at] = new
		jv = new
	}

	return jv
}

func (jc *jwtCache) removeExpired() {
	select {
	case <-jc.removeExpiredT.C:
	default:
		return
	}

	now := time.Now()
	jc.verifiedMux.Lock()
	for at, jui := range jc.verified {
		if jui.tkn.IsExpired(now) {
			delete(jc.verified, at)
		}
	}
	jc.verifiedMux.Unlock()
}
