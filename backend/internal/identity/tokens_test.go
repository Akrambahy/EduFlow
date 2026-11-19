package identity

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func TestIssueAndParse(t *testing.T) {
	ti := NewTokenIssuer(testSecret, time.Minute)
	raw, err := ti.Issue("user-1", []string{"admin"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ti.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || len(claims.Roles) != 1 || claims.Roles[0] != "admin" {
		t.Fatalf("unexpected claims %+v", claims)
	}
}

func TestParseRejectsExpired(t *testing.T) {
	ti := NewTokenIssuer(testSecret, time.Minute)
	ti.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	raw, _ := ti.Issue("user-1", nil)
	ti.now = time.Now
	if _, err := ti.Parse(raw); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	raw, _ := NewTokenIssuer(testSecret, time.Minute).Issue("user-1", nil)
	other := NewTokenIssuer([]byte("ffffffffffffffffffffffffffffffff"), time.Minute)
	if _, err := other.Parse(raw); err == nil {
		t.Fatal("expected signature mismatch to fail")
	}
}

func TestParseRejectsNoneAlg(t *testing.T) {
	claims := AccessClaims{RegisteredClaims: jwt.RegisteredClaims{
		Issuer: tokenIssuer, Subject: "user-1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTokenIssuer(testSecret, time.Minute).Parse(raw); err == nil {
		t.Fatal("expected alg=none to fail")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := NewTokenIssuer(testSecret, time.Minute).Parse("not.a.jwt"); err == nil {
		t.Fatal("expected garbage to fail")
	}
}

func TestRefreshTokenHash(t *testing.T) {
	raw, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 40 || strings.ContainsAny(raw, "+/=") {
		t.Fatalf("unexpected token format %q", raw)
	}
	if !bytes.Equal(hash, HashRefreshToken(raw)) {
		t.Fatal("hash mismatch")
	}
	raw2, _, _ := NewRefreshToken()
	if raw == raw2 {
		t.Fatal("tokens must be unique")
	}
}
