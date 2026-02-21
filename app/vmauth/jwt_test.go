package main

import (
	"fmt"
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
				t.Fatalf("unexpected error; got %q; want %q", err.Error(), expErr)
			}
			return
		}
		users, err := parseJWTUsers(ac)
		if err != nil {
			if expErr != err.Error() {
				t.Fatalf("unexpected error; got %q; want %q", err.Error(), expErr)
			}
			return
		}
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
`, `jwt must contain at least a single public key, public_key_files or have skip_verify=true`)

	// jwt public_keys or skip_verify must be set, part 2
	f(`
users:
- jwt: {public_keys: null}
  url_prefix: http://foo.bar
`, `jwt must contain at least a single public key, public_key_files or have skip_verify=true`)

	// jwt public_keys or skip_verify must be set, part 3
	f(`
users:
- jwt: {public_keys: []}
  url_prefix: http://foo.bar
`, `jwt must contain at least a single public key, public_key_files or have skip_verify=true`)

	// jwt public_keys, public_key_files or skip_verify must be set
	f(`
users:
- jwt: {public_key_files: []}
  url_prefix: http://foo.bar
`, `jwt must contain at least a single public key, public_key_files or have skip_verify=true`)

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
  url_prefix: http://foo.bar/{{.UnsupportedPlaceholder}}/foo
`, "invalid placeholder found in URL or headers; allowed placeholders are: {{.MetricsTenant}}, {{.MetricsExtraLabels}}, {{.MetricsExtraFilters}}, {{.LogsAccountID}}, {{.LogsProjectID}}, {{.LogsExtraFilters}}, {{.LogsExtraStreamFilters}}")

	// unsupported placeholder in a header
	f(`
users:
- jwt: 
    skip_verify: true
  headers:
    - "AccountID: {{.UnsupportedPlaceholder}}"
  url_prefix: http://foo.bar
`, "invalid placeholder found in URL or headers; allowed placeholders are: {{.MetricsTenant}}, {{.MetricsExtraLabels}}, {{.MetricsExtraFilters}}, {{.LogsAccountID}}, {{.LogsProjectID}}, {{.LogsExtraFilters}}, {{.LogsExtraStreamFilters}}")

	// spaces in templating not allowed
	f(`
users:
- jwt: 
    skip_verify: true
  headers:
    - "AccountID: {{ .LogsAccountID }}"
  url_prefix: http://foo.bar
`, "invalid placeholder found in URL or headers; allowed placeholders are: {{.MetricsTenant}}, {{.MetricsExtraLabels}}, {{.MetricsExtraFilters}}, {{.LogsAccountID}}, {{.LogsProjectID}}, {{.LogsExtraFilters}}, {{.LogsExtraStreamFilters}}")
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

		jui, err := parseJWTUsers(ac)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		for _, ui := range jui {
			if ui.JWT == nil {
				t.Fatalf("unexpected nil JWTConfig")
			}

			if ui.JWT.SkipVerify {
				if ui.JWT.verifierPool != nil {
					t.Fatalf("unexpected non-nil verifier pool for skip_verify=true")
				}
				continue
			}

			if ui.JWT.verifierPool == nil {
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
}
