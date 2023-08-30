package auth

import (
	"fmt"
	"strconv"
	"strings"
)

// Token contains settings for request processing
type Token struct {
	AccountID uint32
	ProjectID uint32
}

// String returns string representation of t.
func (t *Token) String() string {
	if t == nil {
		return "multitenant"
	}
	if t.ProjectID == 0 {
		return fmt.Sprintf("%d", t.AccountID)
	}
	return fmt.Sprintf("%d:%d", t.AccountID, t.ProjectID)
}

// NewToken returns new Token for the given authToken.
func NewToken(authToken string) (*Token, error) {
	var t Token
	if err := t.Init(authToken); err != nil {
		return nil, err
	}
	return &t, nil
}

// NewTokenPossibleMultitenant returns new Token for the given authToken.
//
// If authToken == "multitenant", then nil Token is returned.
func NewTokenPossibleMultitenant(authToken string) (*Token, error) {
	if authToken == "multitenant" {
		return nil, nil
	}
	return NewToken(authToken)
}

// Init initializes t from authToken.
func (t *Token) Init(authToken string) error {
	tmp := strings.Split(authToken, ":")
	if len(tmp) > 2 {
		return fmt.Errorf("unexpected number of items in authToken %q; got %d; want 1 or 2", authToken, len(tmp))
	}
	n, err := strconv.ParseUint(tmp[0], 10, 32)
	if err != nil {
		return fmt.Errorf("cannot parse accountID from %q: %w", tmp[0], err)
	}
	accountID := uint32(n)
	projectID := uint32(0)
	if len(tmp) > 1 {
		n, err := strconv.ParseUint(tmp[1], 10, 32)
		if err != nil {
			return fmt.Errorf("cannot parse projectID from %q: %w", tmp[1], err)
		}
		projectID = uint32(n)
	}
	t.Set(accountID, projectID)
	return nil
}

// Set sets accountID and projectID for the t.
func (t *Token) Set(accountID, projectID uint32) {
	t.AccountID = accountID
	t.ProjectID = projectID
}
