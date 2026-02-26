package jwt

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/valyala/fastjson"
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
	header             header
	body               body
	payload, signature []byte
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`

	buf []byte
	p   *fastjson.Parser
}

func (h *header) parse(src string) error {
	var err error
	h.buf, err = decodeB64(h.buf[:0], src)
	if err != nil {
		return err
	}

	h.p = parserPool.Get()
	jv, err := h.p.ParseBytes(h.buf)
	if err != nil {
		return err
	}
	if jv == nil {
		return fmt.Errorf("unexpected empty json")
	}
	if jv.Type() != fastjson.TypeObject {
		return fmt.Errorf("unexpected non json object {} type: %q", jv.Type())
	}
	h.Alg, err = stringFromJSONValue(jv, "alg")
	if err != nil {
		return err
	}
	h.Typ, err = stringFromJSONValue(jv, "typ")
	if err != nil {
		return err
	}
	h.Kid, err = stringFromJSONValue(jv, "kid")
	if err != nil {
		return err
	}

	return nil
}

func (h *header) reset() {
	h.Alg = ""
	h.Typ = ""
	h.Kid = ""

	h.buf = h.buf[:0]
	if h.p != nil {
		parserPool.Put(h.p)
		h.p = nil
	}
}

type body struct {
	// expired at time unix_ts
	Exp int64 `json:"exp"`
	// issued at time unix_ts
	Iat           int64  `json:"iat"`
	Jti           string `json:"jti,omitempty"`
	Scope         string `json:"scope,omitempty"`
	vmAccessClaim VMAccessClaim

	buf []byte
	p   *fastjson.Parser

	// allClaims holds entire json body
	// for the HasClaims() method
	allClaims *fastjson.Value

	// claimsParser holds optional parser for `vm_access` string representation
	claimsParser *fastjson.Parser
}

func (b *body) parse(src string) error {

	var err error
	b.buf, err = decodeB64(b.buf[:0], src)
	if err != nil {
		return err
	}
	b.p = parserPool.Get()
	jv, err := b.p.ParseBytes(b.buf)
	if err != nil {
		return err
	}
	if expObject := jv.Get("exp"); expObject != nil {
		b.Exp, err = expObject.Int64()
		if err != nil {
			return fmt.Errorf("cannot parse `exp` field: %w", err)
		}
	}
	if iatObject := jv.Get("iat"); iatObject != nil {
		b.Iat, err = iatObject.Int64()
		if err != nil {
			return fmt.Errorf("cannot parse `iat` field: %w", err)
		}
	}
	vaObject := jv.Get("vm_access")
	if vaObject == nil {
		return ErrVMAccessFieldMissing
	}
	// some IDPs encode custom claims as a string
	// try parsing as an object and fallback to a string
	switch vaObject.Type() {
	case fastjson.TypeObject:
		if err := b.vmAccessClaim.parseFrom(vaObject); err != nil {
			return err
		}
	case fastjson.TypeString:
		b.claimsParser = parserPool.Get()
		va, err := b.claimsParser.ParseBytes(vaObject.GetStringBytes())
		if err != nil {
			return fmt.Errorf("cannot parse `vm_access` string json: %w", err)
		}
		if err := b.vmAccessClaim.parseFrom(va); err != nil {
			return fmt.Errorf("cannot parse `vm_access` values from string json: %w", err)
		}
	case fastjson.TypeNull:
		return ErrVMAccessFieldMissing
	default:
		return fmt.Errorf("unexpected type for `vm_access` field; got: %q, want object {}", vaObject.Type())
	}
	b.Jti = bytesutil.ToUnsafeString(jv.GetStringBytes("jti"))

	if scopeObject := jv.Get("scope"); scopeObject != nil {
		// some IDPs encode scope as a string and some as an array
		switch scopeObject.Type() {
		case fastjson.TypeString:
			sb := scopeObject.GetStringBytes()
			b.Scope = bytesutil.ToUnsafeString(sb)
		case fastjson.TypeArray:
			var sizeNeeded int
			ss := scopeObject.GetArray()
			for _, v := range ss {
				sizeNeeded += len(v.GetStringBytes()) + 1
			}
			dst := make([]byte, 0, sizeNeeded)
			for idx, v := range ss {
				dst = append(dst, v.GetStringBytes()...)
				if idx < len(ss)-1 {
					dst = append(dst, ',')
				}
			}
			b.Scope = bytesutil.ToUnsafeString(dst)
		default:
			return fmt.Errorf("unexpected type for `scope` field; got %q, want String or []String", scopeObject.Type())
		}
	}
	b.allClaims = jv

	return nil
}

func (b *body) reset() {
	b.Exp = 0
	b.Iat = 0
	b.Jti = ""
	b.Scope = ""
	b.buf = b.buf[:0]
	b.allClaims = nil
	b.vmAccessClaim.reset()
	if b.p != nil {
		parserPool.Put(b.p)
		b.p = nil
	}
	if b.claimsParser != nil {
		parserPool.Put(b.claimsParser)
		b.claimsParser = nil
	}

}

// Parse parses JWT token from given source string
//
// Token field is valid until src is reachable
func (t *Token) Parse(src string, enforceAuthPrefix bool) error {
	if enforceAuthPrefix && (len(src) < len(prefix) || !strings.EqualFold(src[:len(prefix)], prefix)) {
		return fmt.Errorf("wrong format, prefix: %s is missing", prefix)
	}
	// While https://datatracker.ietf.org/doc/html/rfc6750#section-2.1 states that only Bearer prefix is allowed,
	// it claims to be conformant to the generic syntax defined in https://datatracker.ietf.org/doc/html/rfc2617#section-1.2
	// which permits case-insensitive auth scheme.
	// So we should be tolerant to different cases of "Bearer" prefix.
	if len(src) >= len(prefix) && strings.EqualFold(src[:len(prefix)], prefix) {
		src = src[len(prefix):]
	}

	// assume jwt token has the following structure:
	// header.body.signature
	var header, body, signature string
	idx := strings.IndexByte(src, '.')
	if idx <= 0 {
		return ErrBadTokenFormat
	}
	header = src[:idx]
	src = src[idx+1:]
	idx = strings.IndexByte(src, '.')
	if idx <= 0 {
		return ErrBadTokenFormat
	}
	body = src[:idx]
	signature = src[idx+1:]
	if err := t.parse(header, body, signature); err != nil {
		return err
	}
	return nil
}

// HasClaims checks if Token has all given claim key value pairs
func (t *Token) HasClaims(claims map[string]string) bool {
	for k, v := range claims {
		gotV := t.body.allClaims.Get(k)
		if gotV == nil || gotV.Type() != fastjson.TypeString {
			return false
		}
		tcv := bytesutil.ToUnsafeString(gotV.GetStringBytes())
		if tcv != v {
			return false
		}
	}

	return true
}

// VMAccess return a reference to the VMAccessClaim
// all data are valid until Token is reachable
func (t *Token) VMAccess() *VMAccessClaim {
	return &t.body.vmAccessClaim
}

// Reset release memory used by token
// Token cannot be used after this call
func (t *Token) Reset() {
	t.header.reset()
	t.body.reset()
	t.payload = t.payload[:0]
	t.signature = t.signature[:0]
}

// VMAccessClaim represent JWT claim object
type VMAccessClaim struct {
	// promql filters applied to each select query
	ExtraFilters []string `json:"extra_filters,omitempty"`

	MetricsExtraFilters    []string `json:"metrics_extra_filters,omitempty"`
	MetricsExtraLabels     []string `json:"metrics_extra_labels,omitempty"`
	LogsExtraFilters       []string `json:"logs_extra_filters,omitempty"`
	LogsExtraStreamFilters []string `json:"logs_extra_stream_filters,omitempty"`

	Labels []string `json:"extra_labels,omitempty"`
	// labelsBuf holds allocated memory for Labels
	labelsBuf []byte
	Tenant    TenantID `json:"tenant_id"`
	// role can be denied as 1 = read, 2 = write, 3 = read and write
	// 0 = unconfigured - read and write
	Mode int `json:"mode,omitempty"`

	MetricsAccountID uint32 `json:"metrics_account_id,omitempty"`
	MetricsProjectID uint32 `json:"metrics_project_id,omitempty"`

	LogsAccountID uint32 `json:"logs_account_id,omitempty"`
	LogsProjectID uint32 `json:"logs_project_id,omitempty"`
}

func (vac *VMAccessClaim) reset() {
	vac.Tenant.AccountID = 0
	vac.Tenant.ProjectID = 0
	clear(vac.Labels)
	vac.Labels = vac.Labels[:0]
	vac.labelsBuf = vac.labelsBuf[:0]
	clear(vac.ExtraFilters)
	vac.ExtraFilters = vac.ExtraFilters[:0]
	vac.Mode = 0

	vac.MetricsAccountID = 0
	vac.MetricsProjectID = 0
	clear(vac.MetricsExtraFilters)
	vac.MetricsExtraFilters = vac.MetricsExtraFilters[:0]
	clear(vac.MetricsExtraLabels)
	vac.MetricsExtraLabels = vac.MetricsExtraLabels[:0]
	vac.LogsAccountID = 0
	vac.LogsProjectID = 0
	clear(vac.LogsExtraFilters)
	vac.LogsExtraFilters = vac.LogsExtraFilters[:0]
	clear(vac.LogsExtraStreamFilters)
	vac.LogsExtraStreamFilters = vac.LogsExtraStreamFilters[:0]
}

func (vac *VMAccessClaim) parseFrom(jv *fastjson.Value) error {

	if err := vac.Tenant.parseFrom(jv); err != nil {
		return err
	}

	var err error
	vac.ExtraFilters, err = stringSliceFromJSONValue(vac.ExtraFilters, jv, "extra_filters")
	if err != nil {
		return err
	}
	efs := jv.Get("extra_labels")
	if efs != nil {
		efsO, err := efs.Object()
		if err != nil {
			return fmt.Errorf("cannot parse `extra_labels` field: %w", err)
		}
		buf := vac.labelsBuf[:0]
		var visitErr error
		efsO.Visit(func(key []byte, v *fastjson.Value) {
			if visitErr != nil {
				return
			}
			vs, err := v.StringBytes()
			if err != nil {
				visitErr = fmt.Errorf("unexpected value for key=%q: %w", string(key), err)
			}
			start := len(buf)
			sizeNeeded := len(key) + 1 + len(vs)
			if len(buf)+sizeNeeded >= cap(buf) {
				// allocate new slice without memory fragmentation
				// old slice will be referenced by vac.Labels
				start = 0
				buf = make([]byte, 0, len(buf)+sizeNeeded)
			}
			buf = append(buf, key...)
			buf = append(buf, '=')
			buf = append(buf, vs...)
			ef := bytesutil.ToUnsafeString(buf[start:])
			vac.Labels = append(vac.Labels, ef)
		})
		vac.labelsBuf = buf
		if visitErr != nil {
			return fmt.Errorf("cannot parse `extra_labels` field: %w", visitErr)
		}
	}
	mode := jv.Get("mode")
	if mode != nil {
		vac.Mode, err = mode.Int()
		if err != nil {
			return fmt.Errorf("unexpected `mode` value: %w", err)
		}
	}
	vac.MetricsAccountID, err = uint32FromJSONValue(jv, "metrics_account_id")
	if err != nil {
		return err
	}
	vac.MetricsProjectID, err = uint32FromJSONValue(jv, "metrics_project_id")
	if err != nil {
		return err
	}

	vac.MetricsExtraFilters, err = stringSliceFromJSONValue(vac.MetricsExtraFilters, jv, "metrics_extra_filters")
	if err != nil {
		return err
	}
	vac.MetricsExtraLabels, err = stringSliceFromJSONValue(vac.MetricsExtraLabels, jv, "metrics_extra_labels")
	if err != nil {
		return err
	}
	vac.LogsAccountID, err = uint32FromJSONValue(jv, "logs_account_id")
	if err != nil {
		return err
	}
	vac.LogsProjectID, err = uint32FromJSONValue(jv, "logs_project_id")
	if err != nil {
		return err
	}
	vac.LogsExtraFilters, err = stringSliceFromJSONValue(vac.LogsExtraFilters, jv, "logs_extra_filters")
	if err != nil {
		return err
	}
	vac.LogsExtraStreamFilters, err = stringSliceFromJSONValue(vac.LogsExtraStreamFilters, jv, "logs_extra_stream_filters")
	if err != nil {
		return err
	}

	return nil
}

// TenantID represents tenantID.
type TenantID struct {
	ProjectID int32 `json:"project_id"`
	AccountID int32 `json:"account_id"`
}

func (tid *TenantID) parseFrom(jv *fastjson.Value) error {
	tidObject := jv.Get("tenant_id")
	if tidObject == nil {
		return nil
	}
	var err error
	tid.AccountID, err = int32FromJSONValue(tidObject, "account_id")
	if err != nil {
		return err
	}
	tid.ProjectID, err = int32FromJSONValue(tidObject, "project_id")
	if err != nil {
		return err
	}

	return nil
}

// String implements interface.
func (tid TenantID) String() string {
	return fmt.Sprintf("%d:%d", tid.AccountID, tid.ProjectID)
}

// NewToken creates token from raw string.
//
// Deprecated: allocates a new Token on every call.
// Prefer acquiring a Token from a sync.Pool, calling t.Parse(), and returning it after use.
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
	if err := t.parse(jwt[0], jwt[1], jwt[2]); err != nil {
		return nil, err
	}
	return &t, nil
}

// NewTokenFromRequestWithCustomHeader return new jwt token from request by provided header
//
// Deprecated: allocates a new Token on every call.
// Prefer acquiring a Token from a sync.Pool, calling t.Parse(), and returning it after use.
func NewTokenFromRequestWithCustomHeader(r *http.Request, headerName string, enforceAuthPrefix bool) (*Token, error) {
	auth := r.Header.Get(headerName)
	if len(auth) == 0 {
		return nil, ErrHeaderMissing
	}
	return NewToken(auth, enforceAuthPrefix)
}

func (t *Token) parse(header, body, signature string) error {
	if err := t.body.parse(body); err != nil {
		return fmt.Errorf("cannot parse token body: %w", err)
	}
	if err := t.header.parse(header); err != nil {
		return fmt.Errorf("cannot parse token header: %w", err)
	}

	t.payload = bytesutil.ResizeNoCopyNoOverallocate(t.payload, len(header)+len(body)+1)
	t.payload = append(t.payload[:0], header...)
	t.payload = append(t.payload, '.')
	t.payload = append(t.payload, body...)
	var err error
	t.signature, err = decodeB64(t.signature[:0], signature)
	if err != nil {
		return fmt.Errorf("cannot decode token signature: %w", err)
	}

	return nil
}

// IsExpired checks if jwt token is expired.
func (t *Token) IsExpired(currentTime time.Time) bool {
	return currentTime.Unix() > t.body.Exp
}

// CanWrite checks if token has write permissions.
func (t *Token) CanWrite() bool {
	// unconfigured
	if t.body.vmAccessClaim.Mode == 0 {
		return true
	}
	if write&t.body.vmAccessClaim.Mode > 0 {
		return true
	}
	return false
}

// CanRead check if token has read permissions.
func (t *Token) CanRead() bool {
	// unconfigured
	if t.body.vmAccessClaim.Mode == 0 {
		return true
	}
	if read&t.body.vmAccessClaim.Mode > 0 {
		return true
	}
	return false
}

// AccessLabels returns vm_access labels for given JWT token,
// in key=value format.
//
// Returned value is only valid until Token is reachable
func (t *Token) AccessLabels() []string {
	return t.body.vmAccessClaim.Labels
}

// Tenant returns tenantID for token.
func (t *Token) Tenant() TenantID {
	return t.body.vmAccessClaim.Tenant
}

// ExtraFilters metricsql filters for select queries
//
// Returned value is only valid until Token is reachable
func (t *Token) ExtraFilters() []string {
	return t.body.vmAccessClaim.ExtraFilters
}

func decodeB64(dst []byte, src string) ([]byte, error) {
	data := bytesutil.ToUnsafeBytes(src)
	idx := bytes.IndexAny(data, "+/")
	// slow path, std base64 encoding convert it to url encoding
	// it could be encoded with standard Base64 (+/) instead of Base64URL (-_).
	if idx >= 0 {
		// make a copy of provided input, src cannot be modified by parser
		bb := decodeb64BufferPool.Get()
		defer decodeb64BufferPool.Put(bb)
		b := bb.B[:0]
		b = append(b, data...)
		data = b
		for idx, c := range data {
			switch c {
			case '+':
				data[idx] = '-'
			case '/':
				data[idx] = '_'
			}
		}
	}
	dst = bytesutil.ResizeNoCopyNoOverallocate(dst, base64.RawURLEncoding.DecodedLen(len(data)))
	_, err := base64.RawURLEncoding.Decode(dst, data)
	if err != nil {
		return nil, err
	}
	return dst, nil
}

// stringFromJSONValue is a helper with missing String parse method from fastjson package
//
// If key is required, perform check with Exists() call
func stringFromJSONValue(jv *fastjson.Value, key string) (string, error) {
	jvInner := jv.Get(key)
	if jvInner == nil {
		return "", nil
	}
	b, err := jvInner.StringBytes()
	if err != nil {
		return "", fmt.Errorf("unexpected non-string value for key=%q: %w", key, err)
	}

	return bytesutil.ToUnsafeString(b), nil
}

// uint32FromJSONValue is a helper for missing Uint32 parse method from fastjson package
//
// If key is required, perform check with Exists() call
func uint32FromJSONValue(jv *fastjson.Value, key string) (uint32, error) {
	jvInner := jv.Get(key)
	if jvInner == nil {
		return 0, nil
	}
	u64, err := jvInner.Uint64()
	if err != nil {
		return 0, fmt.Errorf("unexpected non-uint32 value for key=%q: %w", key, err)
	}
	if u64 > math.MaxUint32 {
		return 0, fmt.Errorf("value cannot exceed uint32 for key=%q", key)
	}

	return uint32(u64), nil
}

// int32FromJSONValue is a helper for missing Int32 parse method from fastjson package
//
// If key is required, perform check with Exists() call
func int32FromJSONValue(jv *fastjson.Value, key string) (int32, error) {
	jvInner := jv.Get(key)
	if jvInner == nil {
		return 0, nil
	}
	i64, err := jvInner.Int64()
	if err != nil {
		return 0, fmt.Errorf("unexpected non-int32 value for key=%q: %w", key, err)
	}
	if i64 > math.MaxInt32 || i64 < math.MinInt32 {
		return 0, fmt.Errorf("value cannot exceed int32 for key=%q", key)
	}

	return int32(i64), nil
}

// stringSliceFromJSONValue is a helper for missing StringArray parse method from fastjson package
//
// If key is required, perform check with Exists() call
func stringSliceFromJSONValue(dst []string, jv *fastjson.Value, key string) ([]string, error) {
	jvInner := jv.Get(key)
	if jvInner == nil {
		return dst, nil
	}
	if jvInner.Type() != fastjson.TypeArray {
		return nil, fmt.Errorf("unexpected type for key=%q, got: %s, want: array string", key, jvInner.Type())
	}
	for _, ef := range jvInner.GetArray() {
		if ef == nil {
			continue
		}
		efs, err := ef.StringBytes()
		if err != nil {
			return nil, fmt.Errorf("unexpected non string array[] type for key=%q: %w", key, err)
		}
		dst = append(dst, bytesutil.ToUnsafeString(efs))

	}
	return dst, nil
}

var parserPool fastjson.ParserPool

var decodeb64BufferPool bytesutil.ByteBufferPool
