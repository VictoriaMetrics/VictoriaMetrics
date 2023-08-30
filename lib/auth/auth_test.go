package auth

import (
	"testing"
)

func TestNewTokenSuccess(t *testing.T) {
	f := func(token string, want string) {
		t.Helper()
		newToken, err := NewToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		got := newToken.String()
		if got != want {
			t.Fatalf("unexpected NewToken() result;got\n%s\nwant\n%s", got, want)
		}
	}
	// token with accountID only
	f("1", "1")
	// token with accountID and projecTID
	f("1:2", "1:2")
	// max uint32 accountID
	f("4294967295:1", "4294967295:1")
	// max uint32 projectID
	f("1:4294967295", "1:4294967295")
	// max uint32 accountID and projectID
	f("4294967295:4294967295", "4294967295:4294967295")
}

func TestNewTokenPossibleMultitenantSuccess(t *testing.T) {
	f := func(token string, want string) {
		t.Helper()
		newToken, err := NewTokenPossibleMultitenant(token)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		got := newToken.String()
		if got != want {
			t.Fatalf("unexpected NewToken() result;got\n%s\nwant\n%s", got, want)
		}
	}
	// token with accountID only
	f("1", "1")
	// token with accountID and projecTID
	f("1:2", "1:2")
	// multitenant
	f("multitenant", "multitenant")
}

func TestNewTokenFailure(t *testing.T) {
	f := func(token string) {
		t.Helper()
		newToken, err := NewToken(token)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if newToken != nil {
			t.Fatalf("expecting nil token; got\n%#v", newToken)
		}
	}
	// empty token
	f("")
	// empty accountID and projectID"
	f(":")
	// accountID and projectID not int values
	f("a:b")
	// missed projectID
	f("1:")
	// missed accountID
	f(":2")
	// large int value for accountID
	f("9223372036854775808:1")
	// large value for projectID
	f("2:9223372036854775808")
	// both large values incorrect
	f("9223372036854775809:9223372036854775808")
	// large uint32 values incorrect
	f("4294967297:4294967295")
	//negative accountID
	f("-100:100")
	// negative projectID
	f("100:-100")
	// negative accountID and projectID
	f("-100:-100")
	// accountID is string
	f("abcd:2")
	// projectID is string
	f("2:abcd")
	// empty many parts in the token
	f("::")
	// many string parts in the token
	f("a:b:c")
	// many int parts in the token"
	f("1:2:3")
	// multitenant
	f("multitenant")
}
