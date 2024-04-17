// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/auth/internal"
	"cloud.google.com/go/auth/internal/jwt"
)

const (
	// Parameter keys for AuthCodeURL method to support PKCE.
	codeChallengeKey       = "code_challenge"
	codeChallengeMethodKey = "code_challenge_method"

	// Parameter key for Exchange method to support PKCE.
	codeVerifierKey = "code_verifier"

	// 3 minutes and 45 seconds before expiration. The shortest MDS cache is 4 minutes,
	// so we give it 15 seconds to refresh it's cache before attempting to refresh a token.
	defaultExpiryDelta = 215 * time.Second

	universeDomainDefault = "googleapis.com"
)

var (
	defaultGrantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"
	defaultHeader    = &jwt.Header{Algorithm: jwt.HeaderAlgRSA256, Type: jwt.HeaderType}

	// for testing
	timeNow = time.Now
)

// TokenProvider specifies an interface for anything that can return a token.
type TokenProvider interface {
	// Token returns a Token or an error.
	// The Token returned must be safe to use
	// concurrently.
	// The returned Token must not be modified.
	// The context provided must be sent along to any requests that are made in
	// the implementing code.
	Token(context.Context) (*Token, error)
}

// Token holds the credential token used to authorized requests. All fields are
// considered read-only.
type Token struct {
	// Value is the token used to authorize requests. It is usually an access
	// token but may be other types of tokens such as ID tokens in some flows.
	Value string
	// Type is the type of token Value is. If uninitialized, it should be
	// assumed to be a "Bearer" token.
	Type string
	// Expiry is the time the token is set to expire.
	Expiry time.Time
	// Metadata  may include, but is not limited to, the body of the token
	// response returned by the server.
	Metadata map[string]interface{} // TODO(codyoss): maybe make a method to flatten metadata to avoid []string for url.Values
}

// IsValid reports that a [Token] is non-nil, has a [Token.Value], and has not
// expired. A token is considered expired if [Token.Expiry] has passed or will
// pass in the next 10 seconds.
func (t *Token) IsValid() bool {
	return t.isValidWithEarlyExpiry(defaultExpiryDelta)
}

func (t *Token) isValidWithEarlyExpiry(earlyExpiry time.Duration) bool {
	if t == nil || t.Value == "" {
		return false
	}
	if t.Expiry.IsZero() {
		return true
	}
	return !t.Expiry.Round(0).Add(-earlyExpiry).Before(timeNow())
}

// Credentials holds Google credentials, including
// [Application Default Credentials](https://developers.google.com/accounts/docs/application-default-credentials).
type Credentials struct {
	json           []byte
	projectID      CredentialsPropertyProvider
	quotaProjectID CredentialsPropertyProvider
	// universeDomain is the default service domain for a given Cloud universe.
	universeDomain CredentialsPropertyProvider

	TokenProvider
}

// JSON returns the bytes associated with the the file used to source
// credentials if one was used.
func (c *Credentials) JSON() []byte {
	return c.json
}

// ProjectID returns the associated project ID from the underlying file or
// environment.
func (c *Credentials) ProjectID(ctx context.Context) (string, error) {
	if c.projectID == nil {
		return internal.GetProjectID(c.json, ""), nil
	}
	v, err := c.projectID.GetProperty(ctx)
	if err != nil {
		return "", err
	}
	return internal.GetProjectID(c.json, v), nil
}

// QuotaProjectID returns the associated quota project ID from the underlying
// file or environment.
func (c *Credentials) QuotaProjectID(ctx context.Context) (string, error) {
	if c.quotaProjectID == nil {
		return internal.GetQuotaProject(c.json, ""), nil
	}
	v, err := c.quotaProjectID.GetProperty(ctx)
	if err != nil {
		return "", err
	}
	return internal.GetQuotaProject(c.json, v), nil
}

// UniverseDomain returns the default service domain for a given Cloud universe.
// The default value is "googleapis.com".
func (c *Credentials) UniverseDomain(ctx context.Context) (string, error) {
	if c.universeDomain == nil {
		return universeDomainDefault, nil
	}
	v, err := c.universeDomain.GetProperty(ctx)
	if err != nil {
		return "", err
	}
	if v == "" {
		return universeDomainDefault, nil
	}
	return v, err
}

// CredentialsPropertyProvider provides an implementation to fetch a property
// value for [Credentials].
type CredentialsPropertyProvider interface {
	GetProperty(context.Context) (string, error)
}

// CredentialsPropertyFunc is a type adapter to allow the use of ordinary
// functions as a [CredentialsPropertyProvider].
type CredentialsPropertyFunc func(context.Context) (string, error)

// GetProperty loads the properly value provided the given context.
func (p CredentialsPropertyFunc) GetProperty(ctx context.Context) (string, error) {
	return p(ctx)
}

