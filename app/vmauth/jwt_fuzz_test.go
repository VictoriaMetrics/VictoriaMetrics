package main

import (
	"net/url"
	"testing"

	"github.com/VictoriaMetrics/metrics"
	"sigs.k8s.io/randfill"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
)

func FuzzParseJWTUsers(f *testing.F) {
	f.Add("role", "admin", uint8(0))
	f.Add("role", "admin", uint8(1))
	f.Add("role", "admin", uint8(2))
	f.Add("role", "/nonexistent", uint8(3))
	f.Add("role", "", uint8(4))
	f.Add("role", "://", uint8(4))
	f.Add("role", "ftp://localhost", uint8(4))
	f.Add("role", "admin", uint8(5))
	f.Add("role", "[", uint8(1))
	f.Add("", "", uint8(2))

	f.Fuzz(func(t *testing.T, claim, pattern string, mode uint8) {
		if len(claim)+len(pattern) > 4096 {
			t.Skip()
		}

		jwtCfg := &JWTConfig{}
		// Fill YAML fields; parsing builds claims and verifier state.
		randfill.NewFromGoFuzz([]byte(claim+"\x00"+pattern)).
			NilChance(0.2).
			MaxDepth(5).
			NumElements(0, 3).
			Funcs(
				func(cfg *JWTConfig, c randfill.Continue) {
					// Fill every field except internal parsedMatchClaims and verifierPool.
					c.Fill(&cfg.PublicKeys)
					c.Fill(&cfg.PublicKeyFiles)
					c.Fill(&cfg.SkipVerify)
					c.Fill(&cfg.OIDC)
					c.Fill(&cfg.MatchClaims)
					c.Fill(&cfg.DefaultVMAccessClaim)
					c.Fill(&cfg.ProxyCookieAuthorizationToken)
				},
				func(v *jwt.VMAccessClaim, c randfill.Continue) {
					// Fill every field except deprecated fields and deprecated buffer.
					c.Fill(&v.MetricsExtraFilters)
					c.Fill(&v.MetricsExtraLabels)
					c.Fill(&v.LogsExtraFilters)
					c.Fill(&v.LogsExtraStreamFilters)
					c.Fill(&v.MetricsAccountID)
					c.Fill(&v.MetricsProjectID)
					c.Fill(&v.LogsAccountID)
					c.Fill(&v.LogsProjectID)
				},
			).
			Fill(jwtCfg)
		// Keep seed modes valid; other modes also exercise generated claims.
		if mode < 6 {
			clear(jwtCfg.MatchClaims)
		}
		if jwtCfg.MatchClaims == nil {
			jwtCfg.MatchClaims = make(map[string]string)
		}
		jwtCfg.MatchClaims[claim] = pattern
		jwtCfg.PublicKeys = nil
		jwtCfg.PublicKeyFiles = nil
		jwtCfg.SkipVerify = false
		jwtCfg.OIDC = nil
		switch mode % 6 {
		case 0:
			jwtCfg.SkipVerify = true
		case 1:
			jwtCfg.OIDC = &oidcConfig{Issuer: "https://localhost"}
		case 2:
			jwtCfg.PublicKeys = []string{pattern}
		case 3:
			jwtCfg.PublicKeyFiles = []string{pattern}
		case 4:
			jwtCfg.OIDC = &oidcConfig{Issuer: pattern}
		case 5:
			jwtCfg.OIDC = &oidcConfig{Issuer: "https://localhost"}
			jwtCfg.SkipVerify = true
		}
		ac := &AuthConfig{
			ms: metrics.NewSet(),
			Users: []UserInfo{{
				JWT:       jwtCfg,
				URLPrefix: &URLPrefix{busOriginal: []*url.URL{{Scheme: "http", Host: "localhost:8428"}}},
			}},
		}
		pool := &oidcDiscovererPool{}
		_, _ = parseJWTUsers(ac, pool)
		pool.stopDiscovery()
	})
}
