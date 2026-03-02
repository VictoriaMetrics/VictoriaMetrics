package jwt

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestParseJWTHeader_Failure(t *testing.T) {
	f := func(data, expectedErr string, encode bool) {
		t.Helper()
		if encode {
			encodedLen := base64.RawURLEncoding.EncodedLen(len(data))
			encoded := make([]byte, encodedLen)
			base64.RawURLEncoding.Encode(encoded, []byte(data))
			data = string(encoded)
		}
		if _, err := parseJWTHeader(data); err != nil {
			if err.Error() != expectedErr {
				t.Errorf("unexpected error message: \ngot\n%s\nwant\n%s", err.Error(), expectedErr)
			}
		} else {
			t.Errorf("expecting non-nil error")
		}
	}

	// invalid input
	f(
		`bad input`,
		`cannot decode jwt header as b64: cannot decode jwt body as b64: illegal base64 data at input byte 3`,
		false,
	)

	// invalid b644
	f(
		`YmFk`,
		`cannot parse jwt header: invalid character 'b' looking for beginning of value`,
		false,
	)

	// invalid header json
	f(`{]`,
		`cannot parse jwt header: invalid character ']' looking for beginning of object key string`,
		true,
	)

	// invalid header type json
	f(`[]`,
		`cannot parse jwt header: json: cannot unmarshal array into Go value of type jwt.header`,
		true,
	)
}

func TestParseJWTHeader_Success(t *testing.T) {
	f := func(data string, expected *header) {
		t.Helper()
		encodedLen := base64.RawURLEncoding.EncodedLen(len(data))
		encoded := make([]byte, encodedLen)
		base64.RawURLEncoding.Encode(encoded, []byte(data))
		header, err := parseJWTHeader(string(encoded))
		if err != nil {
			t.Fatalf("parseJWTHeader() error: %s", err)
		}
		if !reflect.DeepEqual(header, expected) {
			t.Fatalf("unexpected token header;\ngot\n%v\nwant\n%v", header, expected)
		}
	}

	// parse ok supported algorithms
	supportedAlgorithms := []string{
		"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512",
	}
	for i := range supportedAlgorithms {
		f(fmt.Sprintf(`{
			"alg": %q,
			"kid": "test"
		}`, supportedAlgorithms[i]),
			&header{
				Alg: supportedAlgorithms[i],
				Kid: "test",
			},
		)
	}
}

