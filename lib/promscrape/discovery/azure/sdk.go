package azure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

func newSDKRefreshTokenFunc(ctx context.Context, sdc *SDConfig, ac, proxyAC *promauth.Config, env *cloudEnvironmentEndpoints) (refreshTokenFunc, error) {
	client := newSDKHTTPClient(sdc, ac, proxyAC)
	options := policy.ClientOptions{
		Cloud:     cloud.Configuration{ActiveDirectoryAuthorityHost: env.ActiveDirectoryEndpoint},
		Transport: client,
	}
	cred, err := newSDKCredential(sdc, options)
	if err != nil {
		return nil, sdkAuthError("cannot create Azure SDK credential", err)
	}
	scope := strings.TrimRight(env.ResourceManagerEndpoint, "/") + "/.default"
	return sdkRefreshTokenFunc(ctx, cred, scope), nil
}

func newSDKCredential(sdc *SDConfig, options policy.ClientOptions) (azcore.TokenCredential, error) {
	if strings.EqualFold(sdc.AuthenticationMethod, "WorkloadIdentity") {
		return azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientOptions:            options,
			DisableInstanceDiscovery: strings.EqualFold(sdc.Environment, "AzureStackCloud"),
		})
	}
	return azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{
		ClientOptions:            options,
		DisableInstanceDiscovery: strings.EqualFold(sdc.Environment, "AzureStackCloud"),
		// This sets the tenant for workload identity and developer tools, not EnvironmentCredential.
		TenantID: sdc.TenantID,
	})
}

func sdkRefreshTokenFunc(ctx context.Context, cred azcore.TokenCredential, scope string) refreshTokenFunc {
	return func() (string, time.Duration, error) {
		ctx, cancel := context.WithTimeout(ctx, discoveryutil.DefaultClientReadTimeout)
		defer cancel()
		token, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
		if err != nil {
			// The SDK can flatten context errors into AuthenticationFailedError.
			// Recover cancellation from our context, without exposing SDK details.
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			return "", 0, sdkAuthError("cannot acquire Azure SDK token", err)
		}
		expiresIn := time.Until(token.ExpiresOn)
		if token.Token == "" || expiresIn <= 0 {
			return "", 0, fmt.Errorf("Azure SDK returned an empty or expired token")
		}
		return token.Token, expiresIn, nil
	}
}

// SDK errors can include response bodies or external tool output. Never log these,
// since the identity provider may echo a client assertion or an access token.
func sdkAuthError(message string, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w", message, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", message, context.DeadlineExceeded)
	}
	var authErr *azidentity.AuthenticationFailedError
	if errors.As(err, &authErr) && authErr.RawResponse != nil {
		return fmt.Errorf("%s: HTTP status %d", message, authErr.RawResponse.StatusCode)
	}
	return fmt.Errorf("%s; check Azure identity configuration and credentials", message)
}

// sdkHTTPClient applies the same HTTP options as the discovery client, while
// leaving response handling and retries to the Azure SDK.
type sdkHTTPClient struct {
	client      *http.Client
	ac, proxyAC *promauth.Config
	proxyURL    *proxy.URL
}

func newSDKHTTPClient(sdc *SDConfig, ac, proxyAC *promauth.Config) *sdkHTTPClient {
	tr := httputil.NewTransport(false, "vm_promscrape_discovery")
	if u := sdc.ProxyURL.GetURL(); u != nil {
		tr.Proxy = http.ProxyURL(u)
	}
	tr.DialContext = netutil.NewStatDialFunc("vm_promscrape_discovery")
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.ResponseHeaderTimeout = discoveryutil.DefaultClientReadTimeout
	// Token requests are infrequent. Avoid retaining a separate pool of idle
	// authentication connections after service discovery stops.
	tr.DisableKeepAlives = true
	client := &http.Client{
		Transport: ac.NewRoundTripper(tr),
		Timeout:   discoveryutil.DefaultClientReadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// SDK token POSTs have replayable bodies. Never forward credentials
			// from a secure authority to a plaintext redirect destination.
			if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return errors.New("Azure SDK HTTPS redirect requires HTTPS")
			}
			// A custom CheckRedirect replaces net/http's default ten-request limit.
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}
	if follow := sdc.HTTPClientConfig.FollowRedirects; follow != nil && !*follow {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return &sdkHTTPClient{client: client, ac: ac, proxyAC: proxyAC, proxyURL: sdc.ProxyURL}
}

func (c *sdkHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if err := c.ac.SetHeaders(req, true); err != nil {
		return nil, sanitizeSDKHTTPError(err)
	}
	if err := c.proxyURL.SetHeaders(c.proxyAC, req); err != nil {
		return nil, sanitizeSDKHTTPError(err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, sanitizeSDKHTTPError(err)
	}
	if resp.Body != nil {
		resp.Body = &sdkResponseBody{body: resp.Body}
	}
	return resp, nil
}

// The SDK logs transport and body errors before GetToken returns. Sanitize at
// this boundary, not only in sdkAuthError. Keep control-flow classifications,
// but never retain or unwrap the raw cause: it can contain echoed credentials.
func sanitizeSDKHTTPError(err error) error {
	if err == nil {
		return nil
	}
	for _, safe := range []error{context.Canceled, context.DeadlineExceeded, io.EOF, io.ErrUnexpectedEOF} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return &sdkHTTPNetworkError{timeout: ne.Timeout(), temporary: ne.Temporary()}
	}
	return errors.New("Azure SDK HTTP request failed")
}

type sdkHTTPNetworkError struct {
	timeout, temporary bool
}

func (*sdkHTTPNetworkError) Error() string     { return "Azure SDK HTTP request failed" }
func (e *sdkHTTPNetworkError) Timeout() bool   { return e.timeout }
func (e *sdkHTTPNetworkError) Temporary() bool { return e.temporary }

type sdkResponseBody struct {
	body io.ReadCloser
}

func (b *sdkResponseBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	return n, sanitizeSDKHTTPError(err)
}

func (b *sdkResponseBody) Close() error {
	return sanitizeSDKHTTPError(b.body.Close())
}
