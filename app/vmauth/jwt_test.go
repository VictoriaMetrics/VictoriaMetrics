package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestJWTParseAuthConfigFailure(t *testing.T) {
	validRSAPublicKey := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAiX7oPWKOWRQsGFEWvwZO
mL2PYsdYUsu9nr0qtPCjxQHUJgLfT3rdKlvKpPFYv7ZmKnqTncg36Wz9uiYmWJ7e
IB5Z+fko8kVIMzarCqVvpAJDzYF/pUii68xvuYoK3L9TIOAeyCXv+prwnr2IH+Mw
9AONzWbRrYoO74XyTE9vMU5qmI/L1VPk+PR8lqPOSptLvzsfoaIk2ED4yK2nRB+6
st+k4nccPqbErqHc8aiXnXfugfnr6b+NPFYUzKsDqkymGOokVijrI8B3jNw6c6Do
zphk+D3wgLsXYHfMcZbXIMqffqm/aB8Qg88OpFOkQ3rd2p6R9+hacnZkfkn3Phiw
yQIDAQAB
-----END PUBLIC KEY-----
`
	// ECDSA with the P-521 curve
	validECDSAPublicKey := `-----BEGIN PUBLIC KEY-----
MIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQAU9RmtkCRuYTKCyvLlDn5DtBZOHSe
QTa5j9q/oQVpCKqcXVFrH5dgh0GL+P/ZhkeuowPzCZqntGf0+7wPt9OxSJcADVJm
dv92m540MXss8zdHf5qtE0gsu2Ved0R7Z8a8QwGZ/1mYZ+kFGGbdQTlSvRqDySTq
XOtclIk1uhc03oL9nOQ=
-----END PUBLIC KEY-----
`

	f := func(s string, expErr string) {
		t.Helper()
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			if expErr != err.Error() {
				t.Fatalf("unexpected error; got\n%q\nwant\n%q", err.Error(), expErr)
			}
			return
		}
		users, ds, err := parseJWTUsers(ac)
		if err != nil {
			if expErr != err.Error() {
				t.Fatalf("unexpected error; got\n%q\nwant \n%q", err.Error(), expErr)
			}
			return
		}
		ds.stop()
		t.Fatalf("expecting non-nil error; got %v", users)
	}

	// unauthorized_user cannot be used with jwt
	f(`
unauthorized_user:
  jwt: {skip_verify: true}
  url_prefix: http://foo.bar
`, `field jwt can't be specified for unauthorized_user section`)

	// username and jwt in a single config
	f(`
users:
- username: foo
  jwt: {skip_verify: true}
  url_prefix: http://foo.bar
`, `auth_token, bearer_token, username and password cannot be specified if jwt is set`)
	// bearer_token and jwt in a single config
	f(`
users:
- bearer_token: foo
  jwt: {skip_verify: true}
  url_prefix: http://foo.bar
`, `auth_token, bearer_token, username and password cannot be specified if jwt is set`)
	// bearer_token and jwt in a single config
	f(`
users:
- auth_token: "Foo token"
  jwt: {skip_verify: true}
  url_prefix: http://foo.bar
`, `auth_token, bearer_token, username and password cannot be specified if jwt is set`)

	// jwt public_keys or skip_verify must be set, part 1
	f(`
users:
- jwt: {}
  url_prefix: http://foo.bar
`, `jwt must contain at least a single public key, public_key_files, oidc or have skip_verify=true`)

	// jwt public_keys or skip_verify must be set, part 2
	f(`
users:
- jwt: {public_keys: null}
  url_prefix: http://foo.bar
`, `jwt must contain at least a single public key, public_key_files, oidc or have skip_verify=true`)

	// jwt public_keys or skip_verify must be set, part 3
	f(`
users:
- jwt: {public_keys: []}
  url_prefix: http://foo.bar
`, `jwt must contain at least a single public key, public_key_files, oidc or have skip_verify=true`)

	// jwt public_keys, public_key_files or skip_verify must be set
	f(`
users:
- jwt: {public_key_files: []}
  url_prefix: http://foo.bar
`, `jwt must contain at least a single public key, public_key_files, oidc or have skip_verify=true`)

	// invalid public key, part 1
	f(`
users:
- jwt: {public_keys: [""]}
  url_prefix: http://foo.bar
`, `failed to parse key "": failed to decode PEM block containing public key`)

	// invalid public key, part 2
	f(`
users:
- jwt: {public_keys: ["invalid"]}
  url_prefix: http://foo.bar
`, `failed to parse key "invalid": failed to decode PEM block containing public key`)

	// invalid public key, part 2
	f(fmt.Sprintf(`
users:
- jwt: 
    public_keys:
    - %q
    - %q
    - "invalid"
  url_prefix: http://foo.bar
`, validRSAPublicKey, validECDSAPublicKey), `failed to parse key "invalid": failed to decode PEM block containing public key`)

	// several jwt users
	// invalid public key, part 2
	f(fmt.Sprintf(`
users:
- jwt: 
    public_keys:
    - %q
  url_prefix: http://foo.bar
- jwt:
    public_keys:
    - %q
  url_prefix: http://foo.bar
`, validRSAPublicKey, validECDSAPublicKey), `multiple users with JWT tokens are not supported; found 2 users`)

	// public key file doesn't exist
	f(`
users:
- jwt: 
    public_key_files: 
    - /path/to/nonexistent/file.pem
  url_prefix: http://foo.bar
`, "cannot read public key from file \"/path/to/nonexistent/file.pem\": open /path/to/nonexistent/file.pem: no such file or directory")

	// public key file invalid
	// auth with key from file
	publicKeyFile := filepath.Join(t.TempDir(), "a_public_key.pem")
	if err := os.WriteFile(publicKeyFile, []byte(`invalidPEM`), 0o644); err != nil {
		t.Fatalf("failed to write public key file: %s", err)
	}
	f(`
users:
- jwt: 
    public_key_files: 
    - `+publicKeyFile+`
  url_prefix: http://foo.bar
`, "cannot parse public key from file \""+publicKeyFile+"\": failed to parse key \"invalidPEM\": failed to decode PEM block containing public key")

	// unsupported placeholder in a header
	f(`
users:
- jwt: 
    skip_verify: true
  url_prefix: http://foo.bar/{{.UnsupportedPlaceholder}}/foo`,
		"invalid placeholder found in URL request path: \"/{{.UnsupportedPlaceholder}}/foo\", supported values are: {{.MetricsTenant}}, {{.MetricsExtraLabels}}, {{.MetricsExtraFilters}}, {{.LogsAccountID}}, {{.LogsProjectID}}, {{.LogsExtraFilters}}, {{.LogsExtraStreamFilters}}",
	)
	// unsupported placeholder in a header
	f(`
users:
- jwt: 
    skip_verify: true
  headers:
    - "AccountID: {{.UnsupportedPlaceholder}}"
  url_prefix: http://foo.bar
`,
		"request header: \"AccountID\" has unsupported placeholder: \"{{.UnsupportedPlaceholder}}\", supported values are: {{.MetricsTenant}}, {{.MetricsExtraLabels}}, {{.MetricsExtraFilters}}, {{.LogsAccountID}}, {{.LogsProjectID}}, {{.LogsExtraFilters}}, {{.LogsExtraStreamFilters}}",
	)

	// spaces in templating not allowed
	f(`
users:
- jwt: 
    skip_verify: true
  headers:
    - "AccountID: {{ .LogsAccountID }}"
  url_prefix: http://foo.bar
`,
		"request header: \"AccountID\" has unsupported placeholder: \"{{ .LogsAccountID }}\", supported values are: {{.MetricsTenant}}, {{.MetricsExtraLabels}}, {{.MetricsExtraFilters}}, {{.LogsAccountID}}, {{.LogsProjectID}}, {{.LogsExtraFilters}}, {{.LogsExtraStreamFilters}}",
	)

	// oidc is not an object
	f(`
users:
- jwt: 
    oidc: "not an object"
  url_prefix: http://foo.bar
`,
		"cannot unmarshal AuthConfig data: yaml: unmarshal errors:\n  line 4: cannot unmarshal !!str `not an ...` into main.OIDCConfig",
	)

	// oidc issuer empty
	f(`
users:
- jwt: 
    oidc: {}
  url_prefix: http://foo.bar
`,
		"oidc issuer cannot be empty",
	)

	// oidc and public_keys are not allowed
	f(fmt.Sprintf(`
users:
- jwt: 
    public_keys:
    - %q
    oidc: 
      issuer: https://example.com
  url_prefix: http://foo.bar
`, validRSAPublicKey),
		"jwt with oidc cannot contain public keys or have skip_verify=true",
	)

	// oidc and skip_verify are not allowed
	f(`
users:
- jwt: 
    skip_verify: true
    oidc: 
      issuer: https://example.com
  url_prefix: http://foo.bar
`,
		"jwt with oidc cannot contain public keys or have skip_verify=true",
	)
}

