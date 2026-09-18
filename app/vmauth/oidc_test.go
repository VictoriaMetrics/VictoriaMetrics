package main

import (
	"strings"
	"testing"
	"time"
)

func TestValidateIDToken(t *testing.T) {
	jt := newTokenTester(t)

	issuer := "https://idp.example.com"
	clientID := "my-client"
	nonce := "test-nonce"

	pm := &oidcProviderMetadata{
		Issuer: issuer,
		vp:     jt.NewVerifierPool(),
	}

	// valid token
	validToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err := validateIDToken(validToken, pm, clientID, nonce)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// invalid signature
	invalidSigToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, false)
	_, err = validateIDToken(invalidSigToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
		t.Fatalf("expected signature verification error; got %v", err)
	}

	// expired token
	expiredToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": nonce,
		"exp":   time.Now().Add(-time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(expiredToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error; got %v", err)
	}

	// wrong issuer
	wrongIssuerToken := jt.GenToken(map[string]any{
		"iss":   "https://evil.com",
		"aud":   clientID,
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(wrongIssuerToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "issuer mismatch") {
		t.Fatalf("expected issuer mismatch error; got %v", err)
	}

	// wrong audience
	wrongAudToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   "wrong-client",
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(wrongAudToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "audience mismatch") {
		t.Fatalf("expected audience mismatch error; got %v", err)
	}

	// wrong nonce
	wrongNonceToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": "wrong-nonce",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(wrongNonceToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "nonce mismatch") {
		t.Fatalf("expected nonce mismatch error; got %v", err)
	}

	// client_id with regex metacharacters must be compared literally
	regexAudToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   "anything",
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(regexAudToken, pm, ".*", nonce)
	if err == nil || !strings.Contains(err.Error(), "audience mismatch") {
		t.Fatalf("expected audience mismatch for regex client_id; got %v", err)
	}

	// nonce with regex metacharacters must be compared literally
	regexNonceToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": "anything",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(regexNonceToken, pm, clientID, ".*")
	if err == nil || !strings.Contains(err.Error(), "nonce mismatch") {
		t.Fatalf("expected nonce mismatch for regex nonce; got %v", err)
	}
}
