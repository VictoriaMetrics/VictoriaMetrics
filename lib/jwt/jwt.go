package jwt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

const (
	prefix = "Bearer "
)

const (
	read = 1 << iota
	write
)

var (
	// ErrHeaderMissing missing header.
	ErrHeaderMissing = fmt.Errorf("jwt authorization header is missing")
	// ErrVMAccessFieldMissing missing vm_access field.
	ErrVMAccessFieldMissing = fmt.Errorf("missing `vm_access` claim")
	// ErrBadTokenFormat incorrect format for token
	ErrBadTokenFormat = fmt.Errorf("bad token format, must be jwt")
)

// Token represents jwt token
// https://auth0.com/docs/tokens/json-web-tokens
type Token struct {
	header             *header
	body               *body
	payload, signature []byte
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

type body struct {
	// expired at time unix_ts
	Exp int64 `json:"exp"`
	// issued at time unix_ts
	Iat      int64          `json:"iat"`
	Jti      string         `json:"jti,omitempty"`
	Scope    string         `json:"scope,omitempty"`
	VMAccess *VMAccessClaim `json:"vm_access"`
}

// Labels defines labels added to filters or incoming time series.
type Labels map[string]string

// AsExtraLabels - converts labels to label=value pairs.
func (l Labels) AsExtraLabels() []string {
	if len(l) == 0 {
		return nil
	}
	res := make([]string, 0, len(l))
	for k, v := range l {
		res = append(res, k+"="+v)
	}
	// sort for consistent uri.
	slices.Sort(res)
	return res
}

type VMAccessClaim struct {
	Tenant TenantID `json:"tenant_id"`
	Labels Labels   `json:"extra_labels,omitempty"`
	// promql filters applied to each select query
	ExtraFilters []string `json:"extra_filters,omitempty"`
	// role can be denied as 1 = read, 2 = write, 3 = read and write
	// 0 = unconfigured - read and write
	Mode int `json:"mode,omitempty"`

	MetricsAccountID    int64    `json:"metrics_account_id,omitempty"`
	MetricsProjectID    int64    `json:"metrics_project_id,omitempty"`
	MetricsExtraFilters []string `json:"metrics_extra_filters,omitempty"`
	MetricsExtraLabels  []string `json:"metrics_extra_labels,omitempty"`

	LogsAccountID          int64    `json:"logs_account_id,omitempty"`
	LogsProjectID          int64    `json:"logs_project_id,omitempty"`
	LogsExtraFilters       []string `json:"logs_extra_filters,omitempty"`
	LogsExtraStreamFilters []string `json:"logs_extra_stream_filters,omitempty"`
}

// TenantID represents tenantID.
type TenantID struct {
	ProjectID int32 `json:"project_id"`
	AccountID int32 `json:"account_id"`
}

// String implements interface.
func (tid TenantID) String() string {
	return fmt.Sprintf("%d:%d", tid.AccountID, tid.ProjectID)
}

// NewToken creates token from raw string.
func NewToken(auth string, enforceAuthPrefix bool) (*Token, error) {
	if enforceAuthPrefix && (len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix)) {
		return nil, fmt.Errorf("wrong format, prefix: %s is missing", prefix)
	}

	// While https://datatracker.ietf.org/doc/html/rfc6750#section-2.1 states that only Bearer prefix is allowed,
	// it claims to be conformant to the generic syntax defined in https://datatracker.ietf.org/doc/html/rfc2617#section-1.2
	// which permits case-insensitive auth scheme.
	// So we should be tolerant to different cases of "Bearer" prefix.
	if len(auth) >= len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		auth = auth[len(prefix):]
	}

	jwt := strings.SplitN(auth, ".", 3)
	if len(jwt) != 3 {
		return nil, ErrBadTokenFormat
	}
	var t Token
	return t.parse(jwt[0], jwt[1], jwt[2])
}

// NewTokenFromRequestWithCustomHeader return new jwt token from request by provided header
func NewTokenFromRequestWithCustomHeader(r *http.Request, headerName string, enforceAuthPrefix bool) (*Token, error) {
	auth := r.Header.Get(headerName)
	if len(auth) == 0 {
		return nil, ErrHeaderMissing
	}
	return NewToken(auth, enforceAuthPrefix)
}

