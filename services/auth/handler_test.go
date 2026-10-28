package main

import (
	"testing"
	"time"

	sharedauth "github.com/adil/bug-issue-tracker/pkg/auth"
)

func TestTokenRoundTripAndTampering(t *testing.T) {
	secret := []byte("test-secret")
	token, err := sharedauth.IssueToken(42, "admin", secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	id, role, err := sharedauth.VerifyToken(token, secret)
	if err != nil || id != 42 || role != "admin" {
		t.Fatalf("got (%d, %q, %v)", id, role, err)
	}
	if _, _, err := sharedauth.VerifyToken(token+"x", secret); err == nil {
		t.Fatal("tampered token unexpectedly valid")
	}
	if _, _, err := sharedauth.VerifyToken("bad", secret); err == nil {
		t.Fatal("malformed token unexpectedly valid")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	secret := []byte("test-secret")
	token, err := sharedauth.IssueToken(1, "member", secret, -time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sharedauth.VerifyToken(token, secret); err == nil {
		t.Fatal("expired token unexpectedly valid")
	}
}
