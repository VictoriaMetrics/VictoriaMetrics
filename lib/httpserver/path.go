package httpserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Path contains the following path structure:
// - /{prefix}/{tenantID}/{suffix}
// - /{prefix}/{suffix} -H "{tenantID}"
// in `/{prefix}/{suffix}` format tenantID is extracted from HTTP headers
type Path struct {
	Prefix    string
	AuthToken string
	Suffix    string
}

// ParsePath parses the given path according to /{prefix}/{tenantID}/{suffix} format
func ParsePath(path string) (*Path, error) {
	// The path must have the following form:
	// /{prefix}/{authToken}/{suffix}
	//
	// - prefix must contain `select`, `insert` or `delete`.
	// - authToken contains `accountID[:projectID]`, where projectID is optional.
	//   authToken may also contain `multitenant` string. In this case the accountID and projectID
	//   are obtained from vm_account_id and vm_project_id labels of the ingested samples.
	// - suffix contains arbitrary suffix.
	//
	// prefix must be used for the routing to the appropriate service
	// in the cluster - either vminsert or vmselect.
	s := skipPrefixSlashes(path)
	n := strings.IndexByte(s, '/')
	if n < 0 {
		return nil, fmt.Errorf("cannot find {prefix} in %q; expecting /{prefix}/{authToken}/{suffix} format", path)
	}
	prefix := s[:n]

	s = skipPrefixSlashes(s[n+1:])
	n = strings.IndexByte(s, '/')
	if n < 0 {
		return nil, fmt.Errorf("cannot find {authToken} in %q; expecting /{prefix}/{authToken}/{suffix} format", path)
	}
	authToken := s[:n]

	s = skipPrefixSlashes(s[n+1:])

	// Substitute double slashes with single slashes in the path, since such slashes
	// may appear due improper copy-pasting of the url.
	suffix := strings.ReplaceAll(s, "//", "/")

	p := &Path{
		Prefix:    prefix,
		AuthToken: authToken,
		Suffix:    suffix,
	}
	return p, nil
}

// ParsePathAndHeaders parses the given path and headers.
//
// The path may be one of the following forms:
//
//  1. /{prefix}/{tenantID}/{suffix} — tenantID is in the URL
//  2. /{prefix}/{suffix} — tenantID is omitted and expected to be read from AccountID/ProjectID HTTP headers.
//     If these headers are missing, tenantID is set to "0:0" to be consistent with VictoriaLogs behavior.
//
// prefix is "select", "insert", or "delete".
// tenantID is "accountID[:projectID]" or "multitenant".
// tenantID specified in path always takes priority over headers for backward compatibility.
//
// This function doesn't validate correctness of {tenantID} content.
func ParsePathAndHeaders(path string, h http.Header) (*Path, error) {
	s := skipPrefixSlashes(path)
	n := strings.IndexByte(s, '/')
	if n < 0 {
		return nil, fmt.Errorf("cannot find {prefix} in %q; expecting /{prefix}/{suffix} or /{prefix}/{tenantID}/{suffix} format; "+
			"see https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format", path)
	}

	prefix := s[:n]
	tail := skipPrefixSlashes(s[n+1:])

	if tail == "" {
		return nil, fmt.Errorf("cannot find {suffix} in %q; expecting /{prefix}/{suffix} or /{prefix}/{tenantID}/{suffix} format; "+
			"see https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format", path)
	}

	// Try to split tail into {tenantID}/{suffix} segments.
	// If the first segment is a valid tenantID - consume it, ignore headers
	// Otherwise, treat tail as {suffix} and read tenantID from HTTP headers.
	var tenantID string
	suffix := tail
	n = strings.IndexByte(tail, '/')
	if n >= 0 {
		tenantID = tail[:n]
	}
	if maybeTenantID(tenantID) {
		// cut the tenantID from suffix
		suffix = skipPrefixSlashes(tail[n+1:])
	} else {
		// tenantID is not valid - assume tail is all suffix and tenantID is in headers
		tenantID = tenantIDFromHeadersOrDefault(h, "0:0")
	}

	// Substitute double slashes with single slashes in the path, since such slashes
	// may appear due to improper copy-pasting of the url.
	suffix = strings.ReplaceAll(suffix, "//", "/")

	return &Path{
		Prefix:    prefix,
		AuthToken: tenantID,
		Suffix:    suffix,
	}, nil
}

// maybeTenantID returns true if s is "multitenant", "<uint>" or contains ":" char.
// It doesn't validate correctness of tenantID and is only used for quick routing.
// It is expected that tenantID will be correctly validated later.
func maybeTenantID(tenantID string) bool {
	if tenantID == "" {
		return false
	}
	if tenantID == "multitenant" {
		return true
	}

	idx := strings.IndexByte(tenantID, ':')
	if idx > 0 {
		return true
	}

	_, err := strconv.ParseUint(tenantID, 10, 32)
	return err == nil
}

// tenantIDFromHeaders reads AccountID and ProjectID header values from request.
// If headers are missing, it returns defaultTenantID.
func tenantIDFromHeadersOrDefault(h http.Header, defaultTenantID string) string {
	aID := h.Get("AccountID")
	pID := h.Get("ProjectID")
	if len(aID) == 0 && len(pID) == 0 {
		return defaultTenantID
	}

	if aID == "multitenant" {
		// special case for multitenant
		return "multitenant"
	}

	accountID, projectID := "0", "0"
	if len(aID) > 0 {
		accountID = aID
	}
	if len(pID) > 0 {
		projectID = pID
	}
	return fmt.Sprintf("%s:%s", accountID, projectID)
}

// skipPrefixSlashes remove double slashes which may appear due
// improper copy-pasting of the url
func skipPrefixSlashes(s string) string {
	for len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return s
}