func TestJWTParseAuthConfigSuccess(t *testing.T) {
	validRSAPublicKey := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAiX7oPWKOWRQsGFEWvwZO
mL2PYsdYUsu9nr0qtPCjxQHUJgLfT3rdKlvKpPFYv7ZmKnqTncg36Wz9uiYmWJ7e
IB5Z+fko8kVIMzarCqVvpAJDzYF/pUii68xvuYoK3L9TIOAeyCXv+prwnr2IH+Mw
9AONzWbRrYoO74XyTE9vMU5qmI/L1VPk+PR8lqPOSptLvzsfoaIk2ED4yK2nRB+6
st+k4nccPqbErqHc8aiXnXfugfnr6b+NPFYUzKsDqkymGOokVijrI8B3jNw6c6Do
zphk+D3wgLsXYHfMcZbXIMqffqm/aB8Qg88OpFOkQ3rd2p6R9+hacnZkfkn3Phiw
yQIDAQAB
-----END PUBLIC KEY-----
`
	// ECDSA with the P-521 curve
	validECDSAPublicKey := `-----BEGIN PUBLIC KEY-----
MIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQAU9RmtkCRuYTKCyvLlDn5DtBZOHSe
QTa5j9q/oQVpCKqcXVFrH5dgh0GL+P/ZhkeuowPzCZqntGf0+7wPt9OxSJcADVJm
dv92m540MXss8zdHf5qtE0gsu2Ved0R7Z8a8QwGZ/1mYZ+kFGGbdQTlSvRqDySTq
XOtclIk1uhc03oL9nOQ=
-----END PUBLIC KEY-----
`

	f := func(s string) {
		t.Helper()
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		jui, ds, err := parseJWTUsers(ac)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		defer ds.stop()

		for _, ui := range jui {
			if ui.JWT == nil {
				t.Fatalf("unexpected nil JWTConfig")
			}

			if ui.JWT.SkipVerify {
				if ui.JWT.verifierPool.Load() != nil {
					t.Fatalf("unexpected non-nil verifier pool for skip_verify=true")
				}
				continue
			}

			if ui.JWT.verifierPool.Load() == nil {
				t.Fatalf("unexpected nil verifier pool for non-empty public keys")
			}
		}
	}

	f(fmt.Sprintf(`
users:
- jwt:
    public_keys:
    - %q
  url_prefix: http://foo.bar
`, validRSAPublicKey))

	f(fmt.Sprintf(`
users:
- jwt:
    public_keys:
    - %q
  url_prefix: http://foo.bar
`, validECDSAPublicKey))

	f(fmt.Sprintf(`
users:
- jwt:
    public_keys:
    - %q
    - %q
  url_prefix: http://foo.bar
`, validRSAPublicKey, validECDSAPublicKey))

	f(`
users:
- jwt:
    skip_verify: true
  url_prefix: http://foo.bar
`)

	// combined with other auth methods
	f(`
users:
- username: foo
  password: bar
  url_prefix: http://foo.bar

- jwt:
    skip_verify: true
  url_prefix: http://foo.bar

- bearer_token: foo
  url_prefix: http://foo.bar
`)

	rsaKeyFile := filepath.Join(t.TempDir(), "rsa_public_key.pem")
	if err := os.WriteFile(rsaKeyFile, []byte(validRSAPublicKey), 0o644); err != nil {
		t.Fatalf("failed to write RSA key file: %s", err)
	}
	ecdsaKeyFile := filepath.Join(t.TempDir(), "ecdsa_public_key.pem")
	if err := os.WriteFile(ecdsaKeyFile, []byte(validECDSAPublicKey), 0o644); err != nil {
		t.Fatalf("failed to write ECDSA key file: %s", err)
	}

	// Test single public key file
	f(fmt.Sprintf(`
users:
- jwt:
    public_key_files:
    - %q
  url_prefix: http://foo.bar
`, rsaKeyFile))

	// Test multiple public key files
	f(fmt.Sprintf(`
users:
- jwt:
    public_key_files:
    - %q
    - %q
  url_prefix: http://foo.bar
`, rsaKeyFile, ecdsaKeyFile))

	// Test combined inline keys and files
	f(fmt.Sprintf(`
users:
- jwt:
    public_keys:
    - %q
    public_key_files:
    - %q
  url_prefix: http://foo.bar
`, validECDSAPublicKey, rsaKeyFile))

	// oidc stub server
	var ipSrv *httptest.Server
	ipSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"issuer":   ipSrv.URL,
				"jwks_uri": fmt.Sprintf("%s/jwks", ipSrv.URL),
			})
			return
		}
		if r.URL.Path == "/jwks" {
			// reps generated by https://jwkset.com/generate
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`
{
  "keys": [
    {
      "kty": "RSA",
      "kid": "f13eee91-f566-4829-80fa-fca847c21f0e",
      "d": "Ua1llEFz3LZ05CrK5a2JxKMUEWJGXhBPPF20hHQjzxd1w0IEJK_mhPZQG8dNtBROBNIi1FC9l6QRw-RTnVIVat5Xy4yDFNKXXL3ZLXejOHY8SXrNEIDqQ-cSwIpK9cK7Umib0PcPeEeeAED5mqDH75D8_YssWFF18kLbNB5Z9pZmn6Fshiht7l2Sh4GN-KcReOW6eiQQwckDte3OGmZCRbtEriLWJt5TUGUvfZVIlcclqNMycNB6jGa9E1pO5Up7Ki3ZbI_-6XmRgZPtqnR9oLJ1zn3fj3hYpCXo-zcqLuOu3qxcslsq5igsfBzgGtfIJHY9LfWmHUsaDEa5cAX1gQ",
      "n": "xbLXXBTNREk70UCMiqZ53_mTzYh89W-UaPU61GZ-RZ5lYcLgyWOb5mdyRbvJpcgfZpsOeGAUWbk3GkQ4vqn8kUMnnWhUum2Qk9kGubOJGLW6yaURd00j3E-ilQ5xO2R_Hzz8bAojxV8GKdGTQ-iTf8z8nsSHH8kR2SERbNJCFFtwtFU7vyFWyoH4Lmvu2UpICTHFCR9RqwQVjyoKB1JjJ6Dh1L4zPTlsvQEnqoeFQHPYr0QcQSMYXdfPvlt_FiLOAOE89fX_9T2r9WbFAoda3uTRE5_aal0jxUU2cFyeVSIgauNtF07fp422XFb4XPkWQWrdNx0KX53laSIYQ9HOpw",
      "e": "AQAB",
      "p": "2JT57AD-Q2lamgjgyn0wL7DgYZ3OoCTTrDm5_NHg6h13uDvyIlXSukuUeWm4tzPSDedpstbS7dgXkLw5eQXBHwPYtByTcEZS8Z37CBnhMOOhfo_U1aNIPPanJACvWBgz47-TxHsxW1YhztZqghRoicBZPSSBAj49MgANJ4jF0zc",
      "q": "6a4MkeSXJI-ZzQ-bgP8hwJqpLFr0AiNGQcjZMH4Nn4CPGdnGiqqe6flhfLimgbNhbb67B0-8fLIji8zGhGKDL_JSIpAAdmfs2vzeEsY2hScrqVbd1VbfRcRh0J6lsn7obxkbvQthp9sX2DQbeDcEeaFEvd9gDKQSATYEqWo7eBE",
      "dp": "haL2yu6Z9RJuuxi7S3YPY33qFZF_y0St71j3L854zzw7gMxMTW9TRWwZQwk-1pv9AmNFzvnK0MNDVyUs-UXZsb932TrApshdqYRnPsppLvdl0GgDVYcYrbUr0IUzrFHSwraVAOlavRbaaXvX4EejcUvkRFvf1nh83fs2Iqy8E-U",
      "dq": "Cnf5qC-Ndd3ZDg688LJ9WJuVKJ-Kfu4Fn7zXvgxnn9Wqk4XmFyA9rk21yFidXQIkQz5gMpun3g48-W5bFmMzbVp1w4af_q35NnZNnJm0p5Jxqkxx87TIm9-IYkg5NB3rW87MJ1PzNAnkr5LmCCSu1qQa6Eaxjt9qzxMUcmKH94E",
      "qi": "saAeU11iaKHmye3cwCAYkegcyWbXV3xIXEVJtS9Af_yM19UhspwY2VhuwRaajcwYZwtvR9_ITmX9M-ea7uLdd7aDYO1fujC8NGbopeC4Hkr7yb5vTly3pfKf4h-3LwGGUucJUetdz1lmMIYiyuG4_gSf1yIEtPDLKzXiedgEMdI"
    }
  ]
}
`))
			return
		}

		http.NotFound(w, r)
	}))
	defer ipSrv.Close()

	f(`
users:
- jwt:
    oidc:
      issuer: ` + ipSrv.URL + `
  url_prefix: http://foo.bar
`)
}
