package main

import (
	"regexp"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzNormalizeSSOConfigs(f *testing.F) {
	f.Add([]byte("sso:\n- src_host: example.com\n  oidc:\n    issuer: https://localhost\n    client_id: test\n    client_secret: test\n    cookie_secret: 0123456789abcdef\n"))
	f.Add([]byte("sso: [null]"))
	f.Add([]byte("sso:\n- src_host: '['\n  oidc: {}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		ac, err := parseAuthConfig(data)
		if err != nil {
			return
		}
		_ = normalizeSSOConfigs(ac.SSO)
	})
}

func FuzzStructuredSSOConfigs(f *testing.F) {
	f.Add("example.com", "https://localhost", "10m", "prompt")
	f.Add("[", "ftp://localhost", "-1s", "state")
	f.Add("example.com", "https://localhost", "", "")
	f.Add("example.com", "https://localhost", "1h", "prompt")
	f.Add("example.com", "https://localhost", "0s", "prompt")

	f.Fuzz(func(t *testing.T, host, issuer, duration, param string) {
		if len(host)+len(issuer)+len(duration)+len(param) > 4096 {
			t.Skip()
		}

		// Seed required fields so normalization can reach deeper branches.
		cfg := ssoConfig{
			SrcHost: &Regex{sOriginal: host},
			OIDC: &ssoOIDCConfig{
				Issuer: issuer, ClientID: "fuzz", ClientSecret: "fuzz",
				CookieSecret: "0123456789abcdef", SessionDuration: duration,
			},
		}
		// Fuzz config fields, excluding runtime state.
		r := randfill.NewFromGoFuzz([]byte(host+"\x00"+issuer+"\x00"+duration+"\x00"+param)).
			NilChance(0.2).
			MaxDepth(5).
			NumElements(0, 3).
			SkipFieldsWithPattern(regexp.MustCompile("^(sessionDuration|pm)$"))
		r.Fill(cfg.OIDC)
		_ = cfg.normalize()
	})
}
