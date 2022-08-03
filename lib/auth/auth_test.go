package auth

import (
	"fmt"
	"testing"
)

func TestNewTokenSuccess(t *testing.T) {
	f := func(name string, token string, want string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			newToken, err := NewToken(token)
			if err != nil {
				t.Fatalf("expecting nil error")
			}
			got := fmt.Sprintf("%d:%d", newToken.AccountID, newToken.ProjectID)
			if got != want {
				t.Errorf("NewToken() got = %v, want %v", newToken, want)
			}
		})

	}
	f("token with accountID and projectID", "1:2", "1:2")
	f("max uint32 accountID", "4294967295:1", "4294967295:1")
	f("max uint32 projectID", "1:4294967295", "1:4294967295")
	f("max uint32 accountID and projectID", "4294967295:4294967295", "4294967295:4294967295")
}

func TestNewTokenFailure(t *testing.T) {
	f := func(name string, token string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			newToken, err := NewToken(token)
			if err == nil {
				t.Fatalf("expecting non-nil error")
			}
			if newToken != nil {
				t.Fatalf("expecting nil token")
			}
		})
	}
	f("empty token", "")
	f("empty accountID and projectID", ":")
	f("accountID and projectID not int values", "a:b")
	f("missed projectID", "1:")
	f("missed accountID", ":2")
	f("large int value for accountID", "9223372036854775808:1")
	f("large int value for projectID", "2:9223372036854775808")
	f("both large int values incorrect", "9223372036854775809:9223372036854775808")
	f("large uint32 values incorrect", "4294967297:4294967295")
	f("negative accountID", "-100:100")
	f("negative projectID", "100:-100")
	f("negative accountID and projectID", "-100:-100")
	f("accountID is string", "abcd:2")
	f("projectID is string", "2:abcd")
	f("empty many parts in the token", "::")
	f("many string parts in the token", "a:b:c")
	f("many int parts in the token", "1:2:3")
}
