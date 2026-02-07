package main

import (
	"fmt"
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

	// unauthorized_user cannot be used with jwt_token
	f(`
unauthorized_user:
  jwt_token: {skip_verify: true}
  url_prefix: http://foo.bar
`, `field jwt_token can't be specified for unauthorized_user section`)

	// username and jwt_token in a single config
	f(`
users:
- username: foo
  jwt_token: {skip_verify: true}
  url_prefix: http://foo.bar
`, `auth_token, bearer_token, username and password cannot be specified if jwt_token is set`)
	// bearer_token and jwt_token in a single config
	f(`
users:
- bearer_token: foo
  jwt_token: {skip_verify: true}
  url_prefix: http://foo.bar
`, `auth_token, bearer_token, username and password cannot be specified if jwt_token is set`)
	// bearer_token and jwt_token in a single config
	f(`
users:
- auth_token: "Foo token"
  jwt_token: {skip_verify: true}
  url_prefix: http://foo.bar
`, `auth_token, bearer_token, username and password cannot be specified if jwt_token is set`)

	// jwt_token public_keys or skip_verify must be set, part 1
	f(`
users:
- jwt_token: {}
  url_prefix: http://foo.bar
`, `jwt_token must contain at least a single public key or have skip_verify=true`)

	// jwt_token public_keys or skip_verify must be set, part 2
	f(`
users:
- jwt_token: {public_keys: null}
  url_prefix: http://foo.bar
`, `jwt_token must contain at least a single public key or have skip_verify=true`)

	// jwt_token public_keys or skip_verify must be set, part 3
	f(`
users:
- jwt_token: {public_keys: []}
  url_prefix: http://foo.bar
`, `jwt_token must contain at least a single public key or have skip_verify=true`)

	// invalid public key, part 1
	f(`
users:
- jwt_token: {public_keys: [""]}
  url_prefix: http://foo.bar
`, `failed to parse key "": failed to decode PEM block containing public key`)

	// invalid public key, part 2
	f(`
users:
- jwt_token: {public_keys: ["invalid"]}
  url_prefix: http://foo.bar
`, `failed to parse key "invalid": failed to decode PEM block containing public key`)

	// invalid public key, part 2
	f(fmt.Sprintf(`
users:
- jwt_token: 
    public_keys:
    - %q
    - %q
    - "invalid"
  url_prefix: http://foo.bar
`, validRSAPublicKey, validECDSAPublicKey), `failed to parse key "invalid": failed to decode PEM block containing public key`)

	// several jwt_token users
	// invalid public key, part 2
	f(fmt.Sprintf(`
users:
- jwt_token: 
    public_keys:
    - %q
  url_prefix: http://foo.bar
- jwt_token:
    public_keys:
    - %q
  url_prefix: http://foo.bar
`, validRSAPublicKey, validECDSAPublicKey), `multiple users with JWT tokens are not supported; found 2 users`)
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
			if ui.JWTToken == nil {
				t.Fatalf("unexpected nil JWTToken")
			}

			if ui.JWTToken.SkipVerify {
				if ui.JWTToken.verifierPool != nil {
					t.Fatalf("unexpected non-nil verifier pool for skip_verify=true")
				}
				continue
			}

			if ui.JWTToken.verifierPool == nil {
				t.Fatalf("unexpected nil verifier pool for non-empty public keys")
			}
		}
	}

	f(fmt.Sprintf(`
users:
- jwt_token:
    public_keys:
    - %q
  url_prefix: http://foo.bar
`, validRSAPublicKey))

	f(fmt.Sprintf(`
users:
- jwt_token:
    public_keys:
    - %q
  url_prefix: http://foo.bar
`, validECDSAPublicKey))

	f(fmt.Sprintf(`
users:
- jwt_token:
    public_keys:
    - %q
    - %q
  url_prefix: http://foo.bar
`, validRSAPublicKey, validECDSAPublicKey))

	f(`
users:
- jwt_token:
    skip_verify: true
  url_prefix: http://foo.bar
`)

	// combined with other auth methods
	f(`
users:
- username: foo
  password: bar
  url_prefix: http://foo.bar

- jwt_token:
    skip_verify: true
  url_prefix: http://foo.bar

- bearer_token: foo
  url_prefix: http://foo.bar
`)
}