func (t *Token) parse(header, body, signature string) (*Token, error) {
	b, err := parseJWTBody(body)
	if err != nil {
		return nil, err
	}
	if b.VMAccess == nil {
		return nil, ErrVMAccessFieldMissing
	}
	t.body = b
	h, err := parseJWTHeader(header)
	if err != nil {
		return nil, err
	}
	t.header = h

	t.payload = []byte(header + "." + body)
	t.signature, err = decodeB64([]byte(signature))
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature as b64: %w", err)
	}

	return t, nil
}

// IsExpired checks if jwt token is expired.
func (t *Token) IsExpired(currentTime time.Time) bool {
	return currentTime.Unix() > t.body.Exp
}

// CanWrite checks if token has write permissions.
func (t *Token) CanWrite() bool {
	// unconfigured
	if t.body.VMAccess.Mode == 0 {
		return true
	}
	if write&t.body.VMAccess.Mode > 0 {
		return true
	}
	return false
}

// CanRead check if token has read permissions.
func (t *Token) CanRead() bool {
	// unconfigured
	if t.body.VMAccess.Mode == 0 {
		return true
	}
	if read&t.body.VMAccess.Mode > 0 {
		return true
	}
	return false
}

// AccessLabels returns vm_access labels for given JWT token,
// in key=value format.
func (t *Token) AccessLabels() []string {
	return t.body.VMAccess.Labels.AsExtraLabels()
}

// Tenant returns tenantID for token.
func (t *Token) Tenant() TenantID {
	return t.body.VMAccess.Tenant
}

// ExtraFilters metricsql filters for select queries
func (t *Token) ExtraFilters() []string {
	return t.body.VMAccess.ExtraFilters
}

func (t *Token) VMAccess() *VMAccessClaim {
	return t.body.VMAccess
}

func parseJWTHeader(data string) (*header, error) {
	var jh header
	decoded, err := decodeB64([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("cannot decode jwt header as b64: %w", err)
	}
	if err := json.Unmarshal(decoded, &jh); err != nil {
		return nil, fmt.Errorf("cannot parse jwt header: %w", err)
	}
	return &jh, nil
}

func parseJWTBody(data string) (*body, error) {
	type tbody struct {
		// expired at time unix_ts
		Exp int64 `json:"exp"`
		// issued at time unix_ts
		Iat   int64           `json:"iat"`
		Jti   string          `json:"jti,omitempty"`
		Scope json.RawMessage `json:"scope,omitempty"`
		// store as raw message to support different types
		VMAccess *json.RawMessage `json:"vm_access"`
	}
	var tb tbody

	decoded, err := decodeB64([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("cannot decode jwt body as b64: %w", err)
	}
	if err := json.Unmarshal(decoded, &tb); err != nil {
		return nil, fmt.Errorf("cannot parse jwt body: %w", err)
	}

	if tb.VMAccess == nil {
		return nil, ErrVMAccessFieldMissing
	}

	// some IDPs encode custom claims as a string
	// try parsing as an object and fallback to a string
	var a VMAccessClaim
	if err := json.Unmarshal(*tb.VMAccess, &a); err != nil {
		var s string
		if err := json.Unmarshal(*tb.VMAccess, &s); err != nil {
			return nil, fmt.Errorf("cannot parse jwt body vm_access: %w", err)
		}

		if err := json.Unmarshal([]byte(s), &a); err != nil {
			return nil, fmt.Errorf("cannot parse jwt body vm_access: %w", err)
		}
	}

	// some IDPs encode scope as a string and some as an array
	var scope string
	if tb.Scope != nil {
		if err := json.Unmarshal(tb.Scope, &scope); err != nil {
			var scopeSlice []string
			if err := json.Unmarshal(tb.Scope, &scopeSlice); err != nil {
				return nil, fmt.Errorf("cannot parse jwt body scope: %w", err)
			}
			scope = strings.Join(scopeSlice, " ")
		}
	}

	parsedBody := &body{
		Exp:      tb.Exp,
		Iat:      tb.Iat,
		Jti:      tb.Jti,
		Scope:    scope,
		VMAccess: &a,
	}
	return parsedBody, nil
}

func decodeB64(data []byte) ([]byte, error) {
	idx := bytes.IndexAny(data, "+/")
	// slow path, std base64 encoding convert it to url encoding
	if idx >= 0 {
		for idx, c := range data {
			switch c {
			case '+':
				data[idx] = '-'
			case '/':
				data[idx] = '_'
			}
		}

	}
	dst := make([]byte, base64.RawURLEncoding.DecodedLen(len(data)))
	_, err := base64.RawURLEncoding.Decode(dst, data)
	if err != nil {
		return nil, fmt.Errorf("cannot decode jwt body as b64: %w", err)
	}
	return dst, nil
}
