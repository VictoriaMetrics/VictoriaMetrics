package logstorage

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// TenantID is an id of a tenant for log streams.
//
// Each log stream is associated with a single TenantID.
type TenantID struct {
	// AccountID is the id of the account for the log stream.
	AccountID uint32

	// ProjectID is the id of the project for the log stream.
	ProjectID uint32
}

// Reset resets tid.
func (tid *TenantID) Reset() {
	tid.AccountID = 0
	tid.ProjectID = 0
}

// String returns human-readable representation of tid
func (tid *TenantID) String() string {
	return fmt.Sprintf("{accountID=%d,projectID=%d}", tid.AccountID, tid.ProjectID)
}

// equal returns true if tid equals to a.
func (tid *TenantID) equal(a *TenantID) bool {
	return tid.AccountID == a.AccountID && tid.ProjectID == a.ProjectID
}

// less returns true if tid is less than a.
func (tid *TenantID) less(a *TenantID) bool {
	if tid.AccountID != a.AccountID {
		return tid.AccountID < a.AccountID
	}
	return tid.ProjectID < a.ProjectID
}

func (tid *TenantID) marshalString(dst []byte) []byte {
	n := uint64(tid.AccountID)<<32 | uint64(tid.ProjectID)
	dst = marshalUint64Hex(dst, n)
	return dst
}

// marshal appends the marshaled tid to dst and returns the result
func (tid *TenantID) marshal(dst []byte) []byte {
	dst = encoding.MarshalUint32(dst, tid.AccountID)
	dst = encoding.MarshalUint32(dst, tid.ProjectID)
	return dst
}

// unmarshal unmarshals tid from src and returns the remaining tail.
func (tid *TenantID) unmarshal(src []byte) ([]byte, error) {
	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal tenantID from %d bytes; need at least 8 bytes", len(src))
	}
	tid.AccountID = encoding.UnmarshalUint32(src[:4])
	tid.ProjectID = encoding.UnmarshalUint32(src[4:])
	return src[8:], nil
}

// GetTenantIDFromRequest returns tenantID from r.
func GetTenantIDFromRequest(r *http.Request) (TenantID, error) {
	var tenantID TenantID

	accountID, err := getUint32FromHeader(r, "AccountID")
	if err != nil {
		return tenantID, err
	}
	projectID, err := getUint32FromHeader(r, "ProjectID")
	if err != nil {
		return tenantID, err
	}

	tenantID.AccountID = accountID
	tenantID.ProjectID = projectID
	return tenantID, nil
}

// ParseTenantID returns tenantID from s.
//
// s is expected in the form of accountID:projectID. If s is empty, then zero tenantID is returned.
func ParseTenantID(s string) (TenantID, error) {
	var tenantID TenantID
	if s == "" {
		return tenantID, nil
	}

	n := strings.Index(s, ":")
	if n < 0 {
		account, err := getUint32FromString(s)
		if err != nil {
			return tenantID, fmt.Errorf("cannot parse accountID from %q: %w", s, err)
		}
		tenantID.AccountID = account

		return tenantID, nil
	}

	account, err := getUint32FromString(s[:n])
	if err != nil {
		return tenantID, fmt.Errorf("cannot parse accountID part from %q: %w", s, err)
	}
	tenantID.AccountID = account

	project, err := getUint32FromString(s[n+1:])
	if err != nil {
		return tenantID, fmt.Errorf("cannot parse projectID part from %q: %w", s, err)
	}
	tenantID.ProjectID = project

	return tenantID, nil
}

// MarshalTenantIDs appends marshaled tenantIDs to dst and returns the result.
func MarshalTenantIDs(dst []byte, tenantIDs []TenantID) []byte {
	for i := range tenantIDs {
		dst = tenantIDs[i].marshal(dst)
	}
	return dst
}

// UnmarshalTenantIDs unmarshals tenantIDs from src.
func UnmarshalTenantIDs(src []byte) ([]TenantID, error) {
	var tenantIDs []TenantID
	for len(src) > 0 {
		var tid TenantID
		tail, err := tid.unmarshal(src)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal tenantID #%d: %w", len(tenantIDs), err)
		}
		src = tail

		tenantIDs = append(tenantIDs, tid)
	}
	return tenantIDs, nil
}

func getUint32FromHeader(r *http.Request, headerName string) (uint32, error) {
	s := r.Header.Get(headerName)
	if len(s) == 0 {
		return 0, nil
	}
	return getUint32FromString(s)
}

func getUint32FromString(s string) (uint32, error) {
	if len(s) == 0 {
		return 0, nil
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q as uint32: %w", s, err)
	}
	return uint32(n), nil
}