// CredentialsOptions are used to configure [Credentials].
type CredentialsOptions struct {
	// TokenProvider is a means of sourcing a token for the credentials. Required.
	TokenProvider TokenProvider
	// JSON is the raw contents of the credentials file if sourced from a file.
	JSON []byte
	// ProjectIDProvider resolves the project ID associated with the
	// credentials.
	ProjectIDProvider CredentialsPropertyProvider
	// QuotaProjectIDProvider resolves the quota project ID associated with the
	// credentials.
	QuotaProjectIDProvider CredentialsPropertyProvider
	// UniverseDomainProvider resolves the universe domain with the credentials.
	UniverseDomainProvider CredentialsPropertyProvider
}

// NewCredentials returns new [Credentials] from the provided options. Most users
// will want to build this object a function from the
// [cloud.google.com/go/auth/credentials] package.
func NewCredentials(opts *CredentialsOptions) *Credentials {
	creds := &Credentials{
		TokenProvider:  opts.TokenProvider,
		json:           opts.JSON,
		projectID:      opts.ProjectIDProvider,
		quotaProjectID: opts.QuotaProjectIDProvider,
		universeDomain: opts.UniverseDomainProvider,
	}

	return creds
}

// CachedTokenProviderOptions provided options for configuring a
// CachedTokenProvider.
type CachedTokenProviderOptions struct {
	// DisableAutoRefresh makes the TokenProvider always return the same token,
	// even if it is expired.
	DisableAutoRefresh bool
	// ExpireEarly configures the amount of time before a token expires, that it
	// should be refreshed. If unset, the default value is 10 seconds.
	ExpireEarly time.Duration
}

func (ctpo *CachedTokenProviderOptions) autoRefresh() bool {
	if ctpo == nil {
		return true
	}
	return !ctpo.DisableAutoRefresh
}

func (ctpo *CachedTokenProviderOptions) expireEarly() time.Duration {
	if ctpo == nil {
		return defaultExpiryDelta
	}
	return ctpo.ExpireEarly
}

// NewCachedTokenProvider wraps a [TokenProvider] to cache the tokens returned
// by the underlying provider. By default it will refresh tokens ten seconds
// before they expire, but this time can be configured with the optional
// options.
func NewCachedTokenProvider(tp TokenProvider, opts *CachedTokenProviderOptions) TokenProvider {
	if ctp, ok := tp.(*cachedTokenProvider); ok {
		return ctp
	}
	return &cachedTokenProvider{
		tp:          tp,
		autoRefresh: opts.autoRefresh(),
		expireEarly: opts.expireEarly(),
	}
}

type cachedTokenProvider struct {
	tp          TokenProvider
	autoRefresh bool
	expireEarly time.Duration

	mu          sync.Mutex
	cachedToken *Token
}

func (c *cachedTokenProvider) Token(ctx context.Context) (*Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedToken.IsValid() || !c.autoRefresh {
		return c.cachedToken, nil
	}
	t, err := c.tp.Token(ctx)
	if err != nil {
		return nil, err
	}
	c.cachedToken = t
	return t, nil
}

// Error is a error associated with retrieving a [Token]. It can hold useful
// additional details for debugging.
type Error struct {
	// Response is the HTTP response associated with error. The body will always
	// be already closed and consumed.
	Response *http.Response
	// Body is the HTTP response body.
	Body []byte
	// Err is the underlying wrapped error.
	Err error

	// code returned in the token response
	code string
	// description returned in the token response
	description string
	// uri returned in the token response
	uri string
}

func (e *Error) Error() string {
	if e.code != "" {
		s := fmt.Sprintf("auth: %q", e.code)
		if e.description != "" {
			s += fmt.Sprintf(" %q", e.description)
		}
		if e.uri != "" {
			s += fmt.Sprintf(" %q", e.uri)
		}
		return s
	}
	return fmt.Sprintf("auth: cannot fetch token: %v\nResponse: %s", e.Response.StatusCode, e.Body)
}

