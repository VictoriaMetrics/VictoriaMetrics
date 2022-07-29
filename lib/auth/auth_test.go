package auth

import (
	"fmt"
	"testing"
)

func TestNewToken(t *testing.T) {
	f := func(token string, want string, wantErr bool) {
		t.Helper()
		newToken, err := NewToken(token)
		if (err != nil) != wantErr {
			t.Errorf("NewToken() error = %v, wantErr %v", err, wantErr)
			return
		}
		var got string
		if newToken != nil {
			got = fmt.Sprintf("%d:%d", newToken.AccountID, newToken.ProjectID)
		}
		if got != want {
			t.Errorf("NewToken() got = %v, want %v", got, want)
		}
	}
	f("", "", true)
	f(":", "", true)
	f("a:b", "", true)
	f("1:", "", true)
	f(":2", "", true)
	f("::", "", true)
	f("a:b:c", "", true)
	f("1:2:3", "", true)
	f("1:2", "1:2", false)
}
