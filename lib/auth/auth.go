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
	if t.ProjectID == 0 {
		return fmt.Sprintf("%d", t.AccountID)
	}
	return fmt.Sprintf("%d:%d", t.AccountID, t.ProjectID)
}

// NewToken returns new Token for the given authToken
func NewToken(authToken string) (*Token, error) {
	tmp := strings.Split(authToken, ":")
	if len(tmp) > 2 {
		return nil, fmt.Errorf("unexpected number of items in authToken %q; got %d; want 1 or 2", authToken, len(tmp))
	}
	var at Token
	accountID, err := strconv.ParseUint(tmp[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("cannot parse accountID from %q: %w", tmp[0], err)
	}
	at.AccountID = uint32(accountID)
	if len(tmp) > 1 {
		projectID, err := strconv.ParseUint(tmp[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("cannot parse projectID from %q: %w", tmp[1], err)
		}
		at.ProjectID = uint32(projectID)
	}
	return &at, nil
}