// Temporary returns true if the error is considered temporary and may be able
// to be retried.
func (e *Error) Temporary() bool {
	if e.Response == nil {
		return false
	}
	sc := e.Response.StatusCode
	return sc == http.StatusInternalServerError || sc == http.StatusServiceUnavailable || sc == http.StatusRequestTimeout || sc == http.StatusTooManyRequests
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Style describes how the token endpoint wants to receive the ClientID and
// ClientSecret.
type Style int

const (
	// StyleUnknown means the value has not been initiated. Sending this in
	// a request will cause the token exchange to fail.
	StyleUnknown Style = iota
	// StyleInParams sends client info in the body of a POST request.
	StyleInParams
	// StyleInHeader sends client info using Basic Authorization header.
	StyleInHeader
)

// Options2LO is the configuration settings for doing a 2-legged JWT OAuth2 flow.
type Options2LO struct {
	// Email is the OAuth2 client ID. This value is set as the "iss" in the
	// JWT.
	Email string
	// PrivateKey contains the contents of an RSA private key or the
	// contents of a PEM file that contains a private key. It is used to sign
	// the JWT created.
	PrivateKey []byte
	// TokenURL is th URL the JWT is sent to. Required.
	TokenURL string
	// PrivateKeyID is the ID of the key used to sign the JWT. It is used as the
	// "kid" in the JWT header. Optional.
	PrivateKeyID string
	// Subject is the used for to impersonate a user. It is used as the "sub" in
	// the JWT.m Optional.
	Subject string
	// Scopes specifies requested permissions for the token. Optional.
	Scopes []string
	// Expires specifies the lifetime of the token. Optional.
	Expires time.Duration
	// Audience specifies the "aud" in the JWT. Optional.
	Audience string
	// PrivateClaims allows specifying any custom claims for the JWT. Optional.
	PrivateClaims map[string]interface{}

	// Client is the client to be used to make the underlying token requests.
	// Optional.
	Client *http.Client
	// UseIDToken requests that the token returned be an ID token if one is
	// returned from the server. Optional.
	UseIDToken bool
}

func (o *Options2LO) client() *http.Client {
	if o.Client != nil {
		return o.Client
	}
	return internal.CloneDefaultClient()
}

func (o *Options2LO) validate() error {
	if o == nil {
		return errors.New("auth: options must be provided")
	}
	if o.Email == "" {
		return errors.New("auth: email must be provided")
	}
	if len(o.PrivateKey) == 0 {
		return errors.New("auth: private key must be provided")
	}
	if o.TokenURL == "" {
		return errors.New("auth: token URL must be provided")
	}
	return nil
}

// New2LOTokenProvider returns a [TokenProvider] from the provided options.
func New2LOTokenProvider(opts *Options2LO) (TokenProvider, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	return tokenProvider2LO{opts: opts, Client: opts.client()}, nil
}

type tokenProvider2LO struct {
	opts   *Options2LO
	Client *http.Client
}

func (tp tokenProvider2LO) Token(ctx context.Context) (*Token, error) {
	pk, err := internal.ParseKey(tp.opts.PrivateKey)
	if err != nil {
		return nil, err
	}
	claimSet := &jwt.Claims{
		Iss:              tp.opts.Email,
		Scope:            strings.Join(tp.opts.Scopes, " "),
		Aud:              tp.opts.TokenURL,
		AdditionalClaims: tp.opts.PrivateClaims,
		Sub:              tp.opts.Subject,
	}
	if t := tp.opts.Expires; t > 0 {
		claimSet.Exp = time.Now().Add(t).Unix()
	}
	if aud := tp.opts.Audience; aud != "" {
		claimSet.Aud = aud
	}
	h := *defaultHeader
	h.KeyID = tp.opts.PrivateKeyID
	payload, err := jwt.EncodeJWS(&h, claimSet, pk)
	if err != nil {
		return nil, err
	}
	v := url.Values{}
	v.Set("grant_type", defaultGrantType)
	v.Set("assertion", payload)
	resp, err := tp.Client.PostForm(tp.opts.TokenURL, v)
	if err != nil {
		return nil, fmt.Errorf("auth: cannot fetch token: %w", err)
	}
	defer resp.Body.Close()
	body, err := internal.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("auth: cannot fetch token: %w", err)
	}
	if c := resp.StatusCode; c < http.StatusOK || c >= http.StatusMultipleChoices {
		return nil, &Error{
			Response: resp,
			Body:     body,
		}
	}
	// tokenRes is the JSON response body.
	var tokenRes struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		IDToken     string `json:"id_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenRes); err != nil {
		return nil, fmt.Errorf("auth: cannot fetch token: %w", err)
	}
	token := &Token{
		Value: tokenRes.AccessToken,
		Type:  tokenRes.TokenType,
	}
	token.Metadata = make(map[string]interface{})
	json.Unmarshal(body, &token.Metadata) // no error checks for optional fields

	if secs := tokenRes.ExpiresIn; secs > 0 {
		token.Expiry = time.Now().Add(time.Duration(secs) * time.Second)
	}
	if v := tokenRes.IDToken; v != "" {
		// decode returned id token to get expiry
		claimSet, err := jwt.DecodeJWS(v)
		if err != nil {
			return nil, fmt.Errorf("auth: error decoding JWT token: %w", err)
		}
		token.Expiry = time.Unix(claimSet.Exp, 0)
	}
	if tp.opts.UseIDToken {
		if tokenRes.IDToken == "" {
			return nil, fmt.Errorf("auth: response doesn't have JWT token")
		}
		token.Value = tokenRes.IDToken
	}
	return token, nil
}