func TestParseJWTBody_Failure(t *testing.T) {
	f := func(data, expectedErr string, encode bool) {
		t.Helper()
		if encode {
			encodedLen := base64.RawURLEncoding.EncodedLen(len(data))
			encoded := make([]byte, encodedLen)
			base64.RawURLEncoding.Encode(encoded, []byte(data))
			data = string(encoded)
		}
		if _, err := parseJWTBody(data); err != nil {
			if err.Error() != expectedErr {
				t.Errorf("unexpected error message: \ngot\n%s\nwant\n%s", err.Error(), expectedErr)
			}
		} else {
			t.Errorf("expecting non-nil error")
		}
	}

	// invalid input
	f(
		`bad input`,
		`cannot decode jwt body as b64: cannot decode jwt body as b64: illegal base64 data at input byte 3`,
		false,
	)

	// invalid b644
	f(
		`YmFk`,
		`cannot parse jwt body: invalid character 'b' looking for beginning of value`,
		false,
	)

	// invalid body json
	f(
		`{]`,
		`cannot parse jwt body: invalid character ']' looking for beginning of object key string`,
		true,
	)

	// invalid body type json
	f(
		`[]`,
		`cannot parse jwt body: json: cannot unmarshal array into Go value of type jwt.tbody`,
		true,
	)

	// missing vm_access claim
	f(
		`{}`,
		"missing `vm_access` claim",
		true,
	)

	// vm_access claim invalid type
	f(
		`{"vm_access": 123}`,
		"cannot parse jwt body vm_access: json: cannot unmarshal number into Go value of type string",
		true,
	)

	// vm_access claim null
	f(
		`{"vm_access": null}`,
		"missing `vm_access` claim",
		true,
	)

	// invalid vm_access: account_id type mismatch
	f(
		`{"vm_access": {"tenant_id": {"account_id": "1", "project_id": 5}}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid vm_access: project_id type mismatch
	f(
		`{"vm_access": {"tenant_id": {"account_id": 1, "project_id": "5"}}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid vm_access: extra_label type mismatch
	f(`
{
	"vm_access": {
		"extra_labels": [{
			"project": "dev",
			"team": "mobile"
		}],
		"tenant_id": {
			"account_id": 1,
			"project_id": 5
		}
	}
}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid vm_access: extra_filters type mismatch
	f(`
{
	"vm_access": {
		"extra_filters": [{}],
		"tenant_id": {
			"account_id": 1,
			"project_id": 5
		}
	}
}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid exp claim value type
	f(
		`{"exp": "1610976189", "vm_access": {}}`,
		`cannot parse jwt body: json: cannot unmarshal string into Go struct field tbody.exp of type int64`,
		true,
	)

	// invalid metrics metrics_account_id claim value type
	f(
		`{"vm_access": {"metrics_account_id": "1"}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid metrics metrics_project_id claim value type
	f(
		`{"vm_access": {"metrics_project_id": "1"}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid metrics metrics_extra_labels claim value type
	f(
		`{"vm_access": {"metrics_extra_labels": "aString"}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid metrics metrics_extra_filters claim value type
	f(
		`{"vm_access": {"metrics_extra_filters": "aString"}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid metrics logs_account_id claim value type
	f(
		`{"vm_access": {"logs_account_id": "1"}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid metrics logs_project_id claim value type
	f(
		`{"vm_access": {"logs_project_id": "1"}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid metrics logs_extra_filters claim value type
	f(
		`{"vm_access": {"logs_extra_filters": "aString"}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)

	// invalid metrics logs_extra_stream_filters claim value type
	f(
		`{"vm_access": {"logs_extra_stream_filters": "aString"}}`,
		`cannot parse jwt body vm_access: json: cannot unmarshal object into Go value of type string`,
		true,
	)
}

func TestParseJWTBody_Success(t *testing.T) {
	f := func(data string, resultExpected *body) {
		t.Helper()

		encodedLen := base64.RawURLEncoding.EncodedLen(len(data))
		encoded := make([]byte, encodedLen)
		base64.RawURLEncoding.Encode(encoded, []byte(data))

		result, err := parseJWTBody(string(encoded))
		if err != nil {
			t.Fatalf("parseJWTBody() error: %s", err)
		}
		if result.Exp != resultExpected.Exp {
			t.Fatalf("unexpected Exp; got %d; want %d", result.Exp, resultExpected.Exp)
		}
		if result.Iat != resultExpected.Iat {
			t.Fatalf("unexpected Iat; got %d; want %d", result.Iat, resultExpected.Iat)
		}
		if result.Scope != resultExpected.Scope {
			t.Fatalf("unexpected scope; got %q; want %q", result.Scope, resultExpected.Scope)
		}
		if result.Jti != resultExpected.Jti {
			t.Fatalf("unexpected jti; got %q; want %q", result.Jti, resultExpected.Jti)
		}
		if !reflect.DeepEqual(result.VMAccess.Tenant, resultExpected.VMAccess.Tenant) {
			t.Fatalf("unexpected tenant; got %v; want %v", result.VMAccess.Tenant, resultExpected.VMAccess.Tenant)
		}
		if !reflect.DeepEqual(result.VMAccess.Labels, resultExpected.VMAccess.Labels) {
			t.Fatalf("unexpected labels; got %v; want %v", result.VMAccess.Labels, resultExpected.VMAccess.Labels)
		}
		if !reflect.DeepEqual(result.VMAccess.ExtraFilters, resultExpected.VMAccess.ExtraFilters) {
			t.Fatalf("unexpected extra_filters; got %v; want %v", result.VMAccess.ExtraFilters, resultExpected.VMAccess.ExtraFilters)
		}
	}

	f(`{"vm_access": {}}`, &body{
		VMAccess: &VMAccessClaim{},
	})
	f(`{"vm_access": {"tenant_id": {}}}`, &body{
		VMAccess: &VMAccessClaim{},
	})

	f(
		`
{
    "vm_access": {
        "tenant_id": {
            "project_id": 5,
            "account_id": 1
        }
    }
}`,
		&body{
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 5,
					AccountID: 1,
				},
			},
		},
	)

	f(
		`
{
    "vm_access": {
        "extra_labels": {
            "project": "dev",
            "team": "mobile"
        }
    }
}`,
		&body{
			VMAccess: &VMAccessClaim{
				Labels: Labels{
					"project": "dev",
					"team":    "mobile",
				},
			},
		},
	)

	f(
		`
{
    "vm_access": {
        "extra_filters": [
             "{project=\"dev\"}",
             "{team=~\"mobile\"}"
         ]
    }
}`,
		&body{
			VMAccess: &VMAccessClaim{
				ExtraFilters: []string{
					`{project="dev"}`,
					`{team=~"mobile"}`,
				},
			},
		},
	)

	f(
		`
{
    "vm_access": {
        "tenant_id": {
            "project_id": 5,
            "account_id": 1
        },
        "extra_labels": {
            "project": "dev",
            "team": "mobile"
        },
        "extra_filters": [
             "{project=\"dev\"}",
             "{team=~\"mobile\"}"
         ]
    }
}`,
		&body{
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 5,
					AccountID: 1,
				},
				Labels: Labels{
					"project": "dev",
					"team":    "mobile",
				},
				ExtraFilters: []string{
					`{project="dev"}`,
					`{team=~"mobile"}`,
				},
			},
		},
	)

	f(
		`
{
    "exp": 1610976189,
    "iat": 1610975889,
    "jti": "9b194187-6bb7-4244-9d1b-559eab2ef7f3",
    "scope": "openid email profile",
    "vm_access": {}
}`,
		&body{
			Exp:      1610976189,
			Iat:      1610975889,
			Jti:      "9b194187-6bb7-4244-9d1b-559eab2ef7f3",
			Scope:    "openid email profile",
			VMAccess: &VMAccessClaim{},
		},
	)

	// metrics vm_access claim
	f(
		`
{
    "vm_access": {
        "metrics_account_id": 1,
        "metrics_project_id": 5,
        "metrics_extra_labels": [
            "project=dev",
            "team=mobile"
        ],
        "metrics_extra_filters": [
            "{project=\"dev\"}"
        ]
    }
}`,
		&body{
			VMAccess: &VMAccessClaim{
				MetricsAccountID: 1,
				MetricsProjectID: 5,
				MetricsExtraLabels: []string{
					"project=dev",
					"team=mobile",
				},
				MetricsExtraFilters: []string{
					`{project="dev"}`,
				},
			},
		},
	)

	// logs vm_access claim
	f(
		`
{
    "vm_access": {
        "logs_account_id": 1,
        "logs_project_id": 5,
        "logs_extra_filters": [
            "{\"namespace\":\"my-app\",\"env\":\"prod\"}"
        ],
        "logs_extra_stream_filters": [
            "{project=\"dev\"}"
        ]
    }
}`,
		&body{
			VMAccess: &VMAccessClaim{
				LogsAccountID: 1,
				LogsProjectID: 5,
				LogsExtraFilters: []string{
					`{"namespace":"my-app","env":"prod"}`,
				},
				LogsExtraStreamFilters: []string{
					`{project="dev"}`,
				},
			},
		},
	)
}

func TestNewTokenFromRequest_Failure(t *testing.T) {
	f := func(r *http.Request) {
		t.Helper()

		_, err := NewTokenFromRequestWithCustomHeader(r, "Authorization", false)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// missing header
	f(&http.Request{})

	// bad input
	f(&http.Request{
		Header: map[string][]string{
			"Authorization": {
				"Bearer fsfFSF",
			},
		},
	})

	// bad input malformed
	r := &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJhQVpvQ0d2dUdiRm9mdFdIeFFaeVJTUWVuM3lYNFUwR1BsUDVvWk9RU3djIn0.eyJleHAiOjE2MTA4ODkyNjYsImlhdCI6MTYxMDg4ODk2NiwiYXV0aF90aW1lIjoxNjEwODg4MDQ0LCJqdGkiOiIwOWEwNThhMi0wNzUyLTRlY2QtYTRlOS1iNjVlODVhZjQyM2YiLCJpc3MiOiJodHRwczovL2xvY2FsaG9zdDo4NDQzL2F1dGgvcmVhbG1zL3Rlc3QiLCJhdWQiOiJhY2NvdW50Iiwic3ViIjoiNDYwODU5NDEtYjkyYi00NzFhLWIwNWEtOTU5OWNhMjlkYTFlIiwidHlwIjoiQmVhcmVyIiwiYXpwIjoiZ3JhZmFuYSIsInNlc3Npb25fc3RhdGUiOiIzZGRjODc0OS1lZTI2LTQ2ODEtOWNlYy03M2U5YmIyZmRkOGUiLCJhY3IiOiIwIiwiYWxsb3dlZC1vcmlnaW5zIjpbImh0dHA6Ly9sb2NhbGhvc3Q6MzAwMCJdLCJyZWFsbV9hY2Nlc3MiOnsicm9sZXMiOlsib2ZmbGluZV9hY2Nlc3MiLCJ1bWFfYXV0aG9yaXphdGlvbiJdfSwicmVzb3VyY2VfYWNjZXNzIjp7ImFjY291bnQiOnsicm9sZXMiOlsibWFuYWdlLWFjY291bnQiLCJtYW5hZ2UtYWNjb3VudC1saW5rcyIsInZpZXctcHJvZmlsZSJdfX0sInNjb3BlIjoib3BlbmlkIGVtYWlsIHByb2ZpbGUiLCJ2bS1hY2Nlc3MiOnsibGFiZWxzIjp7InByb2plY3QiOiJkZXYiLCJ0ZWFtIjoibW9iaWxlIn0sInRlbmFudElEIjp7ImFjY291bnRJRCI6MSwicHJvamVjdElEIjo1fX0sImVtYWlsX3ZlcmlmaWVkIjpmYWxzZSwibmFtZSI6InRnIHRnIiwicHJvamVjdCI6Im1vYmlsZSIsInByZWZlcnJlZF91c2VybmFtZSI6InRnIiwidGVhbSI6ImRldiIsImdpdmVuX25hbWUiOiJ0ZyIsImZhbWlseV9uYW1lIjoidGciLCJlbWFpbCI6InRnQGZnaHQubmV0In0",
			},
		},
	}
	f(r)
}

func TestNewTokenFromRequest_Success(t *testing.T) {
	f := func(r *http.Request, resultExpected *Token, enforcePrefix bool) {
		t.Helper()

		result, err := NewTokenFromRequestWithCustomHeader(r, "Authorization", enforcePrefix)
		if err != nil {
			t.Fatalf("NewTokenFromRequest() error: %s", err)
		}
		if !reflect.DeepEqual(result.body.VMAccess, resultExpected.body.VMAccess) {
			t.Fatalf("unexpected token body VMAccess;\ngot\n%v\nwant\n%v", result.body.VMAccess, resultExpected.body.VMAccess)
		}
		if !reflect.DeepEqual(result.header, resultExpected.header) {
			t.Fatalf("unexpected token header\ngot\n%v\nwant\n%v", result.header, resultExpected.header)
		}
	}

	// parse ok
	r := &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJhQVpvQ0d2dUdiRm9mdFdIeFFaeVJTUWVuM3lYNFUwR1BsUDVvWk9RU3djIn0.eyJleHAiOjE2MTA5NzYxODksImlhdCI6MTYxMDk3NTg4OSwiYXV0aF90aW1lIjoxNjEwOTc1ODg5LCJqdGkiOiI5YjE5NDE4Ny02YmI3LTQyNDQtOWQxYi01NTllYWIyZWY3ZjMiLCJpc3MiOiJodHRwczovL2xvY2FsaG9zdDo4NDQzL2F1dGgvcmVhbG1zL3Rlc3QiLCJhdWQiOiJhY2NvdW50Iiwic3ViIjoiNDYwODU5NDEtYjkyYi00NzFhLWIwNWEtOTU5OWNhMjlkYTFlIiwidHlwIjoiQmVhcmVyIiwiYXpwIjoiZ3JhZmFuYSIsInNlc3Npb25fc3RhdGUiOiIxMzc3ZDEwMi03NTJiLTQ0ODYtOTlkYS1jMjA4MjRiODJkMzEiLCJhY3IiOiIxIiwiYWxsb3dlZC1vcmlnaW5zIjpbImh0dHA6Ly9sb2NhbGhvc3Q6MzAwMCJdLCJyZWFsbV9hY2Nlc3MiOnsicm9sZXMiOlsib2ZmbGluZV9hY2Nlc3MiLCJ1bWFfYXV0aG9yaXphdGlvbiJdfSwicmVzb3VyY2VfYWNjZXNzIjp7ImFjY291bnQiOnsicm9sZXMiOlsibWFuYWdlLWFjY291bnQiLCJtYW5hZ2UtYWNjb3VudC1saW5rcyIsInZpZXctcHJvZmlsZSJdfX0sInNjb3BlIjoib3BlbmlkIGVtYWlsIHByb2ZpbGUiLCJ2bV9hY2Nlc3MiOnsiZXh0cmFfbGFiZWxzIjp7InByb2plY3QiOiJkZXYiLCJ0ZWFtIjoibW9iaWxlIn0sInRlbmFudF9pZCI6eyJhY2NvdW50X2lkIjoxLCJwcm9qZWN0X2lkIjo1fX0sImVtYWlsX3ZlcmlmaWVkIjpmYWxzZSwibmFtZSI6InRnIHRnIiwicHJvamVjdCI6Im1vYmlsZSIsInByZWZlcnJlZF91c2VybmFtZSI6InRnIiwidGVhbSI6ImRldiIsImdpdmVuX25hbWUiOiJ0ZyIsImZhbWlseV9uYW1lIjoidGciLCJlbWFpbCI6InRnQGZnaHQubmV0In0.XErPkz-qL-EV8BBAR17MoFytc5ajYRz71f9_GOuG1AVcMnUsD6D3x4z5jL1dLyoGGm8OUW_RIVrjMpZf_xOfgQKRVHAMaJi64UtpwS8EF50mlOCDAdKl6wlzAS4laV3dW9W9QrTH7TMetG33WVsJGaD-MIwSYJ5peh6u__oniezsRavw8Qw3nLpZCQPb-NatT3Q1raj1ymLJErJPtUBSk3ieWCVpTMo4ZYKFIQt2wjHeOVOF_3suhPfhgEgXlN6aUq3xeYJ1aAtl_5Ao3pB2pto46kDSXIulQQuGdttsw7bSDOYqZ-tx3y7DBWNdIcghsO_iMvrA805j5hG4Nu84Sw",
			},
		},
	}
	resultExpected := &Token{
		body: &body{
			Exp:   1610889266,
			Iat:   1610888966,
			Jti:   "09a058a2-0752-4ecd-a4e9-b65e85af423f",
			Scope: "openid email profile",
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 5,
					AccountID: 1,
				},
				Labels: map[string]string{
					"project": "dev",
					"team":    "mobile",
				},
			},
		},
		header: &header{
			Alg: "RS256",
			Kid: "aAZoCGvuGbFoftWHxQZyRSQen3yX4U0GPlP5oZOQSwc",
			Typ: "JWT",
		},
	}
	f(r, resultExpected, true)

	// parse ok with non-standard "BEARER" prefix
	r = &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"BEARER eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJhQVpvQ0d2dUdiRm9mdFdIeFFaeVJTUWVuM3lYNFUwR1BsUDVvWk9RU3djIn0.eyJleHAiOjE2MTA5NzYxODksImlhdCI6MTYxMDk3NTg4OSwiYXV0aF90aW1lIjoxNjEwOTc1ODg5LCJqdGkiOiI5YjE5NDE4Ny02YmI3LTQyNDQtOWQxYi01NTllYWIyZWY3ZjMiLCJpc3MiOiJodHRwczovL2xvY2FsaG9zdDo4NDQzL2F1dGgvcmVhbG1zL3Rlc3QiLCJhdWQiOiJhY2NvdW50Iiwic3ViIjoiNDYwODU5NDEtYjkyYi00NzFhLWIwNWEtOTU5OWNhMjlkYTFlIiwidHlwIjoiQmVhcmVyIiwiYXpwIjoiZ3JhZmFuYSIsInNlc3Npb25fc3RhdGUiOiIxMzc3ZDEwMi03NTJiLTQ0ODYtOTlkYS1jMjA4MjRiODJkMzEiLCJhY3IiOiIxIiwiYWxsb3dlZC1vcmlnaW5zIjpbImh0dHA6Ly9sb2NhbGhvc3Q6MzAwMCJdLCJyZWFsbV9hY2Nlc3MiOnsicm9sZXMiOlsib2ZmbGluZV9hY2Nlc3MiLCJ1bWFfYXV0aG9yaXphdGlvbiJdfSwicmVzb3VyY2VfYWNjZXNzIjp7ImFjY291bnQiOnsicm9sZXMiOlsibWFuYWdlLWFjY291bnQiLCJtYW5hZ2UtYWNjb3VudC1saW5rcyIsInZpZXctcHJvZmlsZSJdfX0sInNjb3BlIjoib3BlbmlkIGVtYWlsIHByb2ZpbGUiLCJ2bV9hY2Nlc3MiOnsiZXh0cmFfbGFiZWxzIjp7InByb2plY3QiOiJkZXYiLCJ0ZWFtIjoibW9iaWxlIn0sInRlbmFudF9pZCI6eyJhY2NvdW50X2lkIjoxLCJwcm9qZWN0X2lkIjo1fX0sImVtYWlsX3ZlcmlmaWVkIjpmYWxzZSwibmFtZSI6InRnIHRnIiwicHJvamVjdCI6Im1vYmlsZSIsInByZWZlcnJlZF91c2VybmFtZSI6InRnIiwidGVhbSI6ImRldiIsImdpdmVuX25hbWUiOiJ0ZyIsImZhbWlseV9uYW1lIjoidGciLCJlbWFpbCI6InRnQGZnaHQubmV0In0.XErPkz-qL-EV8BBAR17MoFytc5ajYRz71f9_GOuG1AVcMnUsD6D3x4z5jL1dLyoGGm8OUW_RIVrjMpZf_xOfgQKRVHAMaJi64UtpwS8EF50mlOCDAdKl6wlzAS4laV3dW9W9QrTH7TMetG33WVsJGaD-MIwSYJ5peh6u__oniezsRavw8Qw3nLpZCQPb-NatT3Q1raj1ymLJErJPtUBSk3ieWCVpTMo4ZYKFIQt2wjHeOVOF_3suhPfhgEgXlN6aUq3xeYJ1aAtl_5Ao3pB2pto46kDSXIulQQuGdttsw7bSDOYqZ-tx3y7DBWNdIcghsO_iMvrA805j5hG4Nu84Sw",
			},
		},
	}
	resultExpected = &Token{
		body: &body{
			Exp:   1610889266,
			Iat:   1610888966,
			Jti:   "09a058a2-0752-4ecd-a4e9-b65e85af423f",
			Scope: "openid email profile",
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 5,
					AccountID: 1,
				},
				Labels: map[string]string{
					"project": "dev",
					"team":    "mobile",
				},
			},
		},
		header: &header{
			Alg: "RS256",
			Kid: "aAZoCGvuGbFoftWHxQZyRSQen3yX4U0GPlP5oZOQSwc",
			Typ: "JWT",
		},
	}
	f(r, resultExpected, true)

	// go-jwt
	r = &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2NDU1MzY3NTgsImlhdCI6MTY0NTUzNjYzOCwidm1fYWNjZXNzIjp7ImV4dHJhX2ZpbHRlcnMiOlsie25hbWVzcGFjZT1-XCJlaWZ0ZGkxLXRlc3RcIn0iXSwibW9kZSI6MSwidGVuYW50X2lkIjp7ImFjY291bnRfaWQiOjEsInByb2plY3RfaWQiOjB9fX0.4r3zE487ochfj_GgYRpbjmid5ktlBH0bKfz3Ut45Foc",
			},
		},
	}
	resultExpected = &Token{
		body: &body{
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 0,
					AccountID: 1,
				},
				ExtraFilters: []string{
					`{namespace=~"eiftdi1-test"}`,
				},
				Mode: 1,
			},
		},
		header: &header{
			Alg: "HS256",
			Typ: "JWT",
		},
	}
	f(r, resultExpected, true)

	// jwt-with-std-b64
	r = &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2NDU2MDY5OTgsImlhdCI6MTY0NTYwNjg3OCwidm1fYWNjZXNzIjp7ImV4dHJhX2ZpbHRlcnMiOlsie25hbWVzcGFjZT1+XCJlaWZ0ZGkxLXRlc3RcIn0iXSwibW9kZSI6MSwidGVuYW50X2lkIjp7ImFjY291bnRfaWQiOjEsInByb2plY3RfaWQiOjB9fX0.oAYJdff8DK4+P1oR6tBE1l2mq79p3eJ5crXlkO+CxcA",
			},
		},
	}
	resultExpected = &Token{
		body: &body{
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 0,
					AccountID: 1,
				},
				ExtraFilters: []string{
					`{namespace=~"eiftdi1-test"}`,
				},
				Mode: 1,
			},
		},
		header: &header{
			Alg: "HS256",
			Typ: "JWT",
		},
	}
	f(r, resultExpected, true)

	// parse ok with filters
	r = &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImFBWm9DR3Z1R2JGb2Z0V0h4UVp5UlNRZW4zeVg0VTBHUGxQNW9aT1FTd2MifQ.eyJleHAiOjE2MTA5NzYxODksImlhdCI6MTYxMDk3NTg4OSwiYXV0aF90aW1lIjoxNjEwOTc1ODg5LCJqdGkiOiI5YjE5NDE4Ny02YmI3LTQyNDQtOWQxYi01NTllYWIyZWY3ZjMiLCJpc3MiOiJodHRwczovL2xvY2FsaG9zdDo4NDQzL2F1dGgvcmVhbG1zL3Rlc3QiLCJhdWQiOiJhY2NvdW50Iiwic3ViIjoiNDYwODU5NDEtYjkyYi00NzFhLWIwNWEtOTU5OWNhMjlkYTFlIiwidHlwIjoiQmVhcmVyIiwiYXpwIjoiZ3JhZmFuYSIsInNlc3Npb25fc3RhdGUiOiIxMzc3ZDEwMi03NTJiLTQ0ODYtOTlkYS1jMjA4MjRiODJkMzEiLCJhY3IiOiIxIiwiYWxsb3dlZC1vcmlnaW5zIjpbImh0dHA6Ly9sb2NhbGhvc3Q6MzAwMCJdLCJyZWFsbV9hY2Nlc3MiOnsicm9sZXMiOlsib2ZmbGluZV9hY2Nlc3MiLCJ1bWFfYXV0aG9yaXphdGlvbiJdfSwicmVzb3VyY2VfYWNjZXNzIjp7ImFjY291bnQiOnsicm9sZXMiOlsibWFuYWdlLWFjY291bnQiLCJtYW5hZ2UtYWNjb3VudC1saW5rcyIsInZpZXctcHJvZmlsZSJdfX0sInNjb3BlIjoib3BlbmlkIGVtYWlsIHByb2ZpbGUiLCJ2bV9hY2Nlc3MiOnsiZXh0cmFfbGFiZWxzIjp7InByb2plY3QiOiJkZXYiLCJ0ZWFtIjoibW9iaWxlIn0sImV4dHJhX2ZpbHRlcnMiOlsie2Vudj1+XCJwcm9kfGRldlwifSIsInt0ZWFtIT1cInRlc3RcIn0iXSwidGVuYW50X2lkIjp7ImFjY291bnRfaWQiOjEsInByb2plY3RfaWQiOjV9fSwiZW1haWxfdmVyaWZpZWQiOmZhbHNlLCJuYW1lIjoidGcgdGciLCJwcm9qZWN0IjoibW9iaWxlIiwicHJlZmVycmVkX3VzZXJuYW1lIjoidGciLCJ0ZWFtIjoiZGV2IiwiZ2l2ZW5fbmFtZSI6InRnIiwiZmFtaWx5X25hbWUiOiJ0ZyJ9.Nx9An-sqto8ClmNah8Mi6y16mjB6jk-I1kxQdtP0j0c",
			},
		},
	}
	resultExpected = &Token{
		body: &body{
			Exp:   1610889266,
			Iat:   1610888966,
			Jti:   "09a058a2-0752-4ecd-a4e9-b65e85af423f",
			Scope: "openid email profile",
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 5,
					AccountID: 1,
				},
				Labels: map[string]string{
					"project": "dev",
					"team":    "mobile",
				},
				ExtraFilters: []string{`{env=~"prod|dev"}`, `{team!="test"}`},
			},
		},
		header: &header{
			Alg: "HS256",
			Kid: "aAZoCGvuGbFoftWHxQZyRSQen3yX4U0GPlP5oZOQSwc",
			Typ: "JWT",
		},
	}
	f(r, resultExpected, true)

	// parse ok without prefix
	r = &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImFBWm9DR3Z1R2JGb2Z0V0h4UVp5UlNRZW4zeVg0VTBHUGxQNW9aT1FTd2MifQ.eyJleHAiOjE2MTA5NzYxODksImlhdCI6MTYxMDk3NTg4OSwiYXV0aF90aW1lIjoxNjEwOTc1ODg5LCJqdGkiOiI5YjE5NDE4Ny02YmI3LTQyNDQtOWQxYi01NTllYWIyZWY3ZjMiLCJpc3MiOiJodHRwczovL2xvY2FsaG9zdDo4NDQzL2F1dGgvcmVhbG1zL3Rlc3QiLCJhdWQiOiJhY2NvdW50Iiwic3ViIjoiNDYwODU5NDEtYjkyYi00NzFhLWIwNWEtOTU5OWNhMjlkYTFlIiwidHlwIjoiQmVhcmVyIiwiYXpwIjoiZ3JhZmFuYSIsInNlc3Npb25fc3RhdGUiOiIxMzc3ZDEwMi03NTJiLTQ0ODYtOTlkYS1jMjA4MjRiODJkMzEiLCJhY3IiOiIxIiwiYWxsb3dlZC1vcmlnaW5zIjpbImh0dHA6Ly9sb2NhbGhvc3Q6MzAwMCJdLCJyZWFsbV9hY2Nlc3MiOnsicm9sZXMiOlsib2ZmbGluZV9hY2Nlc3MiLCJ1bWFfYXV0aG9yaXphdGlvbiJdfSwicmVzb3VyY2VfYWNjZXNzIjp7ImFjY291bnQiOnsicm9sZXMiOlsibWFuYWdlLWFjY291bnQiLCJtYW5hZ2UtYWNjb3VudC1saW5rcyIsInZpZXctcHJvZmlsZSJdfX0sInNjb3BlIjoib3BlbmlkIGVtYWlsIHByb2ZpbGUiLCJ2bV9hY2Nlc3MiOnsiZXh0cmFfbGFiZWxzIjp7InByb2plY3QiOiJkZXYiLCJ0ZWFtIjoibW9iaWxlIn0sImV4dHJhX2ZpbHRlcnMiOlsie2Vudj1+XCJwcm9kfGRldlwifSIsInt0ZWFtIT1cInRlc3RcIn0iXSwidGVuYW50X2lkIjp7ImFjY291bnRfaWQiOjEsInByb2plY3RfaWQiOjV9fSwiZW1haWxfdmVyaWZpZWQiOmZhbHNlLCJuYW1lIjoidGcgdGciLCJwcm9qZWN0IjoibW9iaWxlIiwicHJlZmVycmVkX3VzZXJuYW1lIjoidGciLCJ0ZWFtIjoiZGV2IiwiZ2l2ZW5fbmFtZSI6InRnIiwiZmFtaWx5X25hbWUiOiJ0ZyJ9.Nx9An-sqto8ClmNah8Mi6y16mjB6jk-I1kxQdtP0j0c",
			},
		},
	}
	resultExpected = &Token{
		body: &body{
			Exp:   1610889266,
			Iat:   1610888966,
			Jti:   "09a058a2-0752-4ecd-a4e9-b65e85af423f",
			Scope: "openid email profile",
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 5,
					AccountID: 1,
				},
				Labels: map[string]string{
					"project": "dev",
					"team":    "mobile",
				},
				ExtraFilters: []string{`{env=~"prod|dev"}`, `{team!="test"}`},
			},
		},
		header: &header{
			Alg: "HS256",
			Kid: "aAZoCGvuGbFoftWHxQZyRSQen3yX4U0GPlP5oZOQSwc",
			Typ: "JWT",
		},
	}
	f(r, resultExpected, false)

	// parse ok with string vm_access
	r = &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsImtpZCI6Ikg5bmo1QU9Tc3dNcGhnMVNGeDdqYVYtbEI5dyJ9.eyJhdWQiOiI3YTczMTFlNy1iYTdlLTQ5NWUtOTk1ZS1hZjUzNGU3M2MxMTAiLCJpc3MiOiJodHRwczovL2xvZ2luLm1pY3Jvc29mdG9ubGluZS5jb20vMjVkYTFlY2UtNjY5MS00ODY4LWE3N2ItMWIwZjliYmU1ZjQzL3YyLjAiLCJpYXQiOjE3MjU2MjUzMzIsIm5iZiI6MTcyNTYyNTMzMiwiZXhwIjoxNzI1NjI5MjMyLCJuYW1lIjoiWmFraGFyIEJlc3NhcmFiIiwib2lkIjoiOGI5ZWY2YjMtMWMwMS00YjczLTg0ODItMjRkNmI2NTE1Y2U0IiwicHJlZmVycmVkX3VzZXJuYW1lIjoiei5iZXNzYXJhYkB2aWN0b3JpYW1ldHJpY3MuY29tIiwicmgiOiIwLkFXTUJ6aDdhSlpGbWFFaW5leHNQbTc1ZlEtY1JjM3AtdWw1Sm1WNnZVMDV6d1JCakFaby4iLCJzdWIiOiJXRld3QTlYZjZpZXUxLUgwNDBuU0QxRVo3UWxOLTVHbWxob2p4czdMUFJRIiwidGlkIjoiMjVkYTFlY2UtNjY5MS00ODY4LWE3N2ItMWIwZjliYmU1ZjQzIiwidXRpIjoidlo1MjQySmhNVWFUUktaYVFCRjhBQSIsInZlciI6IjIuMCIsInZtX2FjY2VzcyI6IntcInRlbmFudF9pZFwiOntcInByb2plY3RfaWRcIjogNSwgXCJhY2NvdW50X2lkXCI6IDF9fSJ9.E0pEjbazG1QP5nT7fk3GZ9QjIchxOegBQGWnRN8-xFVSJ61v9-FZ-0fNHCYuMVpWvCLqlAHscITB1EYOt4ezvVdwNhO-TXTFAXGznXD4WRsK_G5KGk1QuV-kYwhvidZsPGQe39KlAJm5BPx1fnoHr4yakD647aspd4p9SAsM_H0l4agVZeAhfBqKHI0-cnLcbGb7mC-pZUB1fJBvwc9OT2gzjmA-2T2Vmv4C33I70oDt-wTYmMyHQ4uItTVkj6JXo6gc4V1APJvtA6fB8iq75J-NZ51MiptVIoocX3fYHuC-FwHpi9AFH-1o06tHN0N_A4Hjf8cyzsG8GBaLLGQblw",
			},
		},
	}

	resultExpected = &Token{
		body: &body{
			Exp: 1725629232,
			Iat: 1725625332,
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 5,
					AccountID: 1,
				},
			},
		},
		header: &header{
			Alg: "RS256",
			Kid: "H9nj5AOSswMphg1SFx7jaV-lB9w",
			Typ: "JWT",
		},
	}
	f(r, resultExpected, false)

	// parse ok with scope being slice of strings
	r = &http.Request{
		Header: map[string][]string{
			"Authorization": {
				"Bearer ewogICJ0eXAiOiJKV1QiLAogICJhbGciOiJSUzI1NiIsCiAgImtpZCI6Ikg5bmo1QU9Tc3dNcGhnMVNGeDdqYVYtbEI5dyIKfQ.ewogICJhdWQiOiI3YTczMTFlNy1iYTdlLTQ5NWUtOTk1ZS1hZjUzNGU3M2MxMTAiLAogICJpc3MiOiJodHRwczovL2xvZ2luLm1pY3Jvc29mdG9ubGluZS5jb20vMjVkYTFlY2UtNjY5MS00ODY4LWE3N2ItMWIwZjliYmU1ZjQzL3YyLjAiLAogICJpYXQiOjE3MjU2MjUzMzIsCiAgIm5iZiI6MTcyNTYyNTMzMiwKICAiZXhwIjoxNzI1NjI5MjMyLAogICJuYW1lIjoiWmFraGFyIEJlc3NhcmFiIiwKICAib2lkIjoiOGI5ZWY2YjMtMWMwMS00YjczLTg0ODItMjRkNmI2NTE1Y2U0IiwKICAicHJlZmVycmVkX3VzZXJuYW1lIjoiei5iZXNzYXJhYkB2aWN0b3JpYW1ldHJpY3MuY29tIiwKICAicmgiOiIwLkFXTUJ6aDdhSlpGbWFFaW5leHNQbTc1ZlEtY1JjM3AtdWw1Sm1WNnZVMDV6d1JCakFaby4iLAogICJzdWIiOiJXRld3QTlYZjZpZXUxLUgwNDBuU0QxRVo3UWxOLTVHbWxob2p4czdMUFJRIiwKICAidGlkIjoiMjVkYTFlY2UtNjY5MS00ODY4LWE3N2ItMWIwZjliYmU1ZjQzIiwKICAidXRpIjoidlo1MjQySmhNVWFUUktaYVFCRjhBQSIsCiAgInZlciI6IjIuMCIsCiAgInZtX2FjY2VzcyI6IntcInRlbmFudF9pZFwiOntcInByb2plY3RfaWRcIjogNSwgXCJhY2NvdW50X2lkXCI6IDF9fSIsCiAgInNjb3BlIjogWyJvcGVuaWQiLCAidm0iXQp9.ZXdvZ0lDSjBlWEFpT2lKS1YxUWlMQW9nSUNKaGJHY2lPaUpTVXpJMU5pSXNDaUFnSW10cFpDSTZJa2c1Ym1vMVFVOVRjM2ROY0dobk1WTkdlRGRxWVZZdGJFSTVkeUlLZlEuLktrUG9qNWJoaDNWcnRyY3RVb0lHaE5vN2hNc2VGT3hESGVEQ2g3MFViV2l2LU5pb1Zia2duZk1CMkhacHN6WGU5WmNmX2FIaURJSVNTYkNTaDlvQnF1aS02OEJDcmplNFJWRkpGZFV6R3V1SmdOTS11YVpBcFJqSFNNZDUxb2RvbHFoUGFHS09URnJXVmlIWlpfVDdXaVNUcV84U3Y1a2x1Y2xMb0hEcU82MU5Na2w0TmRCVnQxM1hjRTBfM243U3VxTDdpaks2dGMwZ2NzcmJ5c3JNdl9jd2VRamZsLU5fV0N0SG40NnhadEhvX0RpZERabzc2TjV1NE52Uk1OZUxNcXZ0YTgzUzhPdzNyUUlhaUFjUUNHYjBqUU5hV2VEQlFzZUZ6SjRyR0h6RjAwZDlqVkNCSHVWRmI5eHNnSnJVUDZ0S05iT2hTeEY1RzBocElVYk5OUQ",
			},
		},
	}

	resultExpected = &Token{
		body: &body{
			Exp: 1725629232,
			Iat: 1725625332,
			VMAccess: &VMAccessClaim{
				Tenant: TenantID{
					ProjectID: 5,
					AccountID: 1,
				},
			},
		},
		header: &header{
			Alg: "RS256",
			Kid: "H9nj5AOSswMphg1SFx7jaV-lB9w",
			Typ: "JWT",
		},
	}
	f(r, resultExpected, false)
}
